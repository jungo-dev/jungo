package domain

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// User status constants.
const (
	UserStatusActive   int16 = 1
	UserStatusInactive int16 = 2
)

// User represents the user entity.
type User struct {
	Uuid        uuid.UUID
	Email       string
	FirstName   string
	LastName    string
	PhoneNumber *string
	AvatarUrl   *string
	Status      int16
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

// IsActive checks if the user is active.
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

// CreateUserInput holds data for creating a user.
type CreateUserInput struct {
	FirstName   string
	LastName    string
	Email       string
	Password    string
	PhoneNumber *string
}

// UpdateUserInput holds data for updating a user.
type UpdateUserInput struct {
	FirstName   *string
	LastName    *string
	PhoneNumber *string
	AvatarUrl   *string
	Status      *int16
}

// UserListFilter holds parameters for filtering and paginating users.
type UserListFilter struct {
	Search    string
	Limit     int32
	Offset    int32
	SortOrder string
}

// UserRepository defines database operations for users.
type UserRepository interface {
	Create(ctx context.Context, input CreateUserInput, passwordHash string) (*User, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, filter UserListFilter) ([]*User, int64, error)
	Update(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserService defines business logic for users.
type UserService interface {
	CreateUser(ctx context.Context, input CreateUserInput) (*User, error)
	GetUser(ctx context.Context, id uuid.UUID) (*User, error)
	GetUsers(ctx context.Context, filter UserListFilter) ([]*User, int64, error)
	UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	UploadAvatar(ctx context.Context, id uuid.UUID, file AvatarFile) (*User, error)
	DeleteAvatar(ctx context.Context, id uuid.UUID) (*User, error)
}

// AvatarFile represents an uploaded avatar file.
type AvatarFile struct {
	Reader   io.Reader
	Filename string
}
