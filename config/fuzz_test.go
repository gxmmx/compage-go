package config

import (
	"reflect"
	"testing"
)

func FuzzJSONFormat(f *testing.F) {
	for _, seed := range []string{`{}`, `{"value":1}`, `{"value":["a"]}`, `{"value":1,"value":2}`, `[1]`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) { _, _ = decodeJSONFormat([]byte(input)) })
}

func FuzzStringConversion(f *testing.F) {
	for _, seed := range []string{"", "0", "true", "1s", "[]", "[\"x\"]", "NaN"} {
		f.Add(seed)
	}
	types := []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[bool](), reflect.TypeFor[int](), reflect.TypeFor[uint](), reflect.TypeFor[float64](), reflect.TypeFor[[]string]()}
	f.Fuzz(func(t *testing.T, input string) {
		for _, target := range types {
			_, _ = coerce(input, target)
		}
	})
}
