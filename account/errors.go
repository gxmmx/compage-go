package account

import (
	"fmt"
	"github.com/gxmmx/compage-go/errx"
)

type NotFoundError struct {
	Key   string
	Cause error
}

func (e *NotFoundError) Error() string { return "account not found: " + e.Key }
func (e *NotFoundError) Unwrap() error { return e.Cause }
func (*NotFoundError) Kind() errx.Kind { return errx.NotFound }

type PrivilegeError struct {
	Capability string
	Cause      error
}

func (e *PrivilegeError) Error() string { return "account: privilege required: " + e.Capability }
func (e *PrivilegeError) Unwrap() error { return e.Cause }
func (*PrivilegeError) Kind() errx.Kind { return errx.Forbidden }

type DriftError struct{ Changes []Change }

func (e *DriftError) Error() string {
	return fmt.Sprintf("account: specification drift (%d field(s))", len(e.Changes))
}
func (*DriftError) Kind() errx.Kind { return errx.Conflict }

type UnsupportedError struct{ Capability string }

func (e *UnsupportedError) Error() string { return "account: unsupported: " + e.Capability }
func (*UnsupportedError) Kind() errx.Kind { return errx.Unavailable }

type ValidationError struct {
	Message string
	Cause   error
}

func (e *ValidationError) Error() string { return "account: invalid specification: " + e.Message }
func (e *ValidationError) Unwrap() error { return e.Cause }
func (*ValidationError) Kind() errx.Kind { return errx.Validation }
