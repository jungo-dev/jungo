package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/jungo-dev/junkit/middleware"
	"github.com/jungo-dev/junkit/notification"
	"github.com/jungo-dev/junkit/response"

	"jungo/internal/config"
)

// Routes defines the interface for registering feature routes.
type Routes interface {
	Register(group *gin.RouterGroup)
}

// Versioned defines an API version prefix for routes.
type Versioned interface {
	Version() string
}

// VersionedRoutes combines Routes and Versioned interfaces.
type VersionedRoutes interface {
	Routes
	Versioned
}

// Params defines dependencies for route registration.
type Params struct {
	fx.In

	Router        *gin.Engine
	Config        *config.Config
	Routes        []Routes              `group:"routes"`
	HttpLogger    *zap.Logger           `name:"http"`
	LimiterLogger *zap.Logger           `name:"limiter"`
	Notifier      notification.Notifier `optional:"true"`
	Responder     response.Responder
}

// RegisterAll sets up global middlewares, system routes, and feature routes.
func RegisterAll(r Params) {
	registerGlobalMiddlewares(r)
	r.Router.HandleMethodNotAllowed = true

	r.Router.NoRoute(func(ctx *gin.Context) {
		r.Responder.Send(ctx, http.StatusNotFound, "api_not_found")
	})
	r.Router.NoMethod(func(ctx *gin.Context) {
		r.Responder.Send(ctx, http.StatusNotFound, "method_not_found")
	})
	r.Router.GET("/health", func(ctx *gin.Context) {
		r.Responder.Send(ctx, http.StatusOK, "app_running")
	})
	r.Router.Static("/uploads", r.Config.Storage.BaseDir)

	api := r.Router.Group("/api")
	versionGroups := make(map[string]*gin.RouterGroup)

	for _, route := range r.Routes {
		target := api

		if v, ok := route.(Versioned); ok && v.Version() != "" {
			version := v.Version()
			group, exists := versionGroups[version]
			if !exists {
				group = api.Group("/" + version)
				versionGroups[version] = group
			}
			target = group
		}

		route.Register(target)
	}
}

// registerGlobalMiddlewares attaches global middlewares to the router.
func registerGlobalMiddlewares(r Params) {
	cfg := r.Config

	r.Router.Use(
		middleware.TracerDebug(cfg.TracerDebugKey, cfg.TracerDebugValue),
		middleware.Security(),
		middleware.Trace(middleware.TraceOptions{
			Version:     cfg.Version,
			Commit:      cfg.Commit,
			Environment: cfg.Environment,
		}),
		middleware.CORS(middleware.CORSOptions{
			AllowOrigins:     cfg.CORS.AllowOrigins,
			AllowMethods:     cfg.CORS.AllowMethods,
			AllowHeaders:     cfg.CORS.AllowHeaders,
			AllowCredentials: cfg.CORS.AllowCredentials,
			MaxAge:           cfg.CORS.MaxAge,
		}),
		middleware.Payload(middleware.PayloadOptions{
			MaxBodySize: cfg.Security.MaxRequestBodySize,
		}, r.Responder),
		middleware.AccessLog(r.HttpLogger, middleware.AccessLogOptions{
			SkipIf: func(c *gin.Context) bool {
				return cfg.TracerDebugKey != "" && c.Query(cfg.TracerDebugKey) == cfg.TracerDebugValue
			},
		}),
		middleware.Limiter(middleware.LimiterOptions{
			RequestsPerSecond: cfg.Limiter.RequestsPerSecond,
			Burst:             cfg.Limiter.Burst,
			NumShards:         cfg.Limiter.NumShards,
			CleanupInterval:   cfg.Limiter.CleanupInterval,
			ClientTTL:         cfg.Limiter.ClientTTL,
			LogTTL:            cfg.Limiter.LogTTL,
			IPWhitelist:       cfg.Limiter.IPWhitelist,
		}, r.LimiterLogger, r.Responder, r.Notifier),
		middleware.Recover(r.HttpLogger, r.Notifier, r.Responder, middleware.RecoverOptions{
			DebugQueryKey:   cfg.TracerDebugKey,
			DebugQueryValue: cfg.TracerDebugValue,
		}),
	)
}
