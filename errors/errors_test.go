package errors

import (
	"fmt"
	"strings"
	"testing"
)

// Simple error type for testing
type simpleError struct {
	msg string
}

func (e simpleError) Error() string {
	return e.msg
}

func TestConstruction(t *testing.T) {
	t.Run("should create a new AppError with kind and message", func(t *testing.T) {
		err := New(KindNotFound, "not found", nil)
		if err.Kind != KindNotFound {
			t.Errorf("expected KindNotFound, got %v", err.Kind)
		}
		if err.Message != "not found" {
			t.Errorf("expected 'not found', got '%s'", err.Message)
		}
	})
	t.Run("should create a new AppError with inner AppError", func(t *testing.T) {
		innerErr := New(KindInvalidInput, "invalid input", nil)
		err := New(KindNotFound, "not found", innerErr)
		if innerAppErr, ok := err.Err.(*AppError); !ok || innerAppErr != innerErr {
			t.Errorf("expected inner error to be App error")
		}
	})
	t.Run("should create a new AppError with inner regular error", func(t *testing.T) {
		innerErr := simpleError{msg: "inner error"}
		err := New(KindNotFound, "not found", innerErr)
		if err.Err != innerErr {
			t.Errorf("expected inner error to be the same")
		}
		if err.Error() != "NotFound: not found: inner error" {
			t.Errorf("expected 'NotFound: not found: inner error', got '%s'", err.Error())
		}
	})
}

func TestWithFields(t *testing.T) {
	t.Run("should create a new AppError with field", func(t *testing.T) {
		err := New(KindNotFound, "not found", nil).WithField("testkey", "testvalue")
		if val, ok := err.Fields["testkey"]; !ok || val != "testvalue" {
			t.Errorf("expected field 'testkey' to be 'testvalue', got '%v'", val)
		}
	})
	t.Run("should create a new AppError with multiple fields", func(t *testing.T) {
		err := New(KindNotFound, "not found", nil).WithFields(map[string]any{
			"key1": "value1",
			"key2": "value2",
		})
		if val, ok := err.Fields["key1"]; !ok || val != "value1" {
			t.Errorf("expected field 'key1' to be 'value1', got '%v'", val)
		}
		if val, ok := err.Fields["key2"]; !ok || val != "value2" {
			t.Errorf("expected field 'key2' to be 'value2', got '%v'", val)
		}
	})
	t.Run("should preserve inner fields", func(t *testing.T) {
		firstError := New(KindInvalidInput, "firstinvaliderr", nil).WithField("first", "firstvalue")
		secondError := New(KindInvalidInput, "secondinvaliderr", firstError).WithField("second", "secondvalue")
		err := New(KindNotFound, "not found", secondError)
		if val, ok := err.Fields["first"]; !ok || val != "firstvalue" {
			t.Errorf("expected field 'first' to be 'firstvalue', got '%v'", val)
		}
		if val, ok := err.Fields["second"]; !ok || val != "secondvalue" {
			t.Errorf("expected field 'second' to be 'secondvalue', got '%v'", val)
		}
		if val, ok := err.Fields["testkey"]; ok {
			t.Errorf("expected field 'testkey' to not exist, got '%v'", val)
		}
	})
}

