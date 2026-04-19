use std::sync::Arc;
use std::time::Duration;

use axum::{
    extract::{
        ws::{Message, WebSocket, WebSocketUpgrade},
        Path, Query, State,
    },
    http::StatusCode,
    response::{IntoResponse, Json},
    routing::{get, post},
    Router,
};
use serde::{Deserialize, Serialize};
use tokio::sync::mpsc;
use tower_http::{cors::{Any, CorsLayer}, trace::TraceLayer, timeout::TimeoutLayer};
use tracing::{error, info, warn};
use uuid::Uuid;

use crate::{
    auth::JwtValidator,
    config::Settings,
    protocol::{
        ChatMessage, ClientMessage, ParticipantInfo, ParticipantPayload,
        ReactionMessage, ServerMessage, WelcomePayload,
    },
    redis_bus::{pubsub_channel, RedisBus},
    room::{ConnectionInfo, RoomRegistry},
};

// ─── App state ────────────────────────────────────────────────────────────────

#[derive(Clone)]
pub struct AppState {
    pub settings:  Arc<Settings>,
    pub registry:  Arc<RoomRegistry>,
    pub redis:     RedisBus,
    pub jwt:       Arc<JwtValidator>,
}

// ─── Router ───────────────────────────────────────────────────────────────────

pub fn router(state: AppState) -> Router {
    Router::new()
        .route("/health",                        get(health_handler))
        .route("/api/v1/parties",                post(create_party_handler))
        .route("/api/v1/parties/:party_id",      get(party_info_handler))
        .route("/ws/party/:party_id",            get(ws_handler))
        .with_state(state)
        .layer(CorsLayer::new().allow_origin(Any).allow_methods(Any).allow_headers(Any))
        .layer(TraceLayer::new_for_http())
        .layer(TimeoutLayer::new(Duration::from_secs(30)))
}

// ─── REST handlers ────────────────────────────────────────────────────────────

async fn health_handler() -> impl IntoResponse {
    Json(serde_json::json!({ "status": "ok", "service": "watch-party-service" }))
}

#[derive(Deserialize)]
struct CreatePartyRequest {
    movie_id: String,
}

#[derive(Serialize)]
struct CreatePartyResponse {
    party_id:  String,
    ws_url:    String,
    invite_url: String,
}

async fn create_party_handler(
    State(mut state): State<AppState>,
    Json(req): Json<CreatePartyRequest>,
) -> impl IntoResponse {
    let party_id = Uuid::new_v4().to_string();

    let party_state = crate::redis_bus::PartyState {
        host_user_id: String::new(),   // set when host connects via WS
        movie_id:     req.movie_id,
        ts_ms:        0,
        paused:       true,
    };

    if let Err(e) = state.redis.create_party(&party_id, &party_state).await {
        error!(error = %e, "failed to create party in Redis");
        return (StatusCode::INTERNAL_SERVER_ERROR,
            Json(serde_json::json!({ "error": "failed to create party" }))).into_response();
    }

    Json(CreatePartyResponse {
        party_id:   party_id.clone(),
        ws_url:     format!("/ws/party/{party_id}"),
        invite_url: format!("https://viewaura.com/party/{party_id}"),
    }).into_response()
}

async fn party_info_handler(
    State(mut state): State<AppState>,
    Path(party_id): Path<String>,
) -> impl IntoResponse {
    match state.redis.get_party(&party_id).await {
        Ok(Some(ps)) => Json(serde_json::json!({
            "party_id":    party_id,
            "movie_id":    ps.movie_id,
            "ts_ms":       ps.ts_ms,
            "paused":      ps.paused,
            "participant_count": state.registry.local_count(&party_id),
        })).into_response(),
        Ok(None) => (StatusCode::NOT_FOUND,
            Json(serde_json::json!({ "error": "party not found" }))).into_response(),
        Err(e) => {
            error!(error = %e, "redis error");
            (StatusCode::INTERNAL_SERVER_ERROR,
                Json(serde_json::json!({ "error": "internal error" }))).into_response()
        }
    }
}

// ─── WebSocket upgrade ────────────────────────────────────────────────────────

