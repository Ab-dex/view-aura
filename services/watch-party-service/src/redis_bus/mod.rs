use anyhow::Context;
use redis::{aio::ConnectionManager, AsyncCommands, Client};
use serde::{Deserialize, Serialize};
use tracing::{debug, warn};

/// PartyState is stored in Redis as a Hash under `watch_party:{party_id}:state`.
/// Updated atomically on every sync event from the host.
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct PartyState {
    pub host_user_id: String,
    pub movie_id:     String,
    pub ts_ms:        u64,
    pub paused:       bool,
}

/// RedisBus provides:
///   - Party state read/write (Redis Hash, 4h TTL)
///   - Member set management (Redis Set)
///   - Chat pub/sub channel (Redis Pub/Sub + Stream)
#[derive(Clone)]
pub struct RedisBus {
    pub cm:             ConnectionManager,
    party_ttl_secs:     u64,
    chat_ttl_secs:      u64,
}

impl RedisBus {
    pub async fn new(url: &str, party_ttl: u64, chat_ttl: u64) -> anyhow::Result<Self> {
        let client = Client::open(url).context("opening Redis connection")?;
        let cm = ConnectionManager::new(client)
            .await
            .context("creating Redis connection manager")?;

        Ok(Self {
            cm,
            party_ttl_secs: party_ttl,
            chat_ttl_secs:  chat_ttl,
        })
    }

    // ─── Party state ──────────────────────────────────────────────────────────

    /// Initialise party state on creation.
    pub async fn create_party(&mut self, party_id: &str, state: &PartyState) -> anyhow::Result<()> {
        let key = party_key(party_id);
        let json = serde_json::to_string(state)?;
        let _: () = self.cm.set_ex(&key, json, self.party_ttl_secs).await?;
        debug!(party_id, "party state created");
        Ok(())
    }

    /// Read the current party state.
    pub async fn get_party(&mut self, party_id: &str) -> anyhow::Result<Option<PartyState>> {
        let key = party_key(party_id);
        let raw: Option<String> = self.cm.get(&key).await?;
        match raw {
            None    => Ok(None),
            Some(s) => Ok(Some(serde_json::from_str(&s)?)),
        }
    }

    /// Update the party state. Refreshes the TTL on every write.
    pub async fn update_party(&mut self, party_id: &str, state: &PartyState) -> anyhow::Result<()> {
        let key = party_key(party_id);
        let json = serde_json::to_string(state)?;
        let _: () = self.cm.set_ex(&key, json, self.party_ttl_secs).await?;
        Ok(())
    }

    /// Delete the party state on party end.
    pub async fn delete_party(&mut self, party_id: &str) -> anyhow::Result<()> {
        let state_key   = party_key(party_id);
        let members_key = members_key(party_id);
        let chat_key    = chat_key(party_id);
        let _: () = redis::pipe()
            .del(&state_key)
            .del(&members_key)
            .del(&chat_key)
            .query_async(&mut self.cm)
            .await?;
        Ok(())
    }

    // ─── Members ──────────────────────────────────────────────────────────────

    pub async fn add_member(&mut self, party_id: &str, user_id: &str) -> anyhow::Result<()> {
        let key = members_key(party_id);
        let _: () = self.cm.sadd(&key, user_id).await?;
        let _: () = self.cm.expire(&key, self.party_ttl_secs as i64).await?;
        Ok(())
    }

    pub async fn remove_member(&mut self, party_id: &str, user_id: &str) -> anyhow::Result<()> {
        let key = members_key(party_id);
        let _: () = self.cm.srem(&key, user_id).await?;
        Ok(())
    }

    pub async fn member_count(&mut self, party_id: &str) -> anyhow::Result<usize> {
        let key = members_key(party_id);
        let n: usize = self.cm.scard(&key).await?;
        Ok(n)
    }

    // ─── Chat pub/sub ─────────────────────────────────────────────────────────

    /// Publish a message to the Redis pub/sub channel for this party.
    /// All service instances subscribed to this channel will receive it.
    pub async fn publish_to_room(&mut self, party_id: &str, msg: &str) -> anyhow::Result<()> {
        let channel = pubsub_channel(party_id);
        let _: () = self.cm.publish(&channel, msg).await?;
        Ok(())
    }

    /// Persist a chat message to the Redis Stream for replay on join.
    /// Trims the stream to the last 500 messages.
    pub async fn append_chat(
        &mut self,
        party_id: &str,
        chat_json: &str,
    ) -> anyhow::Result<()> {
        let key = chat_key(party_id);
        let _: String = self.cm
            .xadd_maxlen(&key, redis::streams::StreamMaxlen::Approx(500), "*",
                &[("msg", chat_json)])
            .await?;
        let _: () = self.cm.expire(&key, self.chat_ttl_secs as i64).await?;
        Ok(())
    }

    /// Read recent chat history (last N messages) for a joining participant.
    pub async fn recent_chat(
        &mut self,
        party_id: &str,
        count: usize,
    ) -> anyhow::Result<Vec<String>> {
        let key = chat_key(party_id);
        let results: Vec<redis::streams::StreamRangeReply> = self.cm
            .xrevrange_count(&key, "+", "-", count)
            .await?;

        let mut messages: Vec<String> = results
            .iter()
            .flat_map(|r| r.ids.iter())
            .filter_map(|entry| {
                entry.map.get("msg").and_then(|v| {
                    if let redis::Value::BulkString(bytes) = v {
                        String::from_utf8(bytes.clone()).ok()
                    } else {
                        None
                    }
                })
            })
            .collect();

        messages.reverse();   // return oldest first
        Ok(messages)
    }
}

// ─── Key helpers ─────────────────────────────────────────────────────────────

pub fn party_key(party_id: &str) -> String {
    format!("watch_party:{}:state", party_id)
}

pub fn members_key(party_id: &str) -> String {
    format!("party:{}:members", party_id)
}

pub fn chat_key(party_id: &str) -> String {
    format!("party:{}:chat", party_id)
}

pub fn pubsub_channel(party_id: &str) -> String {
    format!("party:{}:channel", party_id)
}