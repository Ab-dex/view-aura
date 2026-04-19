use dashmap::DashMap;
use std::sync::Arc;
use tokio::sync::mpsc;
use tracing::{debug, info};

use crate::protocol::ServerMessage;

/// ConnId uniquely identifies one WebSocket connection within this pod.
pub type ConnId = String;

/// A sender half of the per-connection message channel.
/// Used to push server messages to an individual connection.
pub type ConnTx = mpsc::UnboundedSender<ServerMessage>;

/// ConnectionInfo holds metadata for a connected client.
#[derive(Clone)]
pub struct ConnectionInfo {
    pub conn_id:      ConnId,
    pub user_id:      String,
    pub display_name: String,
    pub is_host:      bool,
    pub last_ts_ms:   u64,
}

/// RoomRegistry is the in-process registry of all active WebSocket connections.
///
/// Architecture note: each pod maintains its own in-process registry. When a
/// server message needs to reach connections on *other* pods (e.g. a chat
/// message from pod A to connections on pod B), it is published to the Redis
/// pub/sub channel. Each pod subscribes to the channels of parties it has at
/// least one connection for, and fans out received pub/sub messages to its
/// local connections.
///
/// This means:
///   - In-pod latency: direct channel send (<1µs)
///   - Cross-pod latency: Redis pub/sub + local fan-out (~1-5ms)
pub struct RoomRegistry {
    /// party_id → map of conn_id → (ConnTx, ConnectionInfo)
    rooms: DashMap<String, DashMap<ConnId, (ConnTx, ConnectionInfo)>>,
}

impl RoomRegistry {
    pub fn new() -> Arc<Self> {
        Arc::new(Self {
            rooms: DashMap::new(),
        })
    }

    /// Register a new connection. Returns true if this is the first connection
    /// in the party (used to initialise the Redis pub/sub subscription).
    pub fn join(
        &self,
        party_id:   &str,
        info:       ConnectionInfo,
        tx:         ConnTx,
    ) -> bool {
        let room = self.rooms.entry(party_id.to_string()).or_default();
        let first = room.is_empty();
        room.insert(info.conn_id.clone(), (tx, info));
        first
    }

    /// Deregister a connection. Returns true if this was the last connection
    /// in the party (used to teardown the Redis pub/sub subscription).
    pub fn leave(&self, party_id: &str, conn_id: &str) -> bool {
        if let Some(room) = self.rooms.get(party_id) {
            room.remove(conn_id);
            if room.is_empty() {
                drop(room);
                self.rooms.remove(party_id);
                return true;
            }
        }
        false
    }

    /// Update the last known timestamp for a connection (used for drift detection).
    pub fn update_ts(&self, party_id: &str, conn_id: &str, ts_ms: u64) {
        if let Some(room) = self.rooms.get(party_id) {
            if let Some(mut entry) = room.get_mut(conn_id) {
                entry.1.last_ts_ms = ts_ms;
            }
        }
    }

    /// Get connection info for a single connection.
    pub fn get_info(&self, party_id: &str, conn_id: &str) -> Option<ConnectionInfo> {
        self.rooms
            .get(party_id)
            .and_then(|room| room.get(conn_id).map(|e| e.1.clone()))
    }

    /// Get all connection infos for a party (for the Welcome message participant list).
    pub fn list_participants(&self, party_id: &str) -> Vec<ConnectionInfo> {
        self.rooms
            .get(party_id)
            .map(|room| room.iter().map(|e| e.1.clone()).collect())
            .unwrap_or_default()
    }

    /// Count connections in this party on this pod.
    pub fn local_count(&self, party_id: &str) -> usize {
        self.rooms
            .get(party_id)
            .map(|r| r.len())
            .unwrap_or(0)
    }

    /// Send a message to a specific connection.
    pub fn send_to(&self, party_id: &str, conn_id: &str, msg: ServerMessage) {
        if let Some(room) = self.rooms.get(party_id) {
            if let Some(entry) = room.get(conn_id) {
                let _ = entry.0.send(msg);
            }
        }
    }

    /// Broadcast a message to all connections in a party on this pod,
    /// optionally excluding the sender.
    pub fn broadcast(&self, party_id: &str, msg: ServerMessage, exclude: Option<&str>) {
        if let Some(room) = self.rooms.get(party_id) {
            for entry in room.iter() {
                if Some(entry.key().as_str()) == exclude {
                    continue;
                }
                let _ = entry.0.send(msg.clone());
            }
        }
    }

    /// Broadcast to every connection in the party including the sender.
    pub fn broadcast_all(&self, party_id: &str, msg: ServerMessage) {
        self.broadcast(party_id, msg, None);
    }

    /// Apply a seek correction to participants whose drift exceeds max_drift_ms.
    pub fn apply_drift_correction(
        &self,
        party_id:      &str,
        host_conn_id:  &str,
        host_ts_ms:    u64,
        max_drift_ms:  u64,
        paused:        bool,
    ) {
        use crate::protocol::{SeekCorrectionPayload, ServerMessage};

        if let Some(room) = self.rooms.get(party_id) {
            for entry in room.iter() {
                if entry.key() == host_conn_id {
                    continue;   // never correct the host
                }
                let participant_ts = entry.1.last_ts_ms;
                let drift = host_ts_ms.saturating_sub(participant_ts)
                    .max(participant_ts.saturating_sub(host_ts_ms));

                if drift > max_drift_ms {
                    debug!(
                        conn_id = %entry.key(),
                        drift_ms = drift,
                        "sending seek correction"
                    );
                    let _ = entry.0.send(ServerMessage::SeekCorrection(
                        SeekCorrectionPayload { ts_ms: host_ts_ms, paused },
                    ));
                }
            }
        }
    }
}