//! Wire protocol for the Watch Party WebSocket connection.
//!
//! All messages are JSON-encoded. The `type` field discriminates the variant.
//! Clients MUST send only `ClientMessage` types; server sends `ServerMessage`.

use serde::{Deserialize, Serialize};

// ─── Client → Server ──────────────────────────────────────────────────────────

/// Every message sent by the client over the WebSocket.
#[derive(Debug, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ClientMessage {
    /// Host-only. Broadcast the current playback state to all participants.
    /// Clients should send this every 500 ms while playing.
    Sync(SyncPayload),

    /// Any participant. Send a chat message to the room.
    Chat(ChatPayload),

    /// Any participant. Send an emoji reaction (ephemeral — not persisted).
    Reaction(ReactionPayload),

    /// Host-only. Pause or resume playback for all participants.
    PlayPause(PlayPausePayload),

    /// Host-only. Seek all participants to a specific timestamp.
    Seek(SeekPayload),

    /// Host-only. Change the currently playing movie.
    ChangeMovie(ChangeMoviePayload),

    /// Host-only. Kick a participant from the party.
    Kick(KickPayload),

    /// Any participant. Explicit ping to keep the connection alive.
    Ping,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct SyncPayload {
    /// Current playback position in milliseconds.
    pub ts_ms:  u64,
    /// Whether playback is currently paused.
    pub paused: bool,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct ChatPayload {
    /// Message text. Max 500 characters; server truncates if longer.
    pub text: String,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct ReactionPayload {
    /// Unicode emoji string e.g. "🎬", "😂", "👏"
    pub emoji: String,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct PlayPausePayload {
    pub paused: bool,
    pub ts_ms:  u64,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct SeekPayload {
    pub ts_ms: u64,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct ChangeMoviePayload {
    pub movie_id: String,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct KickPayload {
    pub user_id: String,
}

// ─── Server → Client ──────────────────────────────────────────────────────────

/// Every message sent by the server over the WebSocket.
#[derive(Debug, Serialize, Clone)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ServerMessage {
    /// Sent to the connecting client on successful join.
    Welcome(WelcomePayload),

    /// Sent to all existing participants when someone joins.
    ParticipantJoined(ParticipantPayload),

    /// Sent to all participants when someone leaves or is disconnected.
    ParticipantLeft(ParticipantPayload),

    /// Relayed sync state from the host. Clients use this to stay in sync.
    Sync(SyncPayload),

    /// Drift correction. Sent only to participants who are out of sync.
    SeekCorrection(SeekCorrectionPayload),

    /// Relayed chat message with author metadata.
    Chat(ChatMessage),

    /// Ephemeral emoji reaction — float on screen for ~2 seconds.
    Reaction(ReactionMessage),

    /// Host changed the movie.
    MovieChanged(MovieChangedPayload),

    /// The calling client was kicked by the host.
    Kicked { reason: String },

    /// The party ended (host left or explicitly closed it).
    PartyEnded,

    /// Error message for a malformed or unauthorised client request.
    Error { code: String, message: String },

    /// Pong — response to client Ping.
    Pong,
}

#[derive(Debug, Serialize, Clone)]
pub struct WelcomePayload {
    pub party_id:     String,
    pub user_id:      String,
    pub display_name: String,
    pub is_host:      bool,
    pub movie_id:     String,
    /// Current playback state so the joining participant can catch up instantly.
    pub current_ts_ms: u64,
    pub paused:        bool,
    pub participants:  Vec<ParticipantInfo>,
}

#[derive(Debug, Serialize, Clone)]
pub struct ParticipantInfo {
    pub user_id:      String,
    pub display_name: String,
    pub is_host:      bool,
}

#[derive(Debug, Serialize, Clone)]
pub struct ParticipantPayload {
    pub user_id:      String,
    pub display_name: String,
    pub is_host:      bool,
    pub participant_count: usize,
}

#[derive(Debug, Serialize, Clone)]
pub struct SeekCorrectionPayload {
    pub ts_ms:  u64,
    pub paused: bool,
}

#[derive(Debug, Serialize, Clone)]
pub struct ChatMessage {
    pub id:           String,   // UUID for deduplication
    pub user_id:      String,
    pub display_name: String,
    pub text:         String,
    pub ts_ms:        u64,      // server-side timestamp in ms
}

#[derive(Debug, Serialize, Clone)]
pub struct ReactionMessage {
    pub user_id:      String,
    pub display_name: String,
    pub emoji:        String,
}

#[derive(Debug, Serialize, Clone)]
pub struct MovieChangedPayload {
    pub movie_id: String,
}