package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"jungo/internal/config"
)

// App wires together the running HTTP server's dependencies.
type App struct {
	Config *config.Config
	Router *gin.Engine
	Logger *zap.Logger
}

// NewApp assembles an App from its Fx-injected dependencies.
func NewApp(cfg *config.Config, logger *zap.Logger, router *gin.Engine) *App {
	return &App{Config: cfg, Logger: logger, Router: router}
}

// Run starts the HTTP server on an Fx OnStart hook and shuts it down
// gracefully on OnStop, with timeouts tuned against Slowloris-style attacks.
func (app *App) Run(lc fx.Lifecycle) {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", app.Config.ServerPort),
		Handler:           app.Router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			app.Logger.Info("server starting",
				zap.String("port", app.Config.ServerPort),
				zap.String("env", app.Config.Environment),
			)

			go func() {
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					app.Logger.Error("server failed to start", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			app.Logger.Info("shutting down server")

			if err := srv.Shutdown(ctx); err != nil {
				app.Logger.Error("graceful shutdown failed", zap.Error(err))
				return err
			}

			app.Logger.Info("server stopped gracefully")
			return nil
		},
	})
}
