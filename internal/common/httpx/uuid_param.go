// Package httpx provides shared HTTP handler helpers used across features.
package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jungo-dev/junkit/response"
	"github.com/jungo-dev/junkit/validation"
)

// uuidParam binds the ":uuid" URI path parameter.
type uuidParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

// ParseUUID binds and validates the ":uuid" path parameter, sending the
// appropriate error response itself when binding or parsing fails.
func ParseUUID(ctx *gin.Context, responder response.Responder, validator *validation.Validator) (uuid.UUID, bool) {
	var params uuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		responder.SendWithData(ctx, http.StatusUnprocessableEntity, "validation_error", validator.GetValidationErrors(ctx, err))
		return uuid.Nil, false
	}

	uid, err := uuid.Parse(params.UUID)
	if err != nil {
		responder.Send(ctx, http.StatusBadRequest, "invalid_uuid")
		return uuid.Nil, false
	}
	return uid, true
}
