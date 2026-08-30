package commands

import (
	"context"
	"fmt"

	"github.com/jungo-dev/junkit/console"
	"github.com/jungo-dev/junkit/database"
)

// HealthCheckCommand pings the database and reports the current row count
// for the users table.
//
// Usage:
//
//	make console CMD="health:check"
type HealthCheckCommand struct {
	db *database.DB
}

// NewHealthCheckCommand creates a HealthCheckCommand.
func NewHealthCheckCommand(db *database.DB) *HealthCheckCommand {
	return &HealthCheckCommand{db: db}
}

// Signature implements console.Command.
func (c *HealthCheckCommand) Signature() string {
	return "health:check"
}

// Run implements console.Command.
func (c *HealthCheckCommand) Run(ctx context.Context, args []string) error {
	if err := c.db.Pool().Ping(ctx); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}

	var count int64
	if err := c.db.Executor(ctx).QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	console.Successf("database reachable, %d user(s)", count)
	return nil
}
