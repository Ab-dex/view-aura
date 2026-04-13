package jwt

type Method string

const (
	HS256 Method = "HS256"
	RS256 Method = "RS256"
)

type Config struct {
	Method Method

	// HMAC
	Secret []byte

	// RSA
	PrivateKeyPath string
	PublicKeyPath  string
	Issuer         string
}
