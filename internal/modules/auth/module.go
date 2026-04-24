// Package auth is the dedicated authentication module for ViewAura.
//
// It owns the flows that the user module deliberately does not:
//   - Email verification (send + consume one-time token)
//   - Password reset (send link + consume token + re-hash)
//   - OAuth2 social login (Google, Apple) with PKCE
//   - TOTP MFA enrollment, confirmation, recovery codes, and email OTP fallback
//
// The module follows the same four-layer DDD structure as all other modules:
//
//	domain/ → service/ → repository/ → api/http/
//
// It shares the user module's repositories (UserRepository, SessionRepository,
// AuthProviderRepository, SecurityRepository) through injected interfaces so
// auth events and user state remain consistent without a cross-module DB call.
package auth

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/app/middleware"
	"github.com/Ab-dex/view-aura/internal/contract"
	authapi "github.com/Ab-dex/view-aura/internal/modules/auth/api"
	authhttp "github.com/Ab-dex/view-aura/internal/modules/auth/api/http"
	authrepo "github.com/Ab-dex/view-aura/internal/modules/auth/repository"
	authsvc "github.com/Ab-dex/view-aura/internal/modules/auth/service"
)

var _ contract.Module = (*Module)(nil)

// Module wires together the auth handler and its route registrations.
type Module struct {
	Handler *authhttp.AuthHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *authhttp.AuthHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

// Register mounts all auth routes under the given router.
//
// Public routes (verify-email, password-reset, OAuth callback, MFA verify) are
// open so unauthenticated clients can complete these flows.
// Protected routes (TOTP enroll/confirm, MFA disable) require the auth
// middleware applied by the handler itself via its own group.
func (m *Module) Register(r gin.IRouter) {
	authGroup := r.Group("/auth")
	authGroup.Use(middleware.StrictRateLimit(20, time.Minute))

	m.Handler.RegisterRoutes(authGroup)

	protected := authGroup.Group("")
	protected.Use(m.Auth)

	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideAuthModule(h *authhttp.AuthHandler,
	auth contract.AuthMiddleware) contract.AuthModule {
	return contract.AuthModule{Module: NewModule(h, gin.HandlerFunc(auth))}
}

// AuthModuleSet is the complete Wire provider set for the auth module.
// Listed in cmd/api/wire.go alongside all other module sets.
var AuthModuleSet = wire.NewSet(
	authrepo.ProviderSet,
	authsvc.ProviderSet,
	authapi.ProviderSet,
	ProvideAuthModule,
)