#[derive(Deserialize)]
struct WsQuery {
    /// Bearer token passed as a query parameter on the WS upgrade request.
    /// WebSocket browsers cannot set the Authorization header, so the token
    /// is passed via ?token=<jwt>. It is validated before the upgrade completes.
    token: Option<String>,
}

async fn ws_handler(
    ws:              WebSocketUpgrade,
    State(state):    State<AppState>,
    Path(party_id):  Path<String>,
    Query(query):    Query<WsQuery>,
) -> impl IntoResponse {
    // ── Validate JWT before the upgrade ──────────────────────────────────────
    let token = match query.token {
        Some(t) => t,
        None    => return (StatusCode::UNAUTHORIZED,
            "missing token query parameter").into_response(),
    };

    let claims = match state.jwt.validate(&token) {
        Ok(c)  => c,
        Err(e) => {
            warn!(error = %e, "JWT validation failed on WS upgrade");
            return (StatusCode::UNAUTHORIZED, "invalid or expired token").into_response();
        }
    };

    // ── Upgrade to WebSocket ──────────────────────────────────────────────────
    ws.on_upgrade(move |socket| {
        handle_connection(socket, state, party_id, claims)
    })
}

// ─── Connection handler ───────────────────────────────────────────────────────

async fn handle_connection(
    socket:   WebSocket,
    mut state: AppState,
    party_id: String,
    claims:   crate::auth::Claims,
) {
    let conn_id      = Uuid::new_v4().to_string();
    let user_id      = claims.sub.clone();
    let display_name = if claims.name.is_empty() { user_id.clone() } else { claims.name.clone() };

    info!(
        conn_id = %conn_id,
        user_id = %user_id,
        party_id = %party_id,
        "WebSocket connection established"
    );

    // ── Verify the party exists ───────────────────────────────────────────────
    let party_state = match state.redis.get_party(&party_id).await {
        Ok(Some(ps)) => ps,
        Ok(None)     => {
            let _ = socket.send(Message::Text(
                to_json(&ServerMessage::Error {
                    code:    "PARTY_NOT_FOUND".to_string(),
                    message: "This party does not exist or has ended.".to_string(),
                })
            )).await;
            return;
        }
        Err(e) => {
            error!(error = %e, "Redis error on party lookup");
            return;
        }
    };

    // ── Determine if this user is the host ────────────────────────────────────
    // The first user to create the party in the REST endpoint becomes the host
    // when they connect. Subsequent connections are participants.
    let is_host = party_state.host_user_id.is_empty()
        || party_state.host_user_id == user_id;

    // ── Register connection in-process ────────────────────────────────────────
    let (tx, mut rx) = mpsc::unbounded_channel::<ServerMessage>();
    let info = ConnectionInfo {
        conn_id:      conn_id.clone(),
        user_id:      user_id.clone(),
        display_name: display_name.clone(),
        is_host,
        last_ts_ms:   party_state.ts_ms,
    };
    let is_first = state.registry.join(&party_id, info, tx);

    // ── Add to Redis members set ───────────────────────────────────────────────
    let _ = state.redis.add_member(&party_id, &user_id).await;
    let participant_count = state.registry.local_count(&party_id);

    // ── Send Welcome to the joining client ────────────────────────────────────
    let participants: Vec<ParticipantInfo> = state.registry
        .list_participants(&party_id)
        .iter()
        .map(|p| ParticipantInfo {
            user_id:      p.user_id.clone(),
            display_name: p.display_name.clone(),
            is_host:      p.is_host,
        })
        .collect();

    let welcome = ServerMessage::Welcome(WelcomePayload {
        party_id:      party_id.clone(),
        user_id:       user_id.clone(),
        display_name:  display_name.clone(),
        is_host,
        movie_id:      party_state.movie_id.clone(),
        current_ts_ms: party_state.ts_ms,
        paused:        party_state.paused,
        participants,
    });
    let _ = state.registry.send_to(&party_id, &conn_id, welcome);

    // ── Notify others that someone joined ────────────────────────────────────
    let joined_msg = ServerMessage::ParticipantJoined(ParticipantPayload {
        user_id:           user_id.clone(),
        display_name:      display_name.clone(),
        is_host,
        participant_count,
    });
    state.registry.broadcast(&party_id, joined_msg.clone(), Some(&conn_id));
    let _ = state.redis.publish_to_room(&party_id, &to_json(&joined_msg)).await;

    // ── Split socket into send/recv halves ────────────────────────────────────
    use futures::{SinkExt, StreamExt};
    let (mut ws_tx, mut ws_rx) = socket.split();

    // Spawn a task that drains the channel and forwards to the WebSocket.
    let send_task = tokio::spawn(async move {
        while let Some(msg) = rx.recv().await {
            let text = to_json(&msg);
            if ws_tx.send(Message::Text(text)).await.is_err() {
                break;
            }
        }
    });

    // ── Subscribe to Redis pub/sub for this party ─────────────────────────────
    // Each pod spawns one pub/sub listener per party (on first connection).
    // Subsequent connections on this pod share the same broadcast.
    if is_first {
        let channel   = pubsub_channel(&party_id);
        let registry  = state.registry.clone();
        let party_id2 = party_id.clone();

        tokio::spawn(async move {
            let client = match redis::Client::open("redis://localhost:6379") {
                Ok(c) => c,
                Err(e) => { error!(error = %e, "redis pubsub client failed"); return; }
            };
            let mut pubsub = match client.get_async_pubsub().await {
                Ok(p) => p,
                Err(e) => { error!(error = %e, "redis pubsub connect failed"); return; }
            };
            let _ = pubsub.subscribe(&channel).await;

            use futures::StreamExt;
            let mut stream = pubsub.on_message();
            while let Some(msg) = stream.next().await {
                if let Ok(payload) = msg.get_payload::<String>() {
                    if let Ok(server_msg) = serde_json::from_str::<ServerMessage>(&payload) {
                        registry.broadcast_all(&party_id2, server_msg);
                    }
                }
            }
        });
    }

    // ── Main receive loop ──────────────────────────────────────────────────────
    let max_drift      = state.settings.party.max_drift_ms;
    let party_settings = state.settings.party.clone();

    while let Some(Ok(msg)) = ws_rx.next().await {
        match msg {
            Message::Text(text) => {
                match serde_json::from_str::<ClientMessage>(&text) {
                    Ok(client_msg) => {
                        handle_client_message(
                            client_msg,
                            &party_id,
                            &conn_id,
                            &user_id,
                            &display_name,
                            is_host,
                            max_drift,
                            &state,
                        ).await;
                    }
                    Err(e) => {
                        warn!(error = %e, "malformed client message");
                        state.registry.send_to(&party_id, &conn_id, ServerMessage::Error {
                            code:    "INVALID_MESSAGE".to_string(),
                            message: format!("malformed message: {e}"),
                        });
                    }
                }
            }
            Message::Close(_) => break,
            Message::Ping(data) => {
                // axum handles Pong automatically, but we also send an app-level Pong.
                state.registry.send_to(&party_id, &conn_id, ServerMessage::Pong);
                let _ = data; // suppress unused warning
            }
            _ => {}
        }
    }

    // ── Clean up on disconnect ────────────────────────────────────────────────
    info!(conn_id = %conn_id, user_id = %user_id, party_id = %party_id, "WebSocket disconnected");

    let is_last = state.registry.leave(&party_id, &conn_id);
    let _ = state.redis.remove_member(&party_id, &user_id).await;

    let remaining = state.registry.local_count(&party_id);
    let left_msg = ServerMessage::ParticipantLeft(ParticipantPayload {
        user_id:           user_id.clone(),
        display_name,
        is_host,
        participant_count: remaining,
    });
    state.registry.broadcast_all(&party_id, left_msg.clone());
    let _ = state.redis.publish_to_room(&party_id, &to_json(&left_msg)).await;

    // If host disconnected, end the party.
    if is_host && is_last {
        info!(party_id = %party_id, "host left — ending party");
        state.registry.broadcast_all(&party_id, ServerMessage::PartyEnded);
        let _ = state.redis.delete_party(&party_id).await;
    }

    send_task.abort();
}

