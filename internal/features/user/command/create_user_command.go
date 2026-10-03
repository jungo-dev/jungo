package command

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jungo-dev/junkit/console"

	"jungo/internal/features/user/domain"
)

// minPasswordLength mirrors the HTTP API's password minimum.
const minPasswordLength = 8

// CreateUserCommand creates a user (e.g. the first account); the password is read from
// stdin when -password is omitted.
//
// Usage:
//
//	make console CMD="user:create" ARGS="-email=admin@example.com -first-name=Admin -last-name=User"
type CreateUserCommand struct {
	userService domain.UserService
}

// NewCreateUserCommand creates a CreateUserCommand.
func NewCreateUserCommand(userService domain.UserService) *CreateUserCommand {
	return &CreateUserCommand{userService: userService}
}

// Signature implements console.Command.
func (c *CreateUserCommand) Signature() string {
	return "user:create"
}

// Run implements console.Command.
func (c *CreateUserCommand) Run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet(c.Signature(), flag.ContinueOnError)
	email := flags.String("email", "", "email address (required)")
	firstName := flags.String("first-name", "", "first name (required)")
	lastName := flags.String("last-name", "", "last name (required)")
	password := flags.String("password", "", "password (read from stdin when omitted)")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *email == "" || *firstName == "" || *lastName == "" {
		return fmt.Errorf("-email, -first-name and -last-name are required")
	}

	if *password == "" {
		fmt.Print("Password: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return fmt.Errorf("read password: %w", err)
		}
		*password = strings.TrimRight(line, "\r\n")
	}
	if len(*password) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}

	user, err := c.userService.CreateUser(ctx, domain.CreateUserInput{
		Email:     *email,
		FirstName: *firstName,
		LastName:  *lastName,
		Password:  *password,
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	console.Successf("Created user %s <%s>", user.Uuid, user.Email)
	return nil
}
