package responses

import (
	nethttp "net/http"

	"github.com/goravel/framework/contracts/http"
	contractsvalidation "github.com/goravel/framework/contracts/validation"
)

// ErrorDetail is a single field-level validation error.
type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorBody is the standardized error response payload.
type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
	// Extra is used for CSV processing errors to include the processor report.
	Extra map[string]any `json:"extra,omitempty"`
}

// ErrorResponse is the top-level error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// Error codes.
const (
	CodeValidation      = "VALIDATION_ERROR"
	CodeUnauthenticated = "UNAUTHENTICATED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeInternal        = "INTERNAL_ERROR"
)

// JSON sends a standardized error JSON response.
func JSON(ctx http.Context, status int, code, message string, details []ErrorDetail) http.Response {
	return ctx.Response().Status(status).Json(http.Json{
		"error": ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// ValidationErrors converts Goravel validation.Errors into ErrorDetail slice
// and sends a 400 response.
func ValidationErrors(ctx http.Context, errs contractsvalidation.Errors) http.Response {
	details := make([]ErrorDetail, 0)
	for field, msgs := range errs.All() {
		for _, msg := range msgs {
			details = append(details, ErrorDetail{Field: field, Message: msg})
		}
	}
	return JSON(ctx, nethttp.StatusBadRequest, CodeValidation, "Validation failed", details)
}

// Simple sends a simple error response without details.
func Simple(ctx http.Context, status int, code, message string) http.Response {
	return JSON(ctx, status, code, message, nil)
}

// NotFound sends a 404 error.
func NotFound(ctx http.Context, resource string) http.Response {
	return Simple(ctx, nethttp.StatusNotFound, CodeNotFound, resource+" not found")
}

// Forbidden sends a 403 error.
func Forbidden(ctx http.Context) http.Response {
	return Simple(ctx, nethttp.StatusForbidden, CodeForbidden, "You are not authorized to perform this action")
}

// Unauthenticated sends a 401 error.
func Unauthenticated(ctx http.Context) http.Response {
	return Simple(ctx, nethttp.StatusUnauthorized, CodeUnauthenticated, "Authentication required")
}

// Conflict sends a 409 error.
func Conflict(ctx http.Context, message string) http.Response {
	return Simple(ctx, nethttp.StatusConflict, CodeConflict, message)
}

// Internal sends a 500 error.
func Internal(ctx http.Context, message string) http.Response {
	return Simple(ctx, nethttp.StatusInternalServerError, CodeInternal, message)
}
