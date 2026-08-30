package service

import (
	"context"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jungo-dev/junkit/storage"
	"github.com/jungo-dev/junkit/tracer"

	"jungo/internal/features/user/domain"
)

// UserService implements domain.UserService.
type UserService struct {
	repo    domain.UserRepository
	storage storage.Service
}

// NewUserService creates a UserService.
func NewUserService(repo domain.UserRepository, storageService storage.Service) *UserService {
	return &UserService{repo: repo, storage: storageService}
}

// CreateUser creates a new user with hashed password.
//
// Flow: hash password -> persist user -> return
func (s *UserService) CreateUser(ctx context.Context, input domain.CreateUserInput) (*domain.User, error) {
	stop := tracer.Span(ctx, "Bcrypt Hashing")
	passwordHash, err := hashPassword(input.Password)
	stop()

	if err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, input, passwordHash)
}

// GetUser returns a user by UUID.
func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.repo.GetByUUID(ctx, id)
}

// GetUsers returns a paginated list of users.
func (s *UserService) GetUsers(ctx context.Context, filter domain.UserListFilter) ([]*domain.User, int64, error) {
	return s.repo.List(ctx, filter)
}

// UpdateUser updates user information.
func (s *UserService) UpdateUser(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
	return s.repo.Update(ctx, id, input)
}

// DeleteUser deletes a user by UUID.
func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// UploadAvatar uploads a new avatar and updates the user record.
//
// Flow: get user -> upload file -> update avatar URL -> delete old file.
func (s *UserService) UploadAvatar(ctx context.Context, id uuid.UUID, file domain.AvatarFile) (*domain.User, error) {
	user, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}

	stop := tracer.Span(ctx, "Upload Avatar to Storage", tracer.CategoryExternal)
	newURL, err := s.storage.UploadFile(ctx, file.Reader, file.Filename)
	stop()
	if err != nil {
		return nil, domain.ErrInvalidAvatarFile
	}

	updated, err := s.repo.Update(ctx, id, domain.UpdateUserInput{AvatarUrl: &newURL})
	if err != nil {
		_ = s.storage.DeleteFile(ctx, newURL)
		return nil, err
	}

	if hasAvatar(user) {
		_ = s.storage.DeleteFile(ctx, *user.AvatarUrl)
	}
	return updated, nil
}

// DeleteAvatar removes the avatar file and clears the avatar URL.
//
// Flow: get user -> delete file -> clear avatar URL in DB
func (s *UserService) DeleteAvatar(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !hasAvatar(user) {
		return user, nil
	}

	_ = s.storage.DeleteFile(ctx, *user.AvatarUrl)

	empty := ""
	return s.repo.Update(ctx, id, domain.UpdateUserInput{AvatarUrl: &empty})
}

// hasAvatar checks if the user has an avatar URL.
func hasAvatar(user *domain.User) bool {
	return user.AvatarUrl != nil && *user.AvatarUrl != ""
}

// hashPassword hashes a password using bcrypt.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
