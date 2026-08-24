package certs

import (
	"fmt"

	"github.com/gxmmx/compage-go/errx"
)

type ValidationError struct {
	Message string
	Cause   error
}

func (e *ValidationError) Error() string { return "certs: invalid: " + e.Message }
func (e *ValidationError) Unwrap() error { return e.Cause }
func (*ValidationError) Kind() errx.Kind { return errx.Validation }

type NotFoundError struct {
	Resource string
	Cause    error
}

func (e *NotFoundError) Error() string { return "certs: not found: " + e.Resource }
func (e *NotFoundError) Unwrap() error { return e.Cause }
func (*NotFoundError) Kind() errx.Kind { return errx.NotFound }

type ForbiddenError struct {
	Message string
	Cause   error
}

func (e *ForbiddenError) Error() string { return "certs: forbidden: " + e.Message }
func (e *ForbiddenError) Unwrap() error { return e.Cause }
func (*ForbiddenError) Kind() errx.Kind { return errx.Forbidden }

type ConflictError struct {
	Message string
	Cause   error
}

func (e *ConflictError) Error() string { return "certs: conflict: " + e.Message }
func (e *ConflictError) Unwrap() error { return e.Cause }
func (*ConflictError) Kind() errx.Kind { return errx.Conflict }

type PendingConflictError struct {
	Operation PendingKind
}

func (e *PendingConflictError) Error() string {
	return "certs: pending operation: " + string(e.Operation)
}
func (*PendingConflictError) Kind() errx.Kind { return errx.Conflict }

type DriftError struct {
	Field            string
	Expected, Actual any
}

func (e *DriftError) Error() string { return fmt.Sprintf("certs: configuration drift in %s", e.Field) }
func (*DriftError) Kind() errx.Kind { return errx.Conflict }

type CorruptError struct {
	Message string
	Cause   error
}

func (e *CorruptError) Error() string { return "certs: corrupt: " + e.Message }
func (e *CorruptError) Unwrap() error { return e.Cause }
func (*CorruptError) Kind() errx.Kind { return errx.Invalid }

type UnavailableError struct {
	Message string
	Cause   error
}

func (e *UnavailableError) Error() string { return "certs: unavailable: " + e.Message }
func (e *UnavailableError) Unwrap() error { return e.Cause }
func (*UnavailableError) Kind() errx.Kind { return errx.Unavailable }

// PartialOperationError reports an operation whose durable part succeeded but
// whose cleanup did not. State remains loadable and a retry is safe.
type PartialOperationError struct {
	Operation string
	Cause     error
}

func (e *PartialOperationError) Error() string { return "certs: partial " + e.Operation }
func (e *PartialOperationError) Unwrap() error { return e.Cause }
func (*PartialOperationError) Kind() errx.Kind { return errx.Internal }

func invalid(message string, cause error) error {
	return &ValidationError{Message: message, Cause: cause}
}
func corrupt(message string, cause error) error { return &CorruptError{Message: message, Cause: cause} }
