package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
)

// ─── Google OAuth2 provider ───────────────────────────────────────────────────

// GoogleConfig holds the Google OAuth2 application credentials.
// Injected from Vault via the config system — never hardcoded.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
}

type googleProvider struct {
	cfg    GoogleConfig
	client *http.Client
}

func NewGoogleProvider(cfg GoogleConfig) OAuthProvider {
	return &googleProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *googleProvider) BuildAuthURL(state, pkceChallenge, redirectURI string) string {
	v := url.Values{}
	v.Set("client_id", g.cfg.ClientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("response_type", "code")
	v.Set("scope", "openid email profile")
	v.Set("state", state)
	v.Set("code_challenge", pkceChallenge)
	v.Set("code_challenge_method", "S256")
	v.Set("access_type", "offline")
	v.Set("prompt", "select_account")
	return "https://accounts.google.com/o/oauth2/v2/auth?" + v.Encode()
}

func (g *googleProvider) ExchangeCode(ctx context.Context, code, pkceVerifier, redirectURI string) (*domain.OAuthUserInfo, error) {
	tokenResp, err := g.exchangeCodeForTokens(ctx, code, pkceVerifier, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("google: token exchange: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("google: build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: userinfo fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: userinfo status %d", resp.StatusCode)
	}

	var userInfo struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("google: decode userinfo: %w", err)
	}

	return &domain.OAuthUserInfo{
		Provider:    domain.AuthProvider(domain.ProviderGoogle),
		ProviderID:  userInfo.Sub,
		Email:       userInfo.Email,
		DisplayName: userInfo.Name,
		AvatarURL:   userInfo.Picture,
		Verified:    userInfo.EmailVerified,
	}, nil
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

func (g *googleProvider) exchangeCodeForTokens(ctx context.Context, code, verifier, redirectURI string) (*googleTokenResponse, error) {
	body := url.Values{}
	body.Set("code", code)
	body.Set("client_id", g.cfg.ClientID)
	body.Set("client_secret", g.cfg.ClientSecret)
	body.Set("redirect_uri", redirectURI)
	body.Set("grant_type", "authorization_code")
	body.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/token",
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: token endpoint status %d", resp.StatusCode)
	}

	var tokens googleTokenResponse
	return &tokens, json.NewDecoder(resp.Body).Decode(&tokens)
}

// ─── Apple Sign-In provider ───────────────────────────────────────────────────

// AppleConfig holds Sign In with Apple credentials.
// All values are injected from Vault — never hardcoded.
type AppleConfig struct {
	ClientID   string // Service ID, e.g. "com.viewaura.app"
	TeamID     string
	KeyID      string
	PrivateKey string // PEM-encoded ES256 private key
}

type appleProvider struct {
	cfg    AppleConfig
	client *http.Client
}

func NewAppleProvider(cfg AppleConfig) OAuthProvider {
	return &appleProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *appleProvider) BuildAuthURL(state, pkceChallenge, redirectURI string) string {
	v := url.Values{}
	v.Set("client_id", a.cfg.ClientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("response_type", "code")
	v.Set("scope", "name email")
	v.Set("response_mode", "form_post")
	v.Set("state", state)
	v.Set("code_challenge", pkceChallenge)
	v.Set("code_challenge_method", "S256")
	return "https://appleid.apple.com/auth/authorize?" + v.Encode()
}

func (a *appleProvider) ExchangeCode(ctx context.Context, code, pkceVerifier, redirectURI string) (*domain.OAuthUserInfo, error) {
	clientSecret, err := a.buildClientSecret()
	if err != nil {
		return nil, fmt.Errorf("apple: build client secret: %w", err)
	}

	body := url.Values{}
	body.Set("code", code)
	body.Set("client_id", a.cfg.ClientID)
	body.Set("client_secret", clientSecret)
	body.Set("redirect_uri", redirectURI)
	body.Set("grant_type", "authorization_code")
	body.Set("code_verifier", pkceVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://appleid.apple.com/auth/token",
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apple: token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple: token endpoint status %d", resp.StatusCode)
	}

	var tokenResp struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("apple: decode token response: %w", err)
	}

	claims, err := parseAppleIDToken(tokenResp.IDToken)
	if err != nil {
		return nil, fmt.Errorf("apple: parse id_token: %w", err)
	}

	return &domain.OAuthUserInfo{
		Provider:    domain.AuthProvider(domain.ProviderApple),
		ProviderID:  claims.Sub,
		Email:       claims.Email,
		DisplayName: claims.Name,
		Verified:    claims.EmailVerified,
	}, nil
}

// buildClientSecret creates the short-lived ES256 JWT that Apple requires as
// the client_secret on token requests.
//
// TODO: wire in golang-jwt/jwt with crypto/ecdsa to sign with the Vault-sourced
// private key.  Payload: iss=TeamID, iat=now, exp=now+5min,
// aud="https://appleid.apple.com", sub=ClientID, kid header=KeyID.
func (a *appleProvider) buildClientSecret() (string, error) {
	// Stub — replace with real ES256 JWT signing in production.
	// The private key is available as a.cfg.PrivateKey (PEM, from Vault).
	_ = a.cfg.PrivateKey
	return "", fmt.Errorf("apple: buildClientSecret not yet implemented — wire in golang-jwt/jwt with ES256")
}

type appleIDTokenClaims struct {
	Sub           string
	Email         string
	Name          string
	EmailVerified bool
}

// parseAppleIDToken decodes the Apple id_token JWT payload.
//
// Production note: the signature should be verified against Apple's published
// JWKS at https://appleid.apple.com/auth/keys before trusting any claims.
// This implementation only decodes the payload — add signature verification
// before shipping to production.
func parseAppleIDToken(idToken string) (*appleIDTokenClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("apple: malformed id_token — expected 3 segments, got %d", len(parts))
	}

	payload, err := decodeBase64URLSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("apple: decode id_token payload: %w", err)
	}

	var raw struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		// Apple sends email_verified as a boolean or the string "true".
		EmailVerified any `json:"email_verified"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("apple: unmarshal id_token claims: %w", err)
	}

	verified := raw.EmailVerified == true || raw.EmailVerified == "true"
	return &appleIDTokenClaims{
		Sub:           raw.Sub,
		Email:         raw.Email,
		EmailVerified: verified,
	}, nil
}

// decodeBase64URLSegment decodes a single JWT segment (URL-safe base64,
// no padding).  Uses encoding/base64 directly — no inline function stubs.
func decodeBase64URLSegment(segment string) ([]byte, error) {
	// JWT segments use RawURLEncoding (no padding characters).
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		// Some implementations still include padding — try standard URL encoding.
		data, err = base64.URLEncoding.DecodeString(segment)
		if err != nil {
			return nil, fmt.Errorf("base64url decode: %w", err)
		}
	}
	return data, nil
}
