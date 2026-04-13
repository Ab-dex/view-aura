package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

func NewRS256[T jwt.Claims](priv *rsa.PrivateKey, pub *rsa.PublicKey) Signer[T] {
	return &RS256Signer[T]{privateKey: priv, publicKey: pub}
}

func (s *RS256Signer[T]) Sign(claims T) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *RS256Signer[T]) Verify(tokenStr string, out T) error {
	token, err := jwt.ParseWithClaims(tokenStr, out, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil || !token.Valid {
		return fmt.Errorf("invalid token: %w", err)
	}
	return nil
}

func LoadRSAKeys(privPath, pubPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privBytes, err := os.ReadFile(privPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(privBytes)
	if block == nil {
		return nil, nil, fmt.Errorf("decode private key PEM")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8
		key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, nil, fmt.Errorf("parse private key: %w / %w", err, err2)
		}
		var ok bool
		privKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("private key is not RSA")
		}
	}

	pubBytes, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read public key: %w", err)
	}
	block, _ = pem.Decode(pubBytes)
	if block == nil {
		return nil, nil, fmt.Errorf("decode public key PEM")
	}
	pubIface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse public key: %w", err)
	}
	pubKey, ok := pubIface.(*rsa.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("public key is not RSA")
	}

	return privKey, pubKey, nil
}
