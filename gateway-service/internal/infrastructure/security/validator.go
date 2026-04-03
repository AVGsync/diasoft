package security

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Validator struct {
	validator *validator.Validate
}

func NewValidator() *Validator {
	return &Validator{validator: validator.New()}
}

func (v *Validator) ValidateStruct(value interface{}) (bool, error) {
	if err := v.validator.Struct(value); err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return false, errors.New("internal validation error")
		}

		validationErrors, ok := err.(validator.ValidationErrors)
		if !ok {
			return false, errors.New("validation error")
		}

		messages := make([]string, 0, len(validationErrors))
		for _, fieldError := range validationErrors {
			field := strings.ToLower(fieldError.Field())

			switch fieldError.Tag() {
			case "required":
				messages = append(messages, fmt.Sprintf("field %s is required", field))
			case "email":
				messages = append(messages, fmt.Sprintf("field %s must be a valid email", field))
			case "min":
				messages = append(messages, fmt.Sprintf("field %s must have at least %s characters", field, fieldError.Param()))
			case "max":
				messages = append(messages, fmt.Sprintf("field %s must have at most %s characters", field, fieldError.Param()))
			case "oneof":
				messages = append(messages, fmt.Sprintf("field %s must be one of: %s", field, fieldError.Param()))
			case "gte":
				messages = append(messages, fmt.Sprintf("field %s must be greater than or equal to %s", field, fieldError.Param()))
			case "lte":
				messages = append(messages, fmt.Sprintf("field %s must be less than or equal to %s", field, fieldError.Param()))
			case "len":
				messages = append(messages, fmt.Sprintf("field %s must contain exactly %s characters", field, fieldError.Param()))
			default:
				messages = append(messages, fmt.Sprintf("field %s failed %s validation", field, fieldError.Tag()))
			}
		}

		return false, errors.New(strings.Join(messages, "; "))
	}

	return true, nil
}
