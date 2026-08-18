package config

import (
	"errors"
	"testing"
)

func TestValidateRechecksThePublishedSnapshot(t *testing.T) {
	type Config struct {
		Name string `validate:"nonempty"`
	}
	c, err := Load[Config](WithInitial("name", "good"), WithValidator("nonempty", func(_ FieldContext, value any) error {
		if value.(string) == "" {
			return errors.New("empty")
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if err := c.Set("name", ""); err == nil {
		t.Fatal("Set accepted invalid state")
	}
	if got := c.Values().Name; got != "good" {
		t.Fatalf("failed Set published %q", got)
	}
}

type crossFieldConfig struct {
	Start int `default:"2" validate:"order"`
	End   int `default:"1"`
}

func (c crossFieldConfig) ValidateConfig() error {
	if c.Start >= c.End {
		return errors.New("start must be before end")
	}
	return nil
}

func TestValidatableRunsAfterFieldValidators(t *testing.T) {
	fieldValidatorRan := false
	_, err := Load[crossFieldConfig](WithValidator("order", func(_ FieldContext, value any) error {
		fieldValidatorRan = true
		if value.(int) != 2 {
			return errors.New("unexpected start")
		}
		return nil
	}))
	var validation *ValidationError
	if !fieldValidatorRan || !errors.As(err, &validation) {
		t.Fatalf("validation result = %T %v, field validator ran=%v", err, err, fieldValidatorRan)
	}
}
