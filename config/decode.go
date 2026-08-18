package config

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func coerce(raw any, t reflect.Type) (reflect.Value, error) {
	if reflect.TypeOf(raw) == t {
		v := reflect.ValueOf(raw)
		if t == reflect.TypeFor[float64]() && !finite(v.Float()) {
			return reflect.Value{}, fmt.Errorf("non-finite float")
		}
		if t == reflect.TypeFor[[]string]() {
			v = reflect.ValueOf(append([]string(nil), raw.([]string)...))
		}
		return v, nil
	}
	s, ok := raw.(string)
	if !ok {
		// File formats preserve scalar types. A string field accepts only a
		// format-native string; silently formatting numbers and booleans would
		// turn a schema/type error into a surprising configuration value.
		if t == reflect.TypeFor[string]() {
			return reflect.Value{}, fmt.Errorf("expected string")
		}
		switch n := raw.(type) {
		case json.Number:
			s = n.String()
		case bool, int, int64, uint, uint64, float64:
			s = fmt.Sprint(n)
		case []any:
			if t != reflect.TypeFor[[]string]() {
				return reflect.Value{}, fmt.Errorf("expected %s", t)
			}
			a := make([]string, len(n))
			for i, item := range n {
				var valid bool
				a[i], valid = item.(string)
				if !valid {
					return reflect.Value{}, fmt.Errorf("list element is not a string")
				}
			}
			return reflect.ValueOf(a), nil
		default:
			return reflect.Value{}, fmt.Errorf("expected %s", t)
		}
	}
	if t == reflect.TypeFor[[]string]() {
		a, err := parseStringSlice(s)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(a), nil
	}
	if t == reflect.TypeFor[time.Duration]() {
		v, err := time.ParseDuration(s)
		return reflect.ValueOf(v), err
	}
	v := reflect.New(t).Elem()
	var err error
	switch t.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		var x bool
		x, err = strconv.ParseBool(s)
		v.SetBool(x)
	case reflect.Int, reflect.Int64:
		var x int64
		x, err = strconv.ParseInt(s, 10, t.Bits())
		v.SetInt(x)
	case reflect.Uint, reflect.Uint64:
		var x uint64
		x, err = strconv.ParseUint(s, 10, t.Bits())
		v.SetUint(x)
	case reflect.Float64:
		var x float64
		x, err = strconv.ParseFloat(s, 64)
		if err == nil && !finite(x) {
			err = fmt.Errorf("non-finite float")
		}
		v.SetFloat(x)
	}
	return v, err
}

// parseStringSlice parses textual string slices using pflag-compatible CSV
// syntax. Its optional outer brackets consume pflag Value.String output.
func parseStringSlice(text string) ([]string, error) {
	if len(text) >= 2 && text[0] == '[' && text[len(text)-1] == ']' {
		text = text[1 : len(text)-1]
	}
	if text == "" {
		return []string{}, nil
	}
	reader := csv.NewReader(strings.NewReader(text))
	values, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if _, err := reader.Read(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple CSV records")
		}
		return nil, err
	}
	return values, nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func cloneValue[T any](in T) T { v := reflect.ValueOf(&in).Elem(); cloneReflect(v); return in }
func cloneReflect(v reflect.Value) {
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			cloneReflect(v.Field(i))
		}
	}
	if v.Kind() == reflect.Slice && !v.IsNil() {
		n := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(n, v)
		v.Set(n)
	}
}
func valueAt(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		v = v.Field(i)
	}
	return v
}
func reflectValue[T any](v T) reflect.Value                     { return reflect.ValueOf(v) }
func reflectValueAt(v reflect.Value, index []int) reflect.Value { return valueAt(v, index) }
