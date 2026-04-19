pub mod documents;
pub mod settings;

use anyhow::Context;
use meilisearch_sdk::client::Client;
use tracing::{info, warn};

use crate::config::MeiliConfig;
use documents::MovieDocument;
use settings::{movie_index_settings, person_index_settings};

/// MeiliSearchClient is the thin wrapper held by the application state.
#[derive(Clone)]
pub struct MeiliSearchClient {
    pub inner:  Client,
    pub cfg:    MeiliConfig,
}

impl MeiliSearchClient {
    pub fn new(cfg: &MeiliConfig) -> anyhow::Result<Self> {
        let key = if cfg.master_key.is_empty() {
            None
        } else {
            Some(cfg.master_key.as_str())
        };

        let client = Client::new(&cfg.url, key)
            .context("failed to create MeiliSearch client")?;

        Ok(Self { inner: client, cfg: cfg.clone() })
    }

    /// Ensure both indexes exist and have the correct settings applied.
    /// Idempotent — safe to call on every startup.
    pub async fn bootstrap(&self) -> anyhow::Result<()> {
        self.bootstrap_index(&self.cfg.movie_index, movie_index_settings()).await?;
        self.bootstrap_index(&self.cfg.person_index, person_index_settings()).await?;
        Ok(())
    }

    async fn bootstrap_index(
        &self,
        index_name: &str,
        settings: meilisearch_sdk::settings::Settings,
    ) -> anyhow::Result<()> {
        // Create index if it doesn't exist (uid = primary key for documents).
        match self.inner.create_index(index_name, Some("id")).await {
            Ok(task) => {
                info!(index = index_name, "created MeiliSearch index");
                // Wait for the index creation task to complete.
                task.wait_for_completion(&self.inner, None, None)
                    .await
                    .context("waiting for index creation")?;
            }
            Err(e) if e.to_string().contains("already exists") => {
                info!(index = index_name, "index already exists — skipping create");
            }
            Err(e) => {
                warn!(index = index_name, error = %e, "unexpected index create error");
            }
        }

        // Apply settings (searchable fields, filterable attributes, ranking rules).
        let idx = self.inner.index(index_name);
        idx.set_settings(&settings)
            .await
            .context("applying index settings")?
            .wait_for_completion(&self.inner, None, None)
            .await
            .context("waiting for settings task")?;

        info!(index = index_name, "index settings applied");
        Ok(())
    }

    /// Upsert (add or replace) a movie document.
    pub async fn upsert_movie(&self, doc: &MovieDocument) -> anyhow::Result<()> {
        self.inner
            .index(&self.cfg.movie_index)
            .add_or_replace(&[doc], Some("id"))
            .await
            .context("upsert movie document")?;
        Ok(())
    }

    /// Delete a movie document by ID.
    pub async fn delete_movie(&self, movie_id: &str) -> anyhow::Result<()> {
        self.inner
            .index(&self.cfg.movie_index)
            .delete_document(movie_id)
            .await
            .context("delete movie document")?;
        Ok(())
    }
}