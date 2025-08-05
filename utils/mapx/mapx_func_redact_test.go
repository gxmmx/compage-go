package maputils

import (
	"reflect"
	"testing"
)

func TestRedactFromMap(t *testing.T) {
	tests := []struct {
		input     map[string]any
		sensitive []string
		expected  map[string]any
	}{
		{
			input:     map[string]any{},
			sensitive: []string{"key1"},
			expected:  map[string]any{},
		},
		{
			input: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "subvalue"},
			},
			sensitive: []string{"key1"},
			expected: map[string]any{
				"key1": "<redacted>",
				"key2": map[string]any{"subkey": "subvalue"},
			},
		},
		{
			input: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "subvalue"},
				"key3": "value3",
			},
			sensitive: []string{"key1.subkey", "key2.subkey", "key3"},
			expected: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "<redacted>"},
				"key3": "<redacted>",
			},
		},
		{
			input: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "subvalue"},
			},
			sensitive: []string{""},
			expected: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "subvalue"},
			},
		},
		{
			input: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "subvalue"},
			},
			sensitive: []string{".foo"},
			expected: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "subvalue"},
			},
		},
		{
			input: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "subvalue"},
			},
			sensitive: []string{"key1.subkey", ""},
			expected: map[string]any{
				"key1": "value1",
				"key2": map[string]any{"subkey": "subvalue"},
			},
		},
	}

	for num, test := range tests {
		RedactFromMap(test.input, test.sensitive)
		if !reflect.DeepEqual(test.input, test.expected) {
			t.Errorf("Test index %d: sensitive: %v = %v; expected %v", num, test.sensitive, test.input, test.expected)
		}
	}
}
