package config

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNoConfigFile = errors.New("no config file path configured")
	ErrUnknownKey   = errors.New("key not found in config registry")
)

type ParseError struct {
	Field string
	Key   string
	Err   error
}

func (e *ParseError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("config parse error: field %s (%s): %v", e.Field, e.Key, e.Err)
	}
	return fmt.Sprintf("config parse error: field %s: %v", e.Field, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

type ValidationError struct {
	Missing []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("required config fields missing: %s", strings.Join(e.Missing, ", "))
}
