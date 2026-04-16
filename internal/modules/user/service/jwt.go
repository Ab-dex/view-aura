package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	"github.com/Ab-dex/view-aura/internal/modules/user/repository"
	jwtlib "github.com/Ab-dex/view-aura/internal/platform/auth/jwt"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenService interface {
	Issue(ctx context.Context, user *domain.User) (*domain.TokenPair, error)
	ValidateAccess(ctx context.Context, tokenStr string) (*Claims, error)
	ValidateRefresh(ctx context.Context, tokenStr string) (*Claims, error)
	Invalidate(ctx context.Context, jti string, ttl time.Duration) error
}

type Claims struct {
	jwt.RegisteredClaims
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
}

type tokenService struct {
	signer   jwtlib.Signer[*Claims]
	cfg      config.JWTConfig
	sessions repository.SessionRepository
}

func NewTokenService(cfg config.JWTConfig, sessions repository.SessionRepository) (TokenService, error) {
	// 1. You MUST pass the Type [*Claims] to jwtlib.New
	// 2. Pass the config directly
	fmt.Printf("Initializing JWT signer with config: %+v\n", cfg)
	var signer jwtlib.Signer[*Claims]
	var err error

	hasSecret := cfg.Secret != ""
	hasKeys := cfg.PrivateKeyPath != "" && cfg.PublicKeyPath != ""

	switch {
	case hasSecret:
		signer, err = jwtlib.New[*Claims](jwtlib.Config{
			Method: jwtlib.HS256, // or map from cfg.Method safely
			Secret: []byte(cfg.Secret),
			Issuer: cfg.Issuer,
		})

	case hasKeys:
		signer, err = jwtlib.New[*Claims](jwtlib.Config{
			Method:         jwtlib.RS256,
			PrivateKeyPath: cfg.PrivateKeyPath,
			PublicKeyPath:  cfg.PublicKeyPath,
			Issuer:         cfg.Issuer,
		})

	default:
		return nil, fmt.Errorf("either jwt.secret or both jwt.private_key_path and jwt.public_key_path must be provided")
	}

	if err != nil {
		return nil, fmt.Errorf("token service: init signer: %w", err)
	}

	return &tokenService{
		signer:   signer,
		cfg:      cfg,
		sessions: sessions,
	}, nil
}

func (s *tokenService) Issue(ctx context.Context, user *domain.User) (*domain.TokenPair, error) {
	accessToken, err := s.issueToken(user, "access", s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.issueToken(user, "refresh", s.cfg.RefreshTokenTTL)
	if err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

func (s *tokenService) issueToken(user *domain.User, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			Issuer:    s.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.New().String(),
		},
		Role:      string(user.Role),
		TokenType: tokenType,
	}

	return s.signer.Sign(claims)
}

func (s *tokenService) ValidateAccess(ctx context.Context, tokenStr string) (*Claims, error) {
	return s.validate(ctx, tokenStr, "access")
}

func (s *tokenService) ValidateRefresh(ctx context.Context, tokenStr string) (*Claims, error) {
	return s.validate(ctx, tokenStr, "refresh")
}

func (s *tokenService) validate(ctx context.Context, tokenStr string, expectedType string) (*Claims, error) {
	var claims Claims

	if err := s.signer.Verify(tokenStr, &claims); err != nil {
		return nil, apierror.ErrTokenInvalid.WithCause(err)
	}

	if claims.Issuer != s.cfg.Issuer {
		return nil, apierror.ErrTokenInvalid.WithCause(fmt.Errorf("invalid issuer"))
	}

	if claims.TokenType != expectedType {
		return nil, apierror.ErrTokenInvalid.WithCause(fmt.Errorf("invalid token type"))
	}

	if claims.ID == "" {
		return nil, apierror.ErrTokenInvalid.WithCause(fmt.Errorf("missing jti"))
	}

	blocked, err := s.sessions.IsBlocklisted(ctx, claims.ID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, apierror.ErrSessionRevoked
	}

	return &claims, nil
}

func (s *tokenService) Invalidate(ctx context.Context, jti string, remainingTTL time.Duration) error {
	return s.sessions.AddToBlocklist(ctx, jti, int64(remainingTTL.Seconds()))
}

// ─── Password hashing ──────────────────────────────────────────────────────────

// HashPassword returns a SHA-256 of token for use as refresh-token-hash stored
// in user_sessions. The full bcrypt hash for passwords is separate.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(h[:])
}

// GenerateSecureToken creates a cryptographically random opaque token.
func GenerateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
