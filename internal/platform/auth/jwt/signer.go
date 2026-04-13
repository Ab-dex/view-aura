package jwt

import (
	"crypto/rsa"

	"github.com/golang-jwt/jwt/v5"
)

// T must implement jwt.Claims (like jwt.RegisteredClaims or your custom struct).
type Signer[T jwt.Claims] interface {
	Sign(claims T) (string, error)
	Verify(tokenStr string, out T) error
}

type RS256Signer[T jwt.Claims] struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

type HS256Signer[T jwt.Claims] struct {
	secret []byte
}
