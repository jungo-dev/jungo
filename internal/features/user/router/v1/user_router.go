package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/jungo-dev/junkit/response"

	"jungo/internal/config"
	v1handler "jungo/internal/features/user/handler/v1"
	middleware "jungo/internal/middleware"
)

// UserRoutes registers the user feature's endpoints under a version group.
type UserRoutes struct {
	handler   *v1handler.UserHandler
	cfg       *config.Config
	responder response.Responder
}

// NewUserRoutes creates a UserRoutes.
func NewUserRoutes(handler *v1handler.UserHandler, cfg *config.Config, responder response.Responder) *UserRoutes {
	return &UserRoutes{handler: handler, cfg: cfg, responder: responder}
}

// Version implements router.Versioned, nesting these routes under "/api/v1".
func (r *UserRoutes) Version() string {
	return "v1"
}

// Register implements router.Routes.
func (r *UserRoutes) Register(group *gin.RouterGroup) {
	users := group.Group("/users")
	users.Use(middleware.Auth(r.cfg.APIKey, r.responder))

	users.POST("", r.handler.CreateUser)
	users.GET("", r.handler.ListUsers)
	users.GET("/:uuid", r.handler.GetUser)
	users.PATCH("/:uuid", r.handler.UpdateUser)
	users.DELETE("/:uuid", r.handler.DeleteUser)
	users.POST("/:uuid/avatar", r.handler.UploadAvatar)
	users.DELETE("/:uuid/avatar", r.handler.DeleteAvatar)
}
