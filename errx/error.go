package errx

import "fmt"

// Error is an immutable error wrapper that can add context, classification,
// and an optionally exposed cause. Use New to construct it.
type Error struct {
	message string
	kind    Kind
	cause   error
	unwrap  bool
}

// Option configures an Error created by New.
type Option func(*options) error

type options struct {
	kind   Kind
	cause  error
	unwrap bool
}

// New creates an immutable Error. It panics when passed invalid options, an
// invalid kind, a typed-nil cause, or both an empty message and nil cause.
func New(message string, opts ...Option) *Error {
	config := options{kind: Unknown, unwrap: true}
	for _, option := range opts {
		if option == nil {
			panic("errx: nil option")
		}
		if err := option(&config); err != nil {
			panic(fmt.Sprintf("errx: invalid option: %v", err))
		}
	}
	if !config.kind.Valid() {
		panic(fmt.Sprintf("errx: invalid kind %d", config.kind))
	}
	if message == "" && config.cause == nil {
		panic("errx: message or cause is required")
	}
	return &Error{message: message, kind: config.kind, cause: config.cause, unwrap: config.unwrap}
}

// WithKind sets an Error's semantic classification.
func WithKind(kind Kind) Option {
	return func(config *options) error {
		if !kind.Valid() {
			return fmt.Errorf("invalid kind %d", kind)
		}
		config.kind = kind
		return nil
	}
}

// WithCause attaches cause to an Error. A typed-nil cause is rejected.
func WithCause(cause error) Option {
	return func(config *options) error {
		if isNilError(cause) {
			return fmt.Errorf("cause must not be nil")
		}
		config.cause = cause
		return nil
	}
}

// WithoutUnwrap retains the cause for Error's message but prevents it from
// being observable through errors.Is, errors.As, and errors.Unwrap.
func WithoutUnwrap() Option {
	return func(config *options) error {
		config.unwrap = false
		return nil
	}
}

// Error returns a human-readable message. It is nil-safe to make logging a
// nil *Error harmless.
func (e *Error) Error() string {
	switch {
	case e == nil:
		return "<nil>"
	case e.message == "" && e.cause != nil:
		return e.cause.Error()
	case e.cause == nil || !e.unwrap:
		return e.message
	default:
		return e.message + ": " + e.cause.Error()
	}
}

// Kind returns e's classification, or Unknown for a nil receiver.
func (e *Error) Kind() Kind {
	if e == nil {
		return Unknown
	}
	return e.kind
}

// Unwrap returns e's cause unless WithoutUnwrap was used.
func (e *Error) Unwrap() error {
	if e == nil || !e.unwrap {
		return nil
	}
	return e.cause
}

var _ error = (*Error)(nil)
var _ Classified = (*Error)(nil)
var _ interface{ Unwrap() error } = (*Error)(nil)
