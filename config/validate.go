package config

import (
	"reflect"

	"github.com/gxmmx/compage-go/errx"
)

// FieldContext identifies the field being semantically validated.
type FieldContext struct {
	Field  string
	Key    string
	Origin Origin
}

// FieldValidator validates one decoded present field.
type FieldValidator func(FieldContext, any) error

// Validatable is implemented by configuration values with cross-field rules.
type Validatable interface{ ValidateConfig() error }

func resolve[T any](r registry, layers map[Source]map[string]any, path string, fileLoaded bool, validators map[string]FieldValidator) (state[T], error) {
	var out T
	outValue := reflect.ValueOf(&out).Elem()
	st := state[T]{origins: map[string]Origin{}, present: map[Source]map[string]bool{}, layers: cloneLayers(layers), path: path, fileLoaded: fileLoaded}
	for _, source := range []Source{SourceDefault, SourceInitial, SourceFile, SourceEnv, SourceFlag, SourceSet} {
		st.present[source] = map[string]bool{}
		for key := range layers[source] {
			st.present[source][key] = true
		}
	}
	for _, field := range r.fields {
		var raw any
		source := SourceNone
		for _, candidate := range []Source{SourceSet, SourceFlag, SourceEnv, SourceFile, SourceInitial, SourceDefault} {
			if value, ok := layers[candidate][field.key]; ok {
				raw, source = value, candidate
				break
			}
		}
		if source == SourceNone {
			if field.required {
				return st, configErr("required field missing", errx.Validation, field.key, field.field, "", nil, false)
			}
			continue
		}
		value, err := coerceWithSchema(raw, field.typ, field.elements)
		if err != nil {
			if source == SourceDefault {
				return st, configErr("default value", errx.Invalid, field.key, field.field, source.String(), err, false)
			}
			return st, configErr("invalid value", errx.Invalid, field.key, field.field, source.String(), err, false)
		}
		valueAt(outValue, field.index).Set(value)
		detail := ""
		if source == SourceFile {
			detail = path
		}
		if source == SourceEnv {
			detail = field.env
		}
		if source == SourceFlag {
			detail = field.flag
		}
		st.origins[field.key] = Origin{Source: source, Detail: detail}
	}
	for _, field := range r.fields {
		if field.validate == "" {
			continue
		}
		origin, ok := st.origins[field.key]
		if !ok {
			continue
		}
		if err := validators[field.validate](FieldContext{Field: field.field, Key: field.key, Origin: origin}, valueAt(outValue, field.index).Interface()); err != nil {
			return st, configErr("validation failed", errx.Validation, field.key, field.field, "", err, false)
		}
	}
	if value, ok := any(out).(Validatable); ok {
		if err := value.ValidateConfig(); err != nil {
			return st, configErr("configuration validation failed", errx.Validation, "", "", "", err, false)
		}
	}
	st.values = cloneValue(out)
	return st, nil
}
