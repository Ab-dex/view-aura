package jwt

import (
	"crypto/rsa"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// New creates a Signer for a specific claims type T.
func New[T jwt.Claims](cfg Config, opts ...Option) (Signer[T], error) {
	switch cfg.Method {
	case HS256:
		if len(cfg.Secret) == 0 {
			return nil, fmt.Errorf("jwt: missing secret for HS256")
		}
		return NewHS256[T](cfg.Secret), nil

	case RS256:
		// Extract RSA keys from options
		o := &options{}
		for _, opt := range opts {
			opt(o)
		}

		if o.priv == nil || o.pub == nil {
			return nil, fmt.Errorf("jwt: missing RSA keys for RS256")
		}
		return NewRS256[T](o.priv, o.pub), nil

	default:
		return nil, fmt.Errorf("jwt: unsupported method %s", cfg.Method)
	}
}

// Option helpers
type options struct {
	priv *rsa.PrivateKey
	pub  *rsa.PublicKey
}
type Option func(*options)

func WithRSA(priv *rsa.PrivateKey, pub *rsa.PublicKey) Option {
	return func(o *options) {
		o.priv = priv
		o.pub = pub
	}
}
