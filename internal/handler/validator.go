package handler

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/tyha2404/nexo-app-api/internal/constant"
)

// Validator provides validation functionality for request payloads
type Validator struct {
	validate *validator.Validate
}

// newStructValidator is the one validator configuration every handler uses.
// WithRequiredStructEnabled makes `required` reject zero-valued struct fields
// (CustomTime, time.Time): without it a missing transactionDate passed
// validation and was stored as 0001-01-01.
func newStructValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	_ = v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", fl.Field().String())
		return matched
	})
	return v
}

// requestValidator is shared by decodeAndValidate. validator.Validate is safe
// for concurrent use and caches struct metadata, so one instance is enough.
var requestValidator = NewValidator()

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		validate: newStructValidator(),
	}
}

// ValidateStruct validates a struct and returns validation errors
func (v *Validator) ValidateStruct(s interface{}) error {
	if err := v.validate.Struct(s); err != nil {
		return v.formatValidationError(err)
	}
	return nil
}

// formatValidationError formats validation errors into a consistent format
func (v *Validator) formatValidationError(err error) error {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var errorMessages []string
		for _, e := range validationErrors {
			errorMessages = append(errorMessages, v.getErrorMessage(e))
		}
		return fmt.Errorf("validation failed: %s", strings.Join(errorMessages, "; "))
	}
	return constant.ErrInvalidInput
}

// getErrorMessage converts a validation error to a user-friendly message
func (v *Validator) getErrorMessage(e validator.FieldError) string {
	field := e.Field()
	tag := e.Tag()
	param := e.Param()

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s%s", field, param, sizeUnit(e.Kind()))
	case "max":
		return fmt.Sprintf("%s must be at most %s%s", field, param, sizeUnit(e.Kind()))
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "len":
		return fmt.Sprintf("%s must be exactly %s%s", field, param, sizeUnit(e.Kind()))
	case "alphanum":
		return fmt.Sprintf("%s must contain only alphanumeric characters", field)
	case "username":
		return fmt.Sprintf("%s must contain only letters, numbers, underscores or hyphens", field)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, param)
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, param)
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, param)
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, param)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, param)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

// ValidateRequest validates and decodes a JSON request
func (v *Validator) ValidateRequest(r *http.Request, dest interface{}) error {
	if err := DecodeJSONBody(r, dest); err != nil {
		return err
	}
	return v.ValidateStruct(dest)
}

// sizeUnit names what min/max/len count for a field of the given kind: the
// length of a string, the number of entries of a slice/array/map, or (for
// numbers) the value itself, which needs no unit.
func sizeUnit(kind reflect.Kind) string {
	switch kind {
	case reflect.String:
		return " characters"
	case reflect.Slice, reflect.Array, reflect.Map:
		return " items"
	default:
		return ""
	}
}
