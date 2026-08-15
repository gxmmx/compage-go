package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

func coerce(raw any, t reflect.Type) (reflect.Value, error) {
	if reflect.TypeOf(raw) == t {
		v := reflect.ValueOf(raw)
		if t == reflect.TypeFor[[]string]() {
			v = reflect.ValueOf(append([]string(nil), raw.([]string)...))
		}
		return v, nil
	}
	s, ok := raw.(string)
	if !ok {
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
		var a []string
		if err := json.Unmarshal([]byte(s), &a); err != nil {
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
		if err == nil && (x != x || x > 1.7976931348623157e308 || x < -1.7976931348623157e308) {
			err = fmt.Errorf("non-finite float")
		}
		v.SetFloat(x)
	}
	return v, err
}

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
