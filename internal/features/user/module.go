package user

import (
	"go.uber.org/fx"

	"github.com/jungo-dev/junkit/i18n"

	"jungo/internal/console"
	"jungo/internal/features/user/command"
	"jungo/internal/features/user/domain"
	v1handler "jungo/internal/features/user/handler/v1"
	"jungo/internal/features/user/repository"
	v1route "jungo/internal/features/user/router/v1"
	"jungo/internal/features/user/service"
	"jungo/internal/router"
)

// Module wires the user feature's dependencies into the application's Fx graph.
var Module = fx.Module("user",
	fx.Provide(
		fx.Annotate(
			repository.NewUserRepository,
			fx.As(new(domain.UserRepository)),
		),
	),
	fx.Provide(
		fx.Annotate(
			service.NewUserService,
			fx.As(new(domain.UserService)),
		),
	),
	fx.Provide(v1handler.NewUserHandler),
	fx.Provide(
		fx.Annotate(
			v1route.NewUserRoutes,
			fx.As(new(router.Routes)),
			fx.ResultTags(`group:"routes"`),
		),
	),
	// =============================================================================
	// COMMANDS
	// =============================================================================
	fx.Provide(
		fx.Annotate(
			command.NewListUsersCommand,
			fx.As(new(console.Command)),
			fx.ResultTags(`group:"commands"`),
		),
	),

	fx.Invoke(registerTranslations),
)

// registerTranslations registers domain-specific translation messages.
func registerTranslations(translator *i18n.Translator) {
	translator.AddTranslations(map[string]map[string]string{
		i18n.LangEN: {
			"user_not_found":       "User not found",
			"email_already_exists": "This email is already registered",
			"invalid_avatar_file":  "Invalid avatar file (allowed: jpg, jpeg, png, webp; max 5MB)",
			"user_created":         "User created successfully",
			"user_retrieved":       "User retrieved successfully",
			"users_listed":         "Users listed successfully",
			"user_updated":         "User updated successfully",
			"user_deleted":         "User deleted successfully",
			"avatar_uploaded":      "Avatar uploaded successfully",
			"avatar_deleted":       "Avatar deleted successfully",
		},
	})
}
