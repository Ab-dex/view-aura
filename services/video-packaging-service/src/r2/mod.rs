use anyhow::Context;
use aws_config::Region;
use aws_credential_types::Credentials;
use aws_sdk_s3::{
    config::{Builder as S3ConfigBuilder, SharedCredentialsProvider},
    Client,
    primitives::ByteStream,
};
use std::path::Path;
use tokio::io::AsyncReadExt;
use tracing::{debug, info};

use crate::config::R2Config;

/// R2Client wraps the AWS S3 SDK configured for Cloudflare R2.
#[derive(Clone)]
pub struct R2Client {
    client: Client,
    bucket: String,
}

impl R2Client {
    pub fn new(cfg: &R2Config) -> anyhow::Result<Self> {
        let creds = Credentials::new(
            &cfg.access_key_id,
            &cfg.secret_access_key,
            None,
            None,
            "r2-static",
        );

        let region = Region::new(cfg.region.clone());

        let s3_cfg = S3ConfigBuilder::new()
            .endpoint_url(&cfg.endpoint)
            .region(region)
            .credentials_provider(SharedCredentialsProvider::new(creds))
            .force_path_style(true)      // R2 requires path-style addressing
            .build();

        Ok(Self {
            client: Client::from_conf(s3_cfg),
            bucket: cfg.bucket.clone(),
        })
    }

    /// Upload a single file to R2 at `object_key`.
    pub async fn upload_file(&self, file_path: &Path, object_key: &str) -> anyhow::Result<()> {
        debug!(file = %file_path.display(), key = object_key, "uploading to R2");

        let mut file = tokio::fs::File::open(file_path)
            .await
            .with_context(|| format!("opening file {}", file_path.display()))?;

        let mut bytes = Vec::new();
        file.read_to_end(&mut bytes)
            .await
            .context("reading file bytes")?;

        let content_type = guess_content_type(file_path);

        self.client
            .put_object()
            .bucket(&self.bucket)
            .key(object_key)
            .content_type(content_type)
            .body(ByteStream::from(bytes))
            .send()
            .await
            .with_context(|| format!("uploading {object_key} to R2"))?;

        Ok(())
    }

    /// Upload an entire local directory tree to R2 under `key_prefix`.
    /// Preserves the relative path structure.
    pub async fn upload_directory(
        &self,
        local_dir:  &Path,
        key_prefix: &str,
    ) -> anyhow::Result<Vec<String>> {
        let mut uploaded_keys = Vec::new();
        self.upload_dir_recursive(local_dir, local_dir, key_prefix, &mut uploaded_keys)
            .await?;
        info!(
            count  = uploaded_keys.len(),
            prefix = key_prefix,
            "directory uploaded to R2"
        );
        Ok(uploaded_keys)
    }

    fn upload_dir_recursive<'a>(
        &'a self,
        base:        &'a Path,
        current:     &'a Path,
        key_prefix:  &'a str,
        keys:        &'a mut Vec<String>,
    ) -> std::pin::Pin<Box<dyn std::future::Future<Output = anyhow::Result<()>> + Send + 'a>> {
        Box::pin(async move {
            let mut entries = tokio::fs::read_dir(current)
                .await
                .with_context(|| format!("reading directory {}", current.display()))?;

            while let Some(entry) = entries.next_entry().await? {
                let path = entry.path();
                if path.is_dir() {
                    self.upload_dir_recursive(base, &path, key_prefix, keys).await?;
                } else {
                    let relative = path.strip_prefix(base)
                        .context("stripping base prefix")?;
                    let key = format!("{}/{}", key_prefix, relative.display());
                    self.upload_file(&path, &key).await?;
                    keys.push(key);
                }
            }
            Ok(())
        })
    }
}

fn guess_content_type(path: &Path) -> String {
    match path.extension().and_then(|e| e.to_str()) {
        Some("m3u8") => "application/x-mpegURL".to_string(),
        Some("ts")   => "video/MP2T".to_string(),
        Some("mp4")  => "video/mp4".to_string(),
        Some("jpg") | Some("jpeg") => "image/jpeg".to_string(),
        Some("png")  => "image/png".to_string(),
        Some("webm") => "video/webm".to_string(),
        _            => "application/octet-stream".to_string(),
    }
}