func TestConvenienceConstructors(t *testing.T) {
	t.Run("should create types of errors", func(t *testing.T) {
		kindsTested := make([]Kind, 0)

		err := AlreadyExists(nil, "already exists")
		if err.Kind != KindAlreadyExists {
			t.Errorf("expected KindAlreadyExists, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		err = NotFound(nil, "not found")
		if err.Kind != KindNotFound {
			t.Errorf("expected KindNotFound, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		err = InvalidInput(nil, "invalid input")
		if err.Kind != KindInvalidInput {
			t.Errorf("expected KindInvalidInput, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		err = Unauthorized(nil, "unauthorized")
		if err.Kind != KindUnauthorized {
			t.Errorf("expected KindUnauthorized, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)
		err = Forbidden(nil, "forbidden")
		if err.Kind != KindForbidden {
			t.Errorf("expected KindForbidden, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		err = Conflict(nil, "conflict")
		if err.Kind != KindConflict {
			t.Errorf("expected KindConflict, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		err = Timeout(nil, "timeout")
		if err.Kind != KindTimeout {
			t.Errorf("expected KindTimeout, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		err = Unavailable(nil, "unavailable")
		if err.Kind != KindUnavailable {
			t.Errorf("expected KindUnavailable, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		err = MethodNotAllowed(nil, "method not allowed")
		if err.Kind != KindMethodNotAllowed {
			t.Errorf("expected KindMethodNotAllowed, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		err = Internal(nil, "internal")
		if err.Kind != KindInternal {
			t.Errorf("expected KindInternal, got %v", err.Kind)
		}
		kindsTested = append(kindsTested, err.Kind)

		// Check if all kinds were tested
		for _, kind := range allKinds {
			found := false
			for _, testedKind := range kindsTested {
				if kind == testedKind {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("kind %v was not tested", kind)
			}
		}
	})
}

func TestErrorMethods(t *testing.T) {
	t.Run("should return correct error string", func(t *testing.T) {
		err := NotFound(nil, "Something was not found")
		if err.Error() != "NotFound: Something was not found" {
			t.Errorf("expected 'NotFound: Something was not found', got '%s'", err.Error())
		}
	})
	t.Run("should return correct error message chain with inner error", func(t *testing.T) {
		innerErr := New(KindInvalidInput, "invalid input", nil)
		err := New(KindNotFound, "not found", innerErr)
		if err.Error() != "NotFound: not found: InvalidInput: invalid input" {
			t.Errorf("expected 'NotFound: not found: InvalidInput: invalid input', got '%s'", err.Error())
		}
	})
	t.Run("shoud be able to use is", func(t *testing.T) {
		err := NotFound(nil, "Something was not found")
		if !err.Is(ErrNotFound) {
			t.Errorf("expected error to be KindNotFound")
		}
		if err.Is(ErrInvalidInput) {
			t.Errorf("expected error to not be KindInvalidInput")
		}
	})
}

func TestFormatHelper(t *testing.T) {
	err := &AppError{
		Kind:    "Validation",
		Message: "invalid input",
		Fields: map[string]any{
			"field": "username",
			"value": "admin!",
		},
		Err: nil,
	}

	t.Run("pct s format", func(t *testing.T) {
		out := fmt.Sprintf("%s", err)
		want := err.Error()
		if out != want {
			t.Errorf("%%s format mismatch: got %q, want %q", out, want)
		}
	})

	t.Run("pct q format", func(t *testing.T) {
		out := fmt.Sprintf("%q", err)
		want := fmt.Sprintf("%q", err.Error())
		if out != want {
			t.Errorf("%%q format mismatch: got %q, want %q", out, want)
		}
	})

	t.Run("pct v format", func(t *testing.T) {
		out := fmt.Sprintf("%v", err)
		want := err.Error()
		if out != want {
			t.Errorf("%%v format mismatch: got %q, want %q", out, want)
		}
	})

	t.Run("pct plus v format", func(t *testing.T) {
		out := fmt.Sprintf("%+v", err)

		if !strings.Contains(out, "Validation") ||
			!strings.Contains(out, "invalid input") ||
			!strings.Contains(out, "Fields:") ||
			!strings.Contains(out, "field: username") ||
			!strings.Contains(out, "value: admin!") {
			t.Errorf("%%+v format missing expected content:\n%s", out)
		}
	})
}

func TestHttpStatus(t *testing.T) {
	t.Run("should return correct HTTP status for each error kind", func(t *testing.T) {
		tests := []struct {
			kind     Kind
			expected int
		}{
			{KindAlreadyExists, 409},
			{KindConflict, 409},
			{KindNotFound, 404},
			{KindInvalidInput, 400},
			{KindUnauthorized, 401},
			{KindForbidden, 403},
			{KindTimeout, 504},
			{KindUnavailable, 503},
			{KindMethodNotAllowed, 405},
			{KindInternal, 500},
		}

		for _, test := range tests {
			err := &AppError{Kind: test.kind}
			if err.HTTPStatus() != test.expected {
				t.Errorf("expected %d for %s, got %d", test.expected, test.kind, err.HTTPStatus())
			}
		}
	})
}

func TestJSON(t *testing.T) {
	t.Run("should convert to json correctly", func(t *testing.T) {
		err := New(KindNotFound, "not found", nil).WithField("key", "value")
		jsonErr := err.ToJSON(false)
		if jsonErr.Kind != KindNotFound {
			t.Errorf("expected KindNotFound, got %v", jsonErr.Kind)
		}
		if jsonErr.Message != "not found" {
			t.Errorf("expected 'not found', got '%s'", jsonErr.Message)
		}
		if len(jsonErr.Fields) != 1 || jsonErr.Fields["key"] != "value" {
			t.Errorf("expected field 'key' to be 'value', got '%v'", jsonErr.Fields)
		}
	})

	t.Run("should convert json with no inner message", func(t *testing.T) {
		innerErr := New(KindInvalidInput, "invalid input", nil)
		err := New(KindNotFound, "not found", innerErr)
		jsonErr := err.ToJSON(false)
		if jsonErr.Kind != KindNotFound {
			t.Errorf("expected KindNotFound, got %v", jsonErr.Kind)
		}
		if jsonErr.Message != "not found" {
			t.Errorf("expected 'not found', got '%s'", jsonErr.Message)
		}
		if len(jsonErr.Fields) != 0 {
			t.Errorf("expected no fields, got '%v'", jsonErr.Fields)
		}
	})
	t.Run("should convert json and mask internal error message and fields", func(t *testing.T) {
		err := New(KindInternal, "custommessageshouldnotprint", nil).WithField("key", "value")
		jsonErr := err.ToJSON(false)
		if jsonErr.Kind != KindInternal {
			t.Errorf("expected KindInternal, got %v", jsonErr.Kind)
		}
		if jsonErr.Message != "internal error" {
			t.Errorf("expected 'internal error', got '%s'", jsonErr.Message)
		}
		if len(jsonErr.Fields) != 0 {
			t.Errorf("expected no fields, got '%v'", jsonErr.Fields)
		}
	})
	t.Run("should marshal to json correctly", func(t *testing.T) {
		err := New(KindNotFound, "not found", nil).WithField("key", "value")
		jsonData, _ := err.MarshalJSON()
		expectedJSON := `{"kind":"NotFound","message":"not found","fields":{"key":"value"}}`
		if string(jsonData) != expectedJSON {
			t.Errorf("expected %s, got %s", expectedJSON, string(jsonData))
		}
	})
}
