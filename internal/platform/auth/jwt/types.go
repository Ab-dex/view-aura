package jwt

import "time"

type Claims struct {
	Subject   string
	Issuer    string
	IssuedAt  time.Time
	ExpiresAt time.Time
	ID        string
	Extra     map[string]any
}
