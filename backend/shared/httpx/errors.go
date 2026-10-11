package httpx

import (
	"errors"
	"log/slog"

	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/gofiber/fiber/v2"
)

// ErrorMapper lets a service translate its own domain errors into AppErrors.
type ErrorMapper func(err error) (*errorx.AppError, bool)

type errorBody struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func NewErrorHandler(log *slog.Logger, mappers ...ErrorMapper) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		var appErr *errorx.AppError
		if !errors.As(err, &appErr) {
			for _, m := range mappers {
				if mapped, ok := m(err); ok {
					appErr = mapped
					break
				}
			}
		}

		if appErr == nil {
			var fe *fiber.Error
			if errors.As(err, &fe) {
				appErr = errorx.New(fe.Code, fe.Message)
			} else {
				appErr = errx(err)
			}
		}

		if appErr.Code >= fiber.StatusInternalServerError {
			log.ErrorContext(c.UserContext(), "request failed",
				"error", err, "method", c.Method(), "path", c.Path(), "request_id", RequestID(c))
		}

		return c.Status(appErr.Code).JSON(errorBody{Message: appErr.Message, Fields: appErr.Fields})
	}
}

func errx(err error) *errorx.AppError {
	return errorx.WithCause(errorx.ErrInternal, err)
}
