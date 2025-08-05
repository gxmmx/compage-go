package errors

import (
	"fmt"
	"testing"
)

func TestConvenienceConstructors(t *testing.T) {
	tests := []struct {
		expected    string
		constructor func(string, error) ApplicationError
	}{
		{expected: "AlreadyExists", constructor: NewAlreadyExists},
		{expected: "NotFound", constructor: NewNotFound},
		{expected: "InvalidInput", constructor: NewInvalidInput},
		{expected: "ValidationFailed", constructor: NewValidationFailed},
		{expected: "Unauthorized", constructor: NewUnauthorized},
		{expected: "Forbidden", constructor: NewForbidden},
		{expected: "Conflict", constructor: NewConflict},
		{expected: "Timeout", constructor: NewTimeout},
		{expected: "Unavailable", constructor: NewUnavailable},
		{expected: "MethodNotAllowed", constructor: NewMethodNotAllowed},
		{expected: "Cancelled", constructor: NewCancelled},
		{expected: "NotImplemented", constructor: NewNotImplemented},
		{expected: "PreconditionFailed", constructor: NewPreconditionFailed},
		{expected: "RateLimited", constructor: NewRateLimited},
		{expected: "BadGateway", constructor: NewBadGateway},
		{expected: "DependencyFailed", constructor: NewDependencyFailed},
		{expected: "CorruptedData", constructor: NewCorruptedData},
		{expected: "NotSupported", constructor: NewNotSupported},
		{expected: "AssertionFailed", constructor: NewAssertionFailed},
		{expected: "Internal", constructor: NewInternal},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Constructor: %s", tt.expected), func(t *testing.T) {
			err := tt.constructor("test message", nil)
			if err == nil {
				t.Fatal("expected non-nil error")
			}

			if err.Error() != fmt.Sprintf("%s (%s): test message", tt.expected, tt.expected) {
				t.Errorf("unexpected error message: %s", err.Error())
			}

			if !err.IsKind(Kind(tt.expected)) {
				t.Errorf("expected error to be of kind %s", tt.expected)
			}
		})
	}
}
