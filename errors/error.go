package errors

import (
	"fmt"
	"strings"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type ApplicationError interface {
	Error() string
	HTTPStatus() int
	Unwrap() error
	Is(target error) bool
	WithField(key string, value any) *AppError
	WithFields(fields map[string]any) *AppError
	AllFields() map[string]any
	Format(f fmt.State, verb rune)
	ToJSON(debug bool) JSONError
	MarshalJSON() ([]byte, error)
}

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type AppError struct {
	Kind    Kind
	Message string
	Err     error
	Fields  map[string]any
}

// -----------------------------------------------------------------------------
// Public Functions
// -----------------------------------------------------------------------------

func (e *AppError) Error() string {
	msg := string(e.Kind)

	if e.Message != "" {
		msg += ": " + e.Message
	}

	// Walk the chain of errors manually
	err := e.Err
	for err != nil {
		msg += ": " + err.Error()

		// Try to unwrap further
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = unwrapper.Unwrap()
	}

	return msg
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) Is(target error) bool {
	te, ok := target.(*AppError)
	return ok && e.Kind == te.Kind
}

func (e *AppError) WithField(key string, value any) *AppError {
	if e.Fields == nil {
		e.Fields = make(map[string]any)
	}
	e.Fields[key] = value
	return e
}

func (e *AppError) WithFields(fields map[string]any) *AppError {
	if e.Fields == nil {
		e.Fields = make(map[string]any)
	}
	for k, v := range fields {
		e.Fields[k] = v
	}
	return e
}

func (e *AppError) AllFields() map[string]any {
	merged := make(map[string]any)
	current := e
	for current != nil {
		for k, v := range current.Fields {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
		if next, ok := current.Err.(*AppError); ok {
			current = next
		} else {
			break
		}
	}
	return merged
}

func (e *AppError) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('+') {
			var b strings.Builder
			b.WriteString(string(e.Kind))
			if e.Message != "" {
				b.WriteString(": ")
				b.WriteString(e.Message)
			}
			if len(e.Fields) > 0 {
				b.WriteString("\nFields:")
				for k, v := range e.Fields {
					b.WriteString(fmt.Sprintf("\n  %s: %v", k, v))
				}
			}
			fmt.Fprint(f, b.String())
			return
		}
		fallthrough
	case 's':
		fmt.Fprint(f, e.Error())
	case 'q':
		fmt.Fprintf(f, "%q", e.Error())
	}
}
