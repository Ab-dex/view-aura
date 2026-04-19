use anyhow::{bail, Context};
use jsonwebtoken::{decode, Algorithm, DecodingKey, Validation};
use serde::Deserialize;
use std::path::Path;

use crate::config::JwtConfig;

/// Claims embedded in the ViewAura access JWT.
/// Must match the Go monolith's JWT claims struct.
#[derive(Debug, Deserialize, Clone)]
pub struct Claims {
    pub sub:  String,   // user_id
    pub role: String,
    pub exp:  u64,
    pub iss:  String,
    /// Optional display name — embedded as a custom claim for fast reads.
    #[serde(default)]
    pub name: String,
}

/// JwtValidator validates RS256 JWTs using a public key loaded from disk.
/// The key is loaded once at startup and reused for every connection.
#[derive(Clone)]
pub struct JwtValidator {
    decoding_key: DecodingKey,
    validation:   Validation,
    /// If true, validation is skipped (local dev without keys).
    skip:         bool,
}

impl JwtValidator {
    /// Load the RS256 public key from `cfg.public_key_path`.
    /// If the path is empty, JWT validation is disabled (local dev mode).
    pub fn new(cfg: &JwtConfig) -> anyhow::Result<Self> {
        if cfg.public_key_path.is_empty() {
            tracing::warn!("JWT public key path is empty — JWT validation DISABLED (local dev only)");
            return Ok(Self {
                decoding_key: DecodingKey::from_secret(b"unused"),
                validation:   Validation::new(Algorithm::RS256),
                skip:         true,
            });
        }

        let pem = std::fs::read(&cfg.public_key_path)
            .with_context(|| format!("reading JWT public key from {}", cfg.public_key_path))?;

        let decoding_key = DecodingKey::from_rsa_pem(&pem)
            .context("parsing RS256 public key PEM")?;

        let mut validation = Validation::new(Algorithm::RS256);
        validation.set_issuer(&[&cfg.issuer]);
        validation.validate_exp = true;

        Ok(Self { decoding_key, validation, skip: false })
    }

    /// Validate `token` and return the embedded claims on success.
    pub fn validate(&self, token: &str) -> anyhow::Result<Claims> {
        if self.skip {
            // Return a synthetic claims struct so local dev works without a key.
            return Ok(Claims {
                sub:  "local-dev-user".to_string(),
                role: "user".to_string(),
                exp:  u64::MAX,
                iss:  "local".to_string(),
                name: "Local Dev User".to_string(),
            });
        }

        let data = decode::<Claims>(token, &self.decoding_key, &self.validation)
            .context("invalid or expired JWT")?;

        Ok(data.claims)
    }

    /// Extract the Bearer token from an Authorization header value.
    pub fn extract_bearer(header: &str) -> anyhow::Result<&str> {
        header
            .strip_prefix("Bearer ")
            .ok_or_else(|| anyhow::anyhow!("Authorization header must start with 'Bearer '"))
    }
}