package httpx

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Furkanturan8/rovena/shared/errorx"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

var validate = func() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return f.Name
		}
		return name
	})
	return v
}()

// BindAndValidate parses the JSON body into req and validates its `validate` tags.
func BindAndValidate(c *fiber.Ctx, req any) error {
	if err := c.BodyParser(req); err != nil {
		return errorx.WithMessage(errorx.ErrInvalidRequest, "Malformed request body")
	}
	return Validate(req)
}

func Validate(req any) error {
	err := validate.Struct(req)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return errorx.WithCause(errorx.ErrInvalidRequest, err)
	}
	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		fields[fieldPath(fe)] = message(fe)
	}
	return errorx.WithFields(errorx.ErrValidation, fields)
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	case "uuid":
		return "must be a valid UUID"
	default:
		return "is invalid"
	}
}

// fieldPath drops the leading struct name: "CreateProductRequest.images[0].url" -> "images[0].url".
func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if _, rest, ok := strings.Cut(ns, "."); ok {
		return rest
	}
	return ns
}
