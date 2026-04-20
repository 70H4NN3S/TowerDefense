// Package models holds shared data transfer objects and validation helpers
// used by both the service layer and the HTTP handler layer.
package models

import "fmt"

// FieldError describes a single invalid field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError is returned when one or more request fields fail validation.
// Handlers convert it to a 400 response with code "validation_failed".
type ValidationError struct {
	Fields []FieldError `json:"fields"`
}

// Add appends a field-level error. Calling Add returns the receiver so callers
// can chain: v.Add("email", "invalid format").Add("password", "too short").
func (v *ValidationError) Add(field, message string) *ValidationError {
	v.Fields = append(v.Fields, FieldError{Field: field, Message: message})
	return v
}

// HasErrors reports whether any field errors have been accumulated.
func (v *ValidationError) HasErrors() bool { return len(v.Fields) > 0 }

// Error implements the error interface so ValidationError can be passed to
// RespondError and matched with errors.As.
func (v *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %d field error(s)", len(v.Fields))
}