// ─── Client message dispatcher ────────────────────────────────────────────────

#[allow(clippy::too_many_arguments)]
async fn handle_client_message(
    msg:          ClientMessage,
    party_id:     &str,
    conn_id:      &str,
    user_id:      &str,
    display_name: &str,
    is_host:      bool,
    max_drift_ms: u64,
    state:        &AppState,
) {
    use ClientMessage::*;

    match msg {
        Sync(payload) => {
            if !is_host {
                // Only the host may send sync events.
                state.registry.send_to(party_id, conn_id, ServerMessage::Error {
                    code:    "FORBIDDEN".to_string(),
                    message: "only the host may broadcast sync state".to_string(),
                });
                return;
            }

            // Update in-process timestamp for drift detection.
            state.registry.update_ts(party_id, conn_id, payload.ts_ms);

            // Relay sync to all participants on this pod.
            state.registry.broadcast(
                party_id,
                ServerMessage::Sync(payload.clone()),
                Some(conn_id),
            );

            // Fan-out to other pods via Redis pub/sub.
            let _ = state.redis.publish_to_room(
                party_id,
                &to_json(&ServerMessage::Sync(payload.clone())),
            ).await;

            // Apply drift correction to lagging participants.
            state.registry.apply_drift_correction(
                party_id,
                conn_id,
                payload.ts_ms,
                max_drift_ms,
                payload.paused,
            );
        }

        Chat(payload) => {
            let text = payload.text.chars().take(500).collect::<String>();
            let chat_msg = ChatMessage {
                id:           Uuid::new_v4().to_string(),
                user_id:      user_id.to_string(),
                display_name: display_name.to_string(),
                text,
                ts_ms:        chrono::Utc::now().timestamp_millis() as u64,
            };

            let server_msg = ServerMessage::Chat(chat_msg.clone());

            // Persist to Redis Stream for replay.
            let _ = state.redis
                .publish_to_room(party_id, &to_json(&server_msg))
                .await;

            // Fan-out locally.
            state.registry.broadcast_all(party_id, server_msg);
        }

        Reaction(payload) => {
            // Reactions are ephemeral — no persistence.
            let server_msg = ServerMessage::Reaction(ReactionMessage {
                user_id:      user_id.to_string(),
                display_name: display_name.to_string(),
                emoji:        payload.emoji,
            });
            state.registry.broadcast_all(party_id, server_msg.clone());
            let _ = state.redis.publish_to_room(party_id, &to_json(&server_msg)).await;
        }

        PlayPause(payload) => {
            if !is_host { return; }
            let server_msg = ServerMessage::Sync(crate::protocol::SyncPayload {
                ts_ms:  payload.ts_ms,
                paused: payload.paused,
            });
            state.registry.broadcast(party_id, server_msg.clone(), Some(conn_id));
            let _ = state.redis.publish_to_room(party_id, &to_json(&server_msg)).await;
        }

        Seek(payload) => {
            if !is_host { return; }
            let server_msg = ServerMessage::SeekCorrection(
                crate::protocol::SeekCorrectionPayload { ts_ms: payload.ts_ms, paused: false },
            );
            state.registry.broadcast(party_id, server_msg.clone(), Some(conn_id));
            let _ = state.redis.publish_to_room(party_id, &to_json(&server_msg)).await;
        }

        ChangeMovie(payload) => {
            if !is_host { return; }
            let server_msg = ServerMessage::MovieChanged(
                crate::protocol::MovieChangedPayload { movie_id: payload.movie_id },
            );
            state.registry.broadcast_all(party_id, server_msg.clone());
            let _ = state.redis.publish_to_room(party_id, &to_json(&server_msg)).await;
        }

        Kick(payload) => {
            if !is_host { return; }
            // Find the connection for the kicked user and send Kicked message.
            if let Some(participants) = Some(state.registry.list_participants(party_id)) {
                for p in participants {
                    if p.user_id == payload.user_id {
                        state.registry.send_to(
                            party_id,
                            &p.conn_id,
                            ServerMessage::Kicked { reason: "Removed by host".to_string() },
                        );
                    }
                }
            }
        }

        Ping => {
            state.registry.send_to(party_id, conn_id, ServerMessage::Pong);
        }
    }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

fn to_json(msg: &ServerMessage) -> String {
    serde_json::to_string(msg).unwrap_or_else(|_| r#"{"type":"error"}"#.to_string())
}