package adapter

import (
	"context"
	"errors"

	authdomain "jungo/internal/features/auth/domain"
	userdomain "jungo/internal/features/user/domain"
)

// UserAdapter implements authdomain.UserProvider over the user feature.
type UserAdapter struct {
	userService userdomain.UserService
}

// NewUserAdapter creates a UserAdapter.
func NewUserAdapter(userService userdomain.UserService) *UserAdapter {
	return &UserAdapter{userService: userService}
}

// GetCredentialsByEmail implements authdomain.UserProvider.
func (a *UserAdapter) GetCredentialsByEmail(ctx context.Context, email string) (*authdomain.Credentials, error) {
	creds, err := a.userService.GetCredentialsByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			return nil, authdomain.ErrUserNotFound
		}
		return nil, err
	}

	return &authdomain.Credentials{
		UserID:       creds.ID,
		UserUuid:     creds.Uuid,
		PasswordHash: creds.PasswordHash,
		Active:       creds.Status == userdomain.UserStatusActive,
	}, nil
}
