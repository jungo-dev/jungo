package command

import (
	"context"
	"fmt"

	"github.com/jungo-dev/junkit/console"

	"jungo/internal/features/user/domain"
)

// ListUsersCommand prints a page of users through domain.UserService — the
// same business logic (and DB) the "GET /api/v1/users" handler uses.
//
// Usage:
//
//	make console CMD="user:list"
type ListUsersCommand struct {
	userService domain.UserService
}

// NewListUsersCommand creates a ListUsersCommand.
func NewListUsersCommand(userService domain.UserService) *ListUsersCommand {
	return &ListUsersCommand{userService: userService}
}

// Signature implements console.Command.
func (c *ListUsersCommand) Signature() string {
	return "user:list"
}

// Run implements console.Command.
func (c *ListUsersCommand) Run(ctx context.Context, args []string) error {
	users, total, err := c.userService.GetUsers(ctx, domain.UserListFilter{Limit: 20})
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	console.Infof("showing %d of %d user(s)", len(users), total)
	for _, u := range users {
		fmt.Printf("  %s  %s %s  <%s>\n", u.Uuid, u.FirstName, u.LastName, u.Email)
	}
	return nil
}
