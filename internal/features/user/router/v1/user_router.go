package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/jungo-dev/junkit/middleware"
	"github.com/jungo-dev/junkit/response"

	authdomain "jungo/internal/features/auth/domain"
	v1handler "jungo/internal/features/user/handler/v1"
)

// UserRoutes registers the user feature's endpoints under a version group.
type UserRoutes struct {
	handler       *v1handler.UserHandler
	authenticator middleware.Authenticator[*authdomain.Identity]
	responder     response.Responder
}

// NewUserRoutes creates a UserRoutes.
func NewUserRoutes(handler *v1handler.UserHandler, authenticator middleware.Authenticator[*authdomain.Identity], responder response.Responder) *UserRoutes {
	return &UserRoutes{handler: handler, authenticator: authenticator, responder: responder}
}

// Version implements router.Versioned, nesting these routes under "/api/v1".
func (r *UserRoutes) Version() string {
	return "v1"
}

// Register implements router.Routes. Every route requires a logged-in user.
func (r *UserRoutes) Register(group *gin.RouterGroup) {
	users := group.Group("/users")
	users.Use(middleware.BearerAuth(r.authenticator, r.responder, middleware.BearerAuthOptions{}))

	users.POST("", r.handler.CreateUser)
	users.GET("", r.handler.ListUsers)
	users.GET("/:uuid", r.handler.GetUser)
	users.PATCH("/:uuid", r.handler.UpdateUser)
	users.DELETE("/:uuid", r.handler.DeleteUser)
	users.POST("/:uuid/avatar", r.handler.UploadAvatar)
	users.DELETE("/:uuid/avatar", r.handler.DeleteAvatar)
}
