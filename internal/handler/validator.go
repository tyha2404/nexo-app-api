package handler

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/tyha2404/nexo-app-api/internal/constant"
)

// Validator provides validation functionality for request payloads
type Validator struct {
	validate *validator.Validate
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		validate: validator.New(),
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
		return fmt.Sprintf("%s must be at least %s characters", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, param)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, param)
	case "alphanum":
		return fmt.Sprintf("%s must contain only alphanumeric characters", field)
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

// ValidatePartial validates only the fields that are present in the request
func (v *Validator) ValidatePartial(r *http.Request, dest interface{}) error {
	if err := DecodeJSONBody(r, dest); err != nil {
		return err
	}

	// Only validate non-zero fields
	val := reflect.ValueOf(dest)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	var errors []string
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip zero values
		if field.IsZero() {
			continue
		}

		// Validate the field
		fieldValue := field.Interface()
		if err := v.validate.Var(fieldValue, fieldType.Tag.Get("validate")); err != nil {
			if validationErrors, ok := err.(validator.ValidationErrors); ok {
				for _, e := range validationErrors {
					errors = append(errors, v.getErrorMessage(e))
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// RegisterCustomValidation allows registering custom validation functions
func (v *Validator) RegisterCustomValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}
