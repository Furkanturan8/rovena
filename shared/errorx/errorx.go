package errorx

import (
	"fmt"
	"net/http"
)

var (
	ErrValidation         = New(http.StatusUnprocessableEntity, "Validation failed")
	ErrUnauthorized       = New(http.StatusUnauthorized, "Unauthorized")
	ErrForbidden          = New(http.StatusForbidden, "Forbidden")
	ErrNotFound           = New(http.StatusNotFound, "Resource not found")
	ErrConflict           = New(http.StatusConflict, "Resource already exists")
	ErrInvalidRequest     = New(http.StatusBadRequest, "Invalid request")
	ErrInvalidCredentials = New(http.StatusUnauthorized, "Invalid credentials")
	ErrInternal           = New(http.StatusInternalServerError, "Internal server error")
)

// AppError is the only error shape that is rendered to API clients.
type AppError struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	Err     error             `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// WithMessage keeps the sentinel's status code and overrides the client message.
func WithMessage(base *AppError, message string) *AppError {
	return &AppError{Code: base.Code, Message: message}
}

// WithCause keeps the sentinel's message and attaches the underlying error for logging.
func WithCause(base *AppError, err error) *AppError {
	return &AppError{Code: base.Code, Message: base.Message, Err: err}
}

func WithFields(base *AppError, fields map[string]string) *AppError {
	return &AppError{Code: base.Code, Message: base.Message, Fields: fields}
}
