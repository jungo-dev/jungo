package console

import "context"

// Command is a CLI-invokable unit of work.
type Command interface {
	// Signature is the name used on the command line, e.g. "user:list".
	Signature() string
	// Run executes the command with the CLI arguments following the signature.
	Run(ctx context.Context, args []string) error
}
