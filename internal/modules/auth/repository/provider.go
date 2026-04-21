package repository

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func ProvideAuthProviderRepository(pool *shareddb.Pool) AuthProviderRepository {
	return NewAuthProviderRepository(pool.Pool)
}

func ProvideSessionRepository(rdb *cache.Client) SessionRepository {
	return NewSessionRepository(rdb)
}

func ProvideSecurityRepository(pool *shareddb.Pool) SecurityRepository {
	return NewSecurityRepository(pool.Pool)
}

func ProvideVerificationTokenRepository(pool *shareddb.Pool) VerificationTokenRepository {
	return NewVerificationTokenRepository(pool.Pool)
}

// ProvideMFARepository constructs the composed Postgres+Redis MFA repo.
// The MFA encryption key is sourced from cfg.Auth.MFAEncryptionKey (Vault-injected).
func ProvideMFARepository(pool *shareddb.Pool, rdb *cache.Client, cfg *config.Config) (MFARepository, error) {
	return NewMFARepository(pool.Pool, rdb.Client, []byte(cfg.Auth.MFAEncryptionKey))
}

func ProvideOAuthStateRepository(rdb *cache.Client) OAuthStateRepository {
	return NewOAuthStateRepository(rdb.Client)
}

var ProviderSet = wire.NewSet(
	ProvideAuthProviderRepository,
	ProvideSessionRepository,
	ProvideSecurityRepository,
	ProvideVerificationTokenRepository,
	ProvideMFARepository,
	ProvideOAuthStateRepository,
)
