package v1

import (
	"jungo/internal/features/user/domain"
	"time"

	"github.com/google/uuid"
	"github.com/jungo-dev/junkit/pagination"
)

// UserUuidParam binds the ":uuid" URI path parameter.
type UserUuidParam struct {
	Uuid string `uri:"uuid" binding:"required,uuid"`
}

// CreateUserRequest is the request body for POST /users.
type CreateUserRequest struct {
	FirstName   string  `json:"first_name" binding:"required,min=2,max=50"`
	LastName    string  `json:"last_name" binding:"required,min=2,max=50"`
	Email       string  `json:"email" binding:"required,email,email_advanced"`
	Password    string  `json:"password" binding:"required,min=8,password_strong"`
	PhoneNumber *string `json:"phone_number" binding:"omitempty,phone_advanced"`
}

// ToCreateUserInput converts CreateUserRequest to domain input.
func ToCreateUserInput(req CreateUserRequest) domain.CreateUserInput {
	return domain.CreateUserInput{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Email:       req.Email,
		Password:    req.Password,
		PhoneNumber: req.PhoneNumber,
	}
}

// UpdateUserRequest is the request body for PATCH /users/:uuid.
type UpdateUserRequest struct {
	FirstName   *string `json:"first_name" binding:"omitempty,min=2,max=50"`
	LastName    *string `json:"last_name" binding:"omitempty,min=2,max=50"`
	PhoneNumber *string `json:"phone_number" binding:"omitempty,phone_advanced"`
	Status      *int16  `json:"status" binding:"omitempty,oneof=1 2"`
}

// ToUpdateUserInput converts UpdateUserRequest to domain input.
func ToUpdateUserInput(req UpdateUserRequest) domain.UpdateUserInput {
	return domain.UpdateUserInput{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		PhoneNumber: req.PhoneNumber,
		Status:      req.Status,
	}
}

// ListUsersRequest binds query parameters for listing users.
type ListUsersRequest struct {
	pagination.Request
}

// ToFilter converts ListUsersRequest to domain filter.
func (r *ListUsersRequest) ToFilter() domain.UserListFilter {
	return domain.UserListFilter{
		Search:    r.Search,
		Limit:     r.GetLimit(),
		Offset:    r.GetOffset(),
		SortOrder: r.SortOrder,
	}
}

// UserResponse represents the JSON response for a user.
type UserResponse struct {
	Uuid        uuid.UUID  `json:"uuid"`
	Email       string     `json:"email"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	PhoneNumber *string    `json:"phone_number,omitempty"`
	AvatarUrl   *string    `json:"avatar_url,omitempty"`
	Status      int16      `json:"status"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// NewUserResponse converts domain.User to UserResponse.
func NewUserResponse(u *domain.User) UserResponse {
	avatarURL := u.AvatarUrl
	if avatarURL != nil && *avatarURL == "" {
		avatarURL = nil
	}

	return UserResponse{
		Uuid:        u.Uuid,
		Email:       u.Email,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		PhoneNumber: u.PhoneNumber,
		AvatarUrl:   avatarURL,
		Status:      u.Status,
		Active:      u.IsActive(),
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

// NewUserResponseList converts a slice of domain.User to UserResponse list.
func NewUserResponseList(users []*domain.User) []UserResponse {
	responses := make([]UserResponse, len(users))
	for i, u := range users {
		responses[i] = NewUserResponse(u)
	}
	return responses
}
