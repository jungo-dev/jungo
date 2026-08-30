package console

import "go.uber.org/fx"

// Module collects all Command implementations registered in the "commands" Fx group.
var Module = fx.Annotate(
	collectCommands,
	fx.ParamTags(`group:"commands"`),
)

// collectCommands returns the collected commands from Fx.
func collectCommands(impls []Command) []Command {
	return impls
}
