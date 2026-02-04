package models

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// ValidateRequest validates a request struct and returns a formatted error message
func ValidateRequest(req interface{}) error {
	if err := validate.Struct(req); err != nil {
		validationErrors, ok := err.(validator.ValidationErrors)
		if !ok {
			return fmt.Errorf("validation error: %v", err)
		}

		var messages []string
		for _, ve := range validationErrors {
			message := getValidationMessage(ve)
			messages = append(messages, fmt.Sprintf("%s: %s", ve.Field(), message))
		}

		return fmt.Errorf(strings.Join(messages, "; "))
	}
	return nil
}

// getValidationMessage returns a human-readable validation error message
func getValidationMessage(ve validator.FieldError) string {
	switch ve.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters long", ve.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters long", ve.Param())
	default:
		return fmt.Sprintf("failed validation for tag '%s'", ve.Tag())
	}
}
