package jwt

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

func NewHS256[T jwt.Claims](secret []byte) Signer[T] {
	return &HS256Signer[T]{secret: secret}
}

func (s *HS256Signer[T]) Sign(claims T) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *HS256Signer[T]) Verify(tokenStr string, out T) error {
	token, err := jwt.ParseWithClaims(tokenStr, out, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil || !token.Valid {
		return fmt.Errorf("invalid token: %w", err)
	}
	return nil
}
