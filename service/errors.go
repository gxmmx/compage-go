package service

import "github.com/gxmmx/compage-go/errx"

type ValidationError struct {
	Message string
	Cause   error
}

func (e *ValidationError) Error() string { return "service: invalid specification: " + e.Message }
func (e *ValidationError) Unwrap() error { return e.Cause }
func (*ValidationError) Kind() errx.Kind { return errx.Validation }

type PrivilegeError struct {
	Capability string
	Cause      error
}

func (e *PrivilegeError) Error() string { return "service: privilege required: " + e.Capability }
func (e *PrivilegeError) Unwrap() error { return e.Cause }
func (*PrivilegeError) Kind() errx.Kind { return errx.Forbidden }

type UnsupportedError struct {
	Capability string
	Cause      error
}

func (e *UnsupportedError) Error() string { return "service: unavailable: " + e.Capability }
func (e *UnsupportedError) Unwrap() error { return e.Cause }
func (*UnsupportedError) Kind() errx.Kind { return errx.Unavailable }
