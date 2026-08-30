package v1

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jungo-dev/junkit/pagination"
	"github.com/jungo-dev/junkit/response"
	"github.com/jungo-dev/junkit/validation"

	"jungo/internal/features/user/domain"
	v1dto "jungo/internal/features/user/dto/v1"
)

// maxAvatarFileSize caps an uploaded avatar at 5MB.
const maxAvatarFileSize = 5 << 20

// allowedAvatarExtensions are the file extensions UploadAvatar accepts.
var allowedAvatarExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

// UserHandler adapts HTTP requests to domain.UserService calls.
type UserHandler struct {
	service   domain.UserService
	responder response.Responder
	validator *validation.Validator
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(service domain.UserService, responder response.Responder, validator *validation.Validator) *UserHandler {
	return &UserHandler{service: service, responder: responder, validator: validator}
}

// CreateUser handles POST /users.
func (h *UserHandler) CreateUser(ctx *gin.Context) {
	var req v1dto.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.responder.SendWithData(ctx, http.StatusUnprocessableEntity, "validation_error", h.validator.GetValidationErrors(ctx, err))
		return
	}

	user, err := h.service.CreateUser(ctx.Request.Context(), v1dto.ToCreateUserInput(req))
	if err != nil {
		h.responder.Error(ctx, err)
		return
	}

	h.responder.SendWithData(ctx, http.StatusCreated, "user_created", v1dto.NewUserResponse(user))
}

// GetUser handles GET /users/:uuid.
func (h *UserHandler) GetUser(ctx *gin.Context) {
	id, ok := h.parseUserUUID(ctx)
	if !ok {
		return
	}

	user, err := h.service.GetUser(ctx.Request.Context(), id)
	if err != nil {
		h.responder.Error(ctx, err)
		return
	}

	h.responder.SendWithData(ctx, http.StatusOK, "user_retrieved", v1dto.NewUserResponse(user))
}

// ListUsers handles GET /users.
func (h *UserHandler) ListUsers(ctx *gin.Context) {
	var req v1dto.ListUsersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		h.responder.SendWithData(ctx, http.StatusUnprocessableEntity, "validation_error", h.validator.GetValidationErrors(ctx, err))
		return
	}
	req.SetDefaults("created_at", "desc")

	users, total, err := h.service.GetUsers(ctx.Request.Context(), req.ToFilter())
	if err != nil {
		h.responder.Error(ctx, err)
		return
	}

	meta := pagination.NewPagination(req.Page, req.PageSize, total)
	h.responder.Pagination(ctx, http.StatusOK, "users_listed", v1dto.NewUserResponseList(users), meta)
}

// UpdateUser handles PATCH /users/:uuid.
func (h *UserHandler) UpdateUser(ctx *gin.Context) {
	id, ok := h.parseUserUUID(ctx)
	if !ok {
		return
	}

	var req v1dto.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.responder.SendWithData(ctx, http.StatusUnprocessableEntity, "validation_error", h.validator.GetValidationErrors(ctx, err))
		return
	}

	user, err := h.service.UpdateUser(ctx.Request.Context(), id, v1dto.ToUpdateUserInput(req))
	if err != nil {
		h.responder.Error(ctx, err)
		return
	}

	h.responder.SendWithData(ctx, http.StatusOK, "user_updated", v1dto.NewUserResponse(user))
}

// DeleteUser handles DELETE /users/:uuid.
func (h *UserHandler) DeleteUser(ctx *gin.Context) {
	id, ok := h.parseUserUUID(ctx)
	if !ok {
		return
	}

	if err := h.service.DeleteUser(ctx.Request.Context(), id); err != nil {
		h.responder.Error(ctx, err)
		return
	}

	h.responder.Send(ctx, http.StatusOK, "user_deleted")
}

// UploadAvatar handles POST /users/:uuid/avatar.
func (h *UserHandler) UploadAvatar(ctx *gin.Context) {
	id, ok := h.parseUserUUID(ctx)
	if !ok {
		return
	}

	fileHeader, err := ctx.FormFile("avatar")
	if err != nil {
		h.responder.Send(ctx, http.StatusBadRequest, "invalid_avatar_file")
		return
	}
	if fileHeader.Size > maxAvatarFileSize {
		h.responder.Send(ctx, http.StatusBadRequest, "invalid_avatar_file")
		return
	}
	if ext := extensionOf(fileHeader.Filename); !allowedAvatarExtensions[ext] {
		h.responder.Send(ctx, http.StatusBadRequest, "invalid_avatar_file")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		h.responder.Send(ctx, http.StatusBadRequest, "invalid_avatar_file")
		return
	}
	defer file.Close()

	user, err := h.service.UploadAvatar(ctx.Request.Context(), id, domain.AvatarFile{
		Reader:   file,
		Filename: fileHeader.Filename,
	})
	if err != nil {
		h.responder.Error(ctx, err)
		return
	}

	h.responder.SendWithData(ctx, http.StatusOK, "avatar_uploaded", v1dto.NewUserResponse(user))
}

// DeleteAvatar handles DELETE /users/:uuid/avatar.
func (h *UserHandler) DeleteAvatar(ctx *gin.Context) {
	id, ok := h.parseUserUUID(ctx)
	if !ok {
		return
	}

	user, err := h.service.DeleteAvatar(ctx.Request.Context(), id)
	if err != nil {
		h.responder.Error(ctx, err)
		return
	}

	h.responder.SendWithData(ctx, http.StatusOK, "avatar_deleted", v1dto.NewUserResponse(user))
}

// parseUserUUID binds and validates the ":uuid" path parameter.
func (h *UserHandler) parseUserUUID(ctx *gin.Context) (uuid.UUID, bool) {
	var params v1dto.UserUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		h.responder.SendWithData(ctx, http.StatusUnprocessableEntity, "validation_error", h.validator.GetValidationErrors(ctx, err))
		return uuid.Nil, false
	}

	id, err := uuid.Parse(params.Uuid)
	if err != nil {
		h.responder.Send(ctx, http.StatusBadRequest, "invalid_uuid")
		return uuid.Nil, false
	}
	return id, true
}

// extensionOf returns filename's lowercase extension, including the leading dot.
func extensionOf(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}
