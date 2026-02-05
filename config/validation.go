package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Validate validates all configuration fields using struct tags
func (c *Config) Validate() error {
	if err := validate.Struct(c); err != nil {
		return formatValidationErrors(err)
	}
	return nil
}

// formatValidationErrors formats validator errors to include environment variable names
func formatValidationErrors(err error) error {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return &ValidationError{Message: err.Error()}
	}

	var messages []string
	t := reflect.TypeOf(Config{})

	for _, ve := range validationErrors {
		field, found := t.FieldByName(ve.Field())
		if !found {
			messages = append(messages, fmt.Sprintf("%s: %s", ve.Field(), getValidationMessage(ve)))
			continue
		}

		envVarName := field.Tag.Get("env")
		if envVarName == "" {
			envVarName = ve.Field()
		}

		message := fmt.Sprintf("%s: %s", envVarName, getValidationMessage(ve))
		messages = append(messages, message)
	}

	return &ValidationError{Message: strings.Join(messages, "; ")}
}

// getValidationMessage returns a human-readable validation error message
func getValidationMessage(ve validator.FieldError) string {
	switch ve.Tag() {
	case "required":
		return "is required"
	default:
		return fmt.Sprintf("failed validation for tag '%s'", ve.Tag())
	}
}
