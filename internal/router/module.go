package router

import "go.uber.org/fx"

// Module collects all Routes registered in the "routes" Fx group.
var Module = fx.Annotate(
	collectRoutes,
	fx.ParamTags(`group:"routes"`),
)

// collectRoutes returns the collected routes from Fx.
func collectRoutes(impls []Routes) []Routes {
	return impls
}
