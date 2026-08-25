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
	return coerceWithSchema(raw, t, nil)
}

func coerceWithSchema(raw any, t reflect.Type, elements *elementSchema) (reflect.Value, error) {
	if t.Kind() == reflect.Slice && t != reflect.TypeFor[[]string]() {
		return coerceStructuredSlice(raw, t, elements)
	}
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

func coerceStructuredSlice(raw any, t reflect.Type, elements *elementSchema) (reflect.Value, error) {
	if elements == nil {
		var err error
		elements, err = compileElementSchema(t)
		if err != nil {
			return reflect.Value{}, err
		}
	}
	if reflect.TypeOf(raw) == t {
		return cloneReflectValue(reflect.ValueOf(raw)), nil
	}
	text, ok := raw.(string)
	if ok {
		var err error
		raw, err = parseJSONValue(text)
		if err != nil {
			return reflect.Value{}, err
		}
	}
	var items []any
	if values, ok := raw.([]any); ok {
		items = values
	} else {
		value := reflect.ValueOf(raw)
		if !value.IsValid() || value.Kind() != reflect.Slice {
			return reflect.Value{}, fmt.Errorf("expected array of objects")
		}
		items = make([]any, value.Len())
		for i := range items {
			items[i] = value.Index(i).Interface()
		}
	}
	if items == nil {
		return reflect.Value{}, fmt.Errorf("expected array of objects")
	}
	result := reflect.MakeSlice(t, len(items), len(items))
	for i, item := range items {
		object, ok := structuredObject(item)
		if !ok {
			return reflect.Value{}, fmt.Errorf("element %d is not an object", i)
		}
		element, err := coerceStructuredElement(object, elements)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("element %d: %w", i, err)
		}
		result.Index(i).Set(element)
	}
	return result, nil
}

func structuredObject(value any) (map[string]any, bool) {
	if object, ok := value.(map[string]any); ok {
		return object, true
	}
	object, ok := value.(map[any]any)
	if !ok {
		return nil, false
	}
	converted := make(map[string]any, len(object))
	for key, item := range object {
		name, ok := key.(string)
		if !ok {
			return nil, false
		}
		converted[name] = item
	}
	return converted, true
}

func coerceStructuredElement(object map[string]any, elements *elementSchema) (reflect.Value, error) {
	result := reflect.New(elements.typ).Elem()
	for key, value := range object {
		var field *elementField
		for i := range elements.fields {
			if elements.fields[i].name == key {
				field = &elements.fields[i]
				break
			}
		}
		if field == nil {
			return reflect.Value{}, fmt.Errorf("unknown element key %q", key)
		}
		converted, err := coerce(value, field.typ)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("field %q: %w", key, err)
		}
		result.Field(field.index).Set(converted)
	}
	for _, field := range elements.fields {
		if _, ok := object[field.name]; !ok {
			return reflect.Value{}, fmt.Errorf("missing element key %q", field.name)
		}
	}
	return result, nil
}

func parseJSONValue(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	value, err := decodeJSONValueFormat(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON value")
		}
		return nil, err
	}
	return value, nil
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

func cloneValue[T any](in T) T {
	value := cloneReflectValue(reflect.ValueOf(in))
	if !value.IsValid() {
		var zero T
		return zero
	}
	return value.Interface().(T)
}

func cloneReflectValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(cloneReflectValue(v.Elem()))
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(cloneReflectValue(v.Elem()))
		return out
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(cloneReflectValue(v.Index(i)))
		}
		return out
	case reflect.Array:
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(cloneReflectValue(v.Index(i)))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		for _, key := range v.MapKeys() {
			out.SetMapIndex(cloneReflectValue(key), cloneReflectValue(v.MapIndex(key)))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if out.Field(i).CanSet() && v.Field(i).CanInterface() {
				out.Field(i).Set(cloneReflectValue(v.Field(i)))
			}
		}
		return out
	default:
		return v
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
