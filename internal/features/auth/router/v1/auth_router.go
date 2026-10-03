package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/jungo-dev/junkit/middleware"
	"github.com/jungo-dev/junkit/response"

	"jungo/internal/features/auth/domain"
	v1handler "jungo/internal/features/auth/handler/v1"
)

// AuthRoutes registers the auth feature's endpoints under a version group.
type AuthRoutes struct {
	handler       *v1handler.AuthHandler
	authenticator middleware.Authenticator[*domain.Identity]
	responder     response.Responder
}

// NewAuthRoutes creates a AuthRoutes.
func NewAuthRoutes(handler *v1handler.AuthHandler, authenticator middleware.Authenticator[*domain.Identity], responder response.Responder) *AuthRoutes {
	return &AuthRoutes{handler: handler, authenticator: authenticator, responder: responder}
}

// Version implements router.Versioned, nesting these routes under "/api/v1".
func (r *AuthRoutes) Version() string {
	return "v1"
}

// Register implements router.Routes.
func (r *AuthRoutes) Register(group *gin.RouterGroup) {
	items := group.Group("/auth")

	items.POST("/login", r.handler.Login)
	items.POST("/refresh", r.handler.Refresh)

	protected := items.Group("", middleware.BearerAuth(r.authenticator, r.responder, middleware.BearerAuthOptions{}))
	protected.POST("/logout", r.handler.Logout)
	protected.POST("/logout-all", r.handler.LogoutAll)
	protected.GET("/me", r.handler.Me)
	protected.GET("/sessions", r.handler.Sessions)
}
