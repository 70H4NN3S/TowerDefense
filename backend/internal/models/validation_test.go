package models

import (
	"errors"
	"testing"
)

func TestValidationError_HasErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []FieldError
		want   bool
	}{
		{"empty", nil, false},
		{"one field", []FieldError{{Field: "email", Message: "required"}}, true},
		{"two fields", []FieldError{{Field: "a", Message: "x"}, {Field: "b", Message: "y"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := &ValidationError{Fields: tt.fields}
			if v.HasErrors() != tt.want {
				t.Errorf("HasErrors() = %v, want %v", v.HasErrors(), tt.want)
			}
		})
	}
}

func TestValidationError_Add(t *testing.T) {
	t.Parallel()

	v := &ValidationError{}
	v.Add("email", "invalid format").Add("password", "too short")

	if len(v.Fields) != 2 {
		t.Fatalf("len(Fields) = %d, want 2", len(v.Fields))
	}
	if v.Fields[0].Field != "email" || v.Fields[0].Message != "invalid format" {
		t.Errorf("Fields[0] = %+v, want {email invalid format}", v.Fields[0])
	}
	if v.Fields[1].Field != "password" || v.Fields[1].Message != "too short" {
		t.Errorf("Fields[1] = %+v, want {password too short}", v.Fields[1])
	}
}

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	v := &ValidationError{}
	v.Add("email", "required").Add("username", "required")

	if v.Error() == "" {
		t.Error("Error() must not be empty")
	}
}

func TestValidationError_ImplementsError(t *testing.T) {
	t.Parallel()

	var v error = &ValidationError{}
	v.(*ValidationError).Add("x", "y")

	var ve *ValidationError
	if !errors.As(v, &ve) {
		t.Error("errors.As failed for *ValidationError")
	}
	if !ve.HasErrors() {
		t.Error("HasErrors() must return true after Add")
	}
}
