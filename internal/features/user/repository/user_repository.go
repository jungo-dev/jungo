package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/jungo-dev/junkit/database"

	"jungo/internal/database/sqlc"
	"jungo/internal/features/user/domain"
)

// UserRepository implements domain.UserRepository.
type UserRepository struct {
	db *database.DB
}

// NewUserRepository creates a UserRepository backed by db.
func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create implements domain.UserRepository.
func (r *UserRepository) Create(ctx context.Context, input domain.CreateUserInput, passwordHash string) (*domain.User, error) {
	row, err := sqlc.New(r.db.Executor(ctx)).CreateUser(ctx, sqlc.CreateUserParams{
		Email:        input.Email,
		PasswordHash: passwordHash,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		PhoneNumber:  input.PhoneNumber,
		Status:       domain.UserStatusActive,
	})
	if err != nil {
		return nil, database.Match(err, map[database.ErrorType]error{
			database.ErrorUniqueViolation: domain.ErrEmailAlreadyExists,
		})
	}
	return mapUser(row), nil
}

// GetByUUID implements domain.UserRepository.
func (r *UserRepository) GetByUUID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row, err := sqlc.New(r.db.Executor(ctx)).GetUserByUUID(ctx, id)
	if err != nil {
		return nil, database.Match(err, map[database.ErrorType]error{
			database.ErrorNotFound: domain.ErrUserNotFound,
		})
	}
	return mapUser(row), nil
}

// GetByEmail implements domain.UserRepository.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := sqlc.New(r.db.Executor(ctx)).GetUserByEmail(ctx, email)
	if err != nil {
		return nil, database.Match(err, map[database.ErrorType]error{
			database.ErrorNotFound: domain.ErrUserNotFound,
		})
	}
	return mapUser(row), nil
}

// List implements domain.UserRepository, returning a page of users plus the
// total count matching filter.Search for pagination metadata.
func (r *UserRepository) List(ctx context.Context, filter domain.UserListFilter) ([]*domain.User, int64, error) {
	queries := sqlc.New(r.db.Executor(ctx))

	rows, err := queries.ListUsers(ctx, sqlc.ListUsersParams{
		Limit:     filter.Limit,
		Offset:    filter.Offset,
		Search:    filter.Search,
		SortOrder: filter.SortOrder,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := queries.CountUsers(ctx, filter.Search)
	if err != nil {
		return nil, 0, err
	}

	users := make([]*domain.User, len(rows))
	for i, row := range rows {
		users[i] = mapUser(row)
	}
	return users, total, nil
}

// Update implements domain.UserRepository.
func (r *UserRepository) Update(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
	row, err := sqlc.New(r.db.Executor(ctx)).UpdateUser(ctx, sqlc.UpdateUserParams{
		Uuid:        id,
		FirstName:   input.FirstName,
		LastName:    input.LastName,
		PhoneNumber: input.PhoneNumber,
		AvatarUrl:   input.AvatarUrl,
		Status:      input.Status,
	})
	if err != nil {
		return nil, database.Match(err, map[database.ErrorType]error{
			database.ErrorNotFound: domain.ErrUserNotFound,
		})
	}
	return mapUser(row), nil
}

// Delete implements domain.UserRepository.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rowsAffected, err := sqlc.New(r.db.Executor(ctx)).DeleteUser(ctx, id)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// mapUser converts a generated sqlc.User row to the domain model.
func mapUser(row sqlc.User) *domain.User {
	return &domain.User{
		Uuid:        row.Uuid,
		Email:       row.Email,
		FirstName:   row.FirstName,
		LastName:    row.LastName,
		PhoneNumber: row.PhoneNumber,
		AvatarUrl:   row.AvatarUrl,
		Status:      row.Status,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
