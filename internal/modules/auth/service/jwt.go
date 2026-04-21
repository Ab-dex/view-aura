package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	authdomain "github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	authrepo "github.com/Ab-dex/view-aura/internal/modules/auth/repository"
	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
	jwtlib "github.com/Ab-dex/view-aura/internal/platform/auth/jwt"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type TokenService interface {
	Issue(ctx context.Context, user *userdomain.User) (*authdomain.TokenPair, error)
	ValidateAccess(ctx context.Context, tokenStr string) (*Claims, error)
	ValidateRefresh(ctx context.Context, tokenStr string) (*Claims, error)
	Invalidate(ctx context.Context, jti string, ttl time.Duration) error
}

// Claims is the JWT payload used by ViewAura access and refresh tokens.
type Claims struct {
	jwt.RegisteredClaims
	Role      string `json:"role"`
	TokenType string `json:"token_type"` // "access" | "refresh"
}

type tokenService struct {
	signer   jwtlib.Signer[*Claims]
	cfg      config.JWTConfig
	sessions authrepo.SessionRepository
}

// NewTokenService constructs the JWT TokenService.
// The signer is chosen based on config: HS256 (secret) or RS256 (key files).
func NewTokenService(cfg config.JWTConfig) (*tokenService, error) {
	var signer jwtlib.Signer[*Claims]
	var err error

	hasSecret := cfg.Secret != ""
	hasKeys := cfg.PrivateKeyPath != "" && cfg.PublicKeyPath != ""

	switch {
	case hasSecret:
		signer, err = jwtlib.New[*Claims](jwtlib.Config{
			Method: jwtlib.HS256,
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
		return nil, fmt.Errorf("auth: either jwt.secret or both jwt.private_key_path and jwt.public_key_path must be provided")
	}
	if err != nil {
		return nil, fmt.Errorf("auth: init jwt signer: %w", err)
	}
	return &tokenService{signer: signer, cfg: cfg}, nil
}

// SetSessionRepository injects the SessionRepository after construction.
// Called by the Wire-generated provider so the token service can check
// the blocklist on every ValidateAccess call without a circular dependency.
func (s *tokenService) SetSessionRepository(sessions authrepo.SessionRepository) {
	s.sessions = sessions
}

func (s *tokenService) Issue(_ context.Context, user *userdomain.User) (*authdomain.TokenPair, error) {
	accessToken, err := s.issueToken(user, "access", s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.issueToken(user, "refresh", s.cfg.RefreshTokenTTL)
	if err != nil {
		return nil, err
	}
	return &authdomain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

func (s *tokenService) issueToken(user *userdomain.User, tokenType string, ttl time.Duration) (string, error) {
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

func (s *tokenService) validate(ctx context.Context, tokenStr, expectedType string) (*Claims, error) {
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
	if s.sessions != nil {
		blocked, err := s.sessions.IsBlocklisted(ctx, claims.ID)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, apierror.ErrSessionRevoked
		}
	}
	return &claims, nil
}

func (s *tokenService) Invalidate(ctx context.Context, jti string, remainingTTL time.Duration) error {
	if s.sessions == nil {
		return nil
	}
	return s.sessions.AddToBlocklist(ctx, jti, int64(remainingTTL.Seconds()))
}
