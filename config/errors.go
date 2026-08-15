package config

import (
	"fmt"

	"github.com/gxmmx/compage-go/errx"
)

// Error is a contextual configuration error. Its text never includes raw values.
type Error struct {
	typ, key, field, source, path string
	kind                          errx.Kind
	cause                         error
	unwrap                        bool
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	msg := "config: " + e.typ
	if e.key != "" {
		msg += fmt.Sprintf(" for %q", e.key)
	}
	if e.source != "" {
		msg += " from " + e.source
	}
	return msg
}
func (e *Error) Kind() errx.Kind {
	if e == nil {
		return errx.Unknown
	}
	return e.kind
}
func (e *Error) Unwrap() error {
	if e != nil && e.unwrap {
		return e.cause
	}
	return nil
}
func (e *Error) Key() string {
	if e == nil {
		return ""
	}
	return e.key
}
func (e *Error) Field() string {
	if e == nil {
		return ""
	}
	return e.field
}
func (e *Error) Type() string {
	if e == nil {
		return ""
	}
	return e.typ
}

// Source returns the source label associated with the failure, when relevant.
func (e *Error) Source() string {
	if e == nil {
		return ""
	}
	return e.source
}

// Path returns the file path associated with a file or persistence failure.
func (e *Error) Path() string {
	if e == nil {
		return ""
	}
	return e.path
}

func configErr(typ string, kind errx.Kind, key, field, source string, cause error, unwrap bool) error {
	return classifyError(&Error{typ: typ, kind: kind, key: key, field: field, source: source, cause: cause, unwrap: unwrap})
}

func fileErr(typ string, kind errx.Kind, path string, cause error, unwrap bool) error {
	return classifyError(&Error{typ: typ, kind: kind, source: "file", path: path, cause: cause, unwrap: unwrap})
}

func classifyError(base *Error) error {
	typ := base.typ
	switch typ {
	case "invalid option", "nil option", "invalid config environment variable", "invalid environment prefix":
		return &OptionError{errorDetail{base}}
	case "schema must be a struct", "excluded field has config tags", "invalid config key", "embedded field cannot have cfg tag", "unsupported nested field tag", "unsupported field type", "invalid required tag", "required field has default", "invalid sensitive tag", "invalid save tag", "unknown validator", "invalid env tag", "invalid flag tag", "duplicate config key", "scalar conflicts with namespace", "duplicate environment variable", "duplicate flag":
		return &SchemaError{errorDetail{base}}
	case "default value":
		return &DefaultValueError{errorDetail{base}}
	case "invalid value", "decode file":
		return &DecodeError{errorDetail{base}}
	case "unknown key", "unknown initial key", "unknown file key":
		return &UnknownKeyError{errorDetail{base}}
	case "required field missing":
		return &RequiredFieldError{errorDetail{base}}
	case "validation failed", "configuration validation failed":
		return &ValidationError{errorDetail{base}}
	case "flag binding":
		return &FlagBindingError{errorDetail{base}}
	case "not loaded":
		return &NotLoadedError{errorDetail{base}}
	case "no config file":
		return &NoConfigFileError{errorDetail{base}}
	case "read file", "home directory":
		return &FileReadError{errorDetail{base}}
	case "save file", "create temporary file", "stat save target", "unsafe save target", "encode file":
		return &SaveError{errorDetail{base}}
	case "unsupported file format", "invalid file path":
		return &FileFormatError{errorDetail{base}}
	case "parse file":
		return &FileParseError{errorDetail{base}}
	default:
		return base
	}
}

// Typed errors are stable inspection points; their common accessors are promoted
// from the immutable embedded Error.
type errorDetail struct{ e *Error }

func (d errorDetail) Error() string   { return d.e.Error() }
func (d errorDetail) Kind() errx.Kind { return d.e.Kind() }
func (d errorDetail) Unwrap() error   { return d.e.Unwrap() }
func (d errorDetail) Key() string     { return d.e.Key() }
func (d errorDetail) Field() string   { return d.e.Field() }
func (d errorDetail) Type() string    { return d.e.Type() }
func (d errorDetail) Source() string  { return d.e.Source() }
func (d errorDetail) Path() string    { return d.e.Path() }

type OptionError struct{ errorDetail }
type SchemaError struct{ errorDetail }
type DefaultValueError struct{ errorDetail }
type DecodeError struct{ errorDetail }
type UnknownKeyError struct{ errorDetail }
type RequiredFieldError struct{ errorDetail }
type ValidationError struct{ errorDetail }
type FlagBindingError struct{ errorDetail }
type NotLoadedError struct{ errorDetail }
type NoConfigFileError struct{ errorDetail }
type FileReadError struct{ errorDetail }
type SaveError struct{ errorDetail }
type FileFormatError struct{ errorDetail }
type FileParseError struct{ errorDetail }

var _ error = (*Error)(nil)
var _ errx.Classified = (*Error)(nil)
var _ error = (*OptionError)(nil)
var _ errx.Classified = (*OptionError)(nil)
var _ error = (*SchemaError)(nil)
var _ errx.Classified = (*SchemaError)(nil)
var _ error = (*DecodeError)(nil)
var _ errx.Classified = (*DecodeError)(nil)
var _ error = (*ValidationError)(nil)
var _ errx.Classified = (*ValidationError)(nil)
