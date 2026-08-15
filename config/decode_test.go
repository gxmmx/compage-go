package config

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestConversionContract(t *testing.T) {
	type Config struct {
		String   string        `default:"value"`
		Bool     bool          `default:"true"`
		Int      int           `default:"-1"`
		Int64    int64         `default:"2"`
		Uint     uint          `default:"3"`
		Uint64   uint64        `default:"4"`
		Float    float64       `default:"1.5"`
		Duration time.Duration `default:"2s"`
		Tags     []string      `default:"[\"a\",\"b\"]"`
	}
	c, err := Load[Config]()
	if err != nil {
		t.Fatal(err)
	}
	v := c.Values()
	if v.String != "value" || !v.Bool || v.Int != -1 || v.Int64 != 2 || v.Uint != 3 || v.Uint64 != 4 || v.Float != 1.5 || v.Duration != 2*time.Second || len(v.Tags) != 2 {
		t.Fatalf("converted values = %#v", v)
	}
	_, err = Load[struct {
		Value int `default:"not-a-number"`
	}]()
	var defaultError *DefaultValueError
	if !errors.As(err, &defaultError) {
		t.Fatalf("default error = %T", err)
	}
}

func TestSetDoesNotRetainCallerSlice(t *testing.T) {
	c, err := Load[struct{ Tags []string }]()
	if err != nil {
		t.Fatal(err)
	}
	input := []string{"first"}
	if err := c.Set("tags", input); err != nil {
		t.Fatal(err)
	}
	input[0] = "mutated"
	if got := c.Values().Tags[0]; got != "first" {
		t.Fatalf("stored caller slice mutated to %q", got)
	}
}

func TestSetRejectsNonFiniteFloat(t *testing.T) {
	c, err := Load[struct{ Value float64 }]()
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := c.Set("value", value); err == nil {
			t.Fatalf("Set accepted %v", value)
		}
	}
}
