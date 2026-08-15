package config

import (
	"errors"
	"testing"

	"github.com/gxmmx/compage-go/errx"
)

func TestTypedErrorsAndAccessors(t *testing.T) {
	_, err := Load[int]()
	var schema *SchemaError
	if !errors.As(err, &schema) || schema.Kind() != errx.Invalid || schema.Type() == "" {
		t.Fatalf("schema error = %T %v", err, err)
	}
	c := New[testConfig]()
	err = c.Set("name", "x")
	var notLoaded *NotLoadedError
	if !errors.As(err, &notLoaded) || notLoaded.Kind() != errx.Conflict {
		t.Fatalf("not loaded error = %T", err)
	}
	err = c.Validate()
	if !errors.As(err, &notLoaded) {
		t.Fatalf("Validate before load error = %T", err)
	}
	_, err = Load[struct {
		Value string `required:"true"`
	}]()
	var required *RequiredFieldError
	if !errors.As(err, &required) || required.Key() != "value" || required.Field() != "Value" {
		t.Fatalf("required error = %T %v", err, err)
	}
}
