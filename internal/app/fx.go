package app

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/jungo-dev/junkit/cache"
	"github.com/jungo-dev/junkit/database"
	"github.com/jungo-dev/junkit/httpclient"
	"github.com/jungo-dev/junkit/i18n"
	"github.com/jungo-dev/junkit/logger"
	"github.com/jungo-dev/junkit/notification"
	"github.com/jungo-dev/junkit/recaptcha"
	"github.com/jungo-dev/junkit/response"
	"github.com/jungo-dev/junkit/storage"
	"github.com/jungo-dev/junkit/telegram"
	"github.com/jungo-dev/junkit/tracer"
	"github.com/jungo-dev/junkit/validation"

	"jungo/internal/config"
	"jungo/internal/console"
	"jungo/internal/console/commands"
	"jungo/internal/features/user"
	"jungo/internal/router"
)

// CoreModules returns Fx options shared between the API server and console.
func CoreModules(cfg *config.Config) []fx.Option {
	return []fx.Option{
		// Route Fx event logs through Zap at Debug level.
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			zlog := &fxevent.ZapLogger{Logger: log}
			zlog.UseLogLevel(zapcore.DebugLevel)
			return zlog
		}),

		// =====================================================================
		// CONFIG
		// =====================================================================
		fx.Supply(cfg),

		// =====================================================================
		// CORE PACKAGES (github.com/jungo-dev/junkit)
		// =====================================================================
		fx.Options(provideLoggers(cfg)...),

		fx.Module("database",
			fx.Decorate(fx.Annotate(
				func(sqlLogger *zap.Logger) *zap.Logger { return sqlLogger },
				fx.ParamTags(`name:"sql"`),
			)),
			fx.Provide(provideDatabaseOptions),
			database.Module,
		),

		fx.Provide(provideResponseOptions),
		i18n.Module,
		response.Module,

		fx.Provide(provideValidationOptions),
		validation.Module,

		// =====================================================================
		// OPTIONAL MODULES & INTEGRATIONS
		// =====================================================================
		fx.Provide(provideCacheOptions),
		cache.Module,

		fx.Provide(provideStorageOptions),
		storage.Module,

		fx.Provide(httpclient.NewDefaultClient),

		fx.Provide(provideTelegramOptions),
		telegram.Module,

		fx.Provide(provideNotificationOptions),
		notification.Module,

		fx.Provide(provideRecaptchaOptions),
		recaptcha.Module,

		// =====================================================================
		// FEATURES
		// =====================================================================
		user.Module,

		// =====================================================================
		// GLOBAL CONSOLE COMMANDS
		// =====================================================================
		commands.Module,
	}
}

// HTTPModules returns Fx options for the HTTP server lifecycle and routing.
func HTTPModules() []fx.Option {
	return []fx.Option{
		fx.Provide(provideGin),

		fx.Provide(router.Module),
		fx.Invoke(router.RegisterAll),

		fx.Provide(NewApp),
		fx.Invoke((*App).Run),
	}
}

// GetFxOptions returns all Fx options configured for the API server.
func GetFxOptions(cfg *config.Config) []fx.Option {
	return append(CoreModules(cfg), HTTPModules()...)
}

// NewAPIFx initializes and assembles the full API server Fx application.
func NewAPIFx() *fx.App {
	cfg := config.NewConfig()
	return fx.New(GetFxOptions(cfg)...)
}

// NewConsoleFx assembles the Fx application for running a single CLI command.
func NewConsoleFx() *fx.App {
	cfg := config.NewConfig()
	return fx.New(append(CoreModules(cfg),
		fx.Provide(console.Module),
		fx.Invoke(console.RunSelected),
	)...)
}

// provideGin initializes the Gin engine with trusted proxies.
func provideGin(cfg *config.Config) *gin.Engine {
	r := gin.New()
	if len(cfg.Security.TrustedProxies) > 0 {
		_ = r.SetTrustedProxies(cfg.Security.TrustedProxies)
	} else {
		_ = r.SetTrustedProxies(nil)
	}
	return r
}

// provideLoggers builds the app/http/sql/limiter named logging channels.
func provideLoggers(cfg *config.Config) []fx.Option {
	shared := logger.Options{
		Environment: cfg.Environment,
		MaxSize:     cfg.Logger.MaxSize,
		MaxBackups:  cfg.Logger.MaxBackups,
		MaxAge:      cfg.Logger.MaxAge,
		Compress:    cfg.Logger.Compress,
	}

	defs := []logger.Definition{
		{Name: "app", Level: cfg.Logger.AppLevel, Filename: cfg.Logger.AppFilename},
		{Name: "http", Level: cfg.Logger.HTTPLevel, Filename: cfg.Logger.HTTPFilename},
		{Name: "sql", Level: cfg.Logger.SQLLevel, Filename: cfg.Logger.SQLFilename},
		{Name: "limiter", Level: cfg.Logger.LimiterLevel, Filename: cfg.Logger.LimiterFilename},
	}

	return logger.ProvideNamed(defs, shared)
}

// provideDatabaseOptions configures database connections and query tracing.
func provideDatabaseOptions(cfg *config.Config) database.Options {
	return database.Options{
		DSN:                cfg.DSN(),
		MaxConnection:      cfg.DB.MaxConnection,
		MinConnection:      cfg.DB.MinConnection,
		MaxConnLifetime:    cfg.DB.MaxConnLifetime,
		MaxConnIdleTime:    cfg.DB.MaxConnIdleTime,
		HealthCheckPeriod:  cfg.DB.HealthCheckPeriod,
		QueryLogging:       cfg.DB.QueryLogging,
		SlowQueryThreshold: cfg.DB.SlowQueryThreshold,
		Decorate: func(inner database.DBTX) database.DBTX {
			return tracer.NewDBWrapper(inner)
		},
	}
}

func provideResponseOptions(cfg *config.Config) response.Options {
	return response.Options{Language: cfg.Language}
}

func provideValidationOptions(cfg *config.Config) validation.Options {
	return validation.Options{Language: cfg.Language}
}

func provideCacheOptions(cfg *config.Config) cache.Options {
	if !cfg.Redis.Enabled {
		return cache.Options{Driver: cache.DriverNoop}
	}
	return cache.Options{
		Driver: cache.DriverRedis,
		Redis: cache.RedisOptions{
			Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
	}
}

func provideStorageOptions(cfg *config.Config) storage.Options {
	return storage.Options{BaseDir: cfg.Storage.BaseDir, BaseURL: cfg.Storage.BaseURL}
}

func provideTelegramOptions(cfg *config.Config) telegram.Options {
	return telegram.Options{Token: cfg.Telegram.Token}
}

func provideNotificationOptions(cfg *config.Config) notification.Options {
	return notification.Options{TelegramChatID: cfg.Telegram.ChatID, Environment: cfg.Environment}
}

func provideRecaptchaOptions(cfg *config.Config) recaptcha.Options {
	return recaptcha.Options{
		SecretKey: cfg.Recaptcha.SecretKey,
		MinScore:  cfg.Recaptcha.MinScore,
		Mock:      cfg.Recaptcha.Mock,
	}
}
