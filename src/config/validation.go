package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom validators for time.Duration
	validate.RegisterValidation("duration_min", validateDurationMin)
	validate.RegisterValidation("duration_max", validateDurationMax)
}

// validateDurationMin validates that a duration is at least the specified minimum
func validateDurationMin(fl validator.FieldLevel) bool {
	duration, ok := fl.Field().Interface().(time.Duration)
	if !ok {
		return false
	}

	minDurationStr := fl.Param()
	minDuration, err := time.ParseDuration(minDurationStr)
	if err != nil {
		return false
	}

	return duration >= minDuration
}

// validateDurationMax validates that a duration is at most the specified maximum
func validateDurationMax(fl validator.FieldLevel) bool {
	duration, ok := fl.Field().Interface().(time.Duration)
	if !ok {
		return false
	}

	maxDurationStr := fl.Param()
	maxDuration, err := time.ParseDuration(maxDurationStr)
	if err != nil {
		return false
	}

	return duration <= maxDuration
}

// Validate validates all configuration fields using struct tags
func (c *Config) Validate() error {
	if err := validate.Struct(c); err != nil {
		// Format validation errors with environment variable names
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
	case "min":
		return fmt.Sprintf("must be at least %s characters long", ve.Param())
	case "duration_min":
		return fmt.Sprintf("must be at least %s", ve.Param())
	case "duration_max":
		return fmt.Sprintf("must not exceed %s", ve.Param())
	default:
		return fmt.Sprintf("failed validation for tag '%s'", ve.Tag())
	}
}
