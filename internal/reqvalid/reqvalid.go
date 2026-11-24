// Package reqvalid binds Fiber query/body structs and validates them with
// go-playground/validator tags (Java Bean Validation–style annotations).
package reqvalid

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/httperr"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/models"
)

var validate = newValidator()

// Normalizer trims / normalizes fields before validation.
type Normalizer interface {
	Normalize()
}

func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		for _, key := range []string{"query", "json", "form"} {
			name := fld.Tag.Get(key)
			if name == "" || name == "-" {
				continue
			}
			if i := strings.IndexByte(name, ','); i >= 0 {
				name = name[:i]
			}
			if name != "" {
				return name
			}
		}
		return fld.Name
	})

	_ = v.RegisterValidation("entityname", validateEntityName)
	_ = v.RegisterValidation("hhmm", validateHHMM)
	_ = v.RegisterValidation("rfc3339", validateRFC3339)
	_ = v.RegisterValidation("mediakind", validateMediaKind)
	_ = v.RegisterValidation("maxrunes", validateMaxRunes)
	_ = v.RegisterValidation("chatid", validateChatID)
	return v
}

func validateEntityName(fl validator.FieldLevel) bool {
	s, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	return models.ValidName(s)
}

func validateHHMM(fl validator.FieldLevel) bool {
	s := strings.TrimSpace(fl.Field().String())
	if s == "" {
		return true
	}
	_, err := time.Parse("15:04", s)
	return err == nil
}

func validateRFC3339(fl validator.FieldLevel) bool {
	s := strings.TrimSpace(fl.Field().String())
	if s == "" {
		return true
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func validateMediaKind(fl validator.FieldLevel) bool {
	return models.ValidMediaKind(strings.TrimSpace(fl.Field().String()))
}

func validateChatID(fl validator.FieldLevel) bool {
	return models.ValidTelegramChatID(fl.Field().String())
}

func validateMaxRunes(fl validator.FieldLevel) bool {
	limit, err := parseIntParam(fl.Param())
	if err != nil {
		return false
	}
	return utf8.RuneCountInString(fl.Field().String()) <= limit
}

func parseIntParam(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// Struct runs validate tags on dest (after optional Normalize).
func Struct(dest any) error {
	if n, ok := dest.(Normalizer); ok {
		n.Normalize()
	}
	return validate.Struct(dest)
}

// Query parses c query string into dest and validates tags.
func Query(c fiber.Ctx, dest any) error {
	if err := c.Bind().Query(dest); err != nil {
		return httperr.Write(c, fiber.StatusBadRequest, "bad_request", "Invalid query parameters.")
	}
	if err := Struct(dest); err != nil {
		return httperr.Write(c, fiber.StatusBadRequest, "bad_request", Message(err))
	}
	return nil
}

// Body parses JSON body into dest and validates tags.
func Body(c fiber.Ctx, dest any) error {
	if err := c.Bind().Body(dest); err != nil {
		return httperr.Write(c, fiber.StatusBadRequest, "bad_request", "Expected JSON body.")
	}
	if err := Struct(dest); err != nil {
		return httperr.Write(c, fiber.StatusBadRequest, "bad_request", Message(err))
	}
	return nil
}

// Message turns the first validation error into a short client message.
func Message(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) || len(ve) == 0 {
		return "Validation failed."
	}
	fe := ve[0]
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required.", field)
	case "min":
		if fe.Kind() == reflect.String || fe.Type().String() == "string" {
			return fmt.Sprintf("%s is too short.", field)
		}
		if fe.Kind() == reflect.Slice || fe.Kind() == reflect.Array || fe.Kind() == reflect.Map {
			return fmt.Sprintf("%s must have at least %s item(s).", field, fe.Param())
		}
		return fmt.Sprintf("%s must be >= %s.", field, fe.Param())
	case "max":
		if fe.Kind() == reflect.String || fe.Kind() == reflect.Invalid && fe.Type().Kind() == reflect.String {
			return fmt.Sprintf("%s is too long.", field)
		}
		if fe.Kind() == reflect.Slice || fe.Kind() == reflect.Array || fe.Kind() == reflect.Map {
			return fmt.Sprintf("%s must have at most %s item(s).", field, fe.Param())
		}
		return fmt.Sprintf("%s must be <= %s.", field, fe.Param())
	case "maxrunes":
		return fmt.Sprintf("%s must be at most %s characters.", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s.", field, strings.ReplaceAll(fe.Param(), " ", ", "))
	case "gte":
		return fmt.Sprintf("%s must be >= %s.", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be <= %s.", field, fe.Param())
	case "unique":
		return fmt.Sprintf("%s must not contain duplicates.", field)
	case "entityname":
		return fmt.Sprintf("%s must be 1–%d characters.", field, models.MaxEntityName)
	case "hhmm":
		return fmt.Sprintf("%s must be HH:mm.", field)
	case "rfc3339":
		return fmt.Sprintf("%s must be an ISO-8601 datetime.", field)
	case "mediakind":
		return fmt.Sprintf(`%s must be "image", "audio", or "video".`, field)
	case "chatid":
		return fmt.Sprintf("%s must be a numeric Telegram chat id.", field)
	default:
		return fmt.Sprintf("%s is invalid.", field)
	}
}
