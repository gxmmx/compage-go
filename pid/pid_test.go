package pid

import "testing"

func TestChild(t *testing.T) {
	tests := []struct {
		name     string
		parent   *PID
		childID  string
		expected *PID
	}{
		{
			name:     "Child with valid ID",
			parent:   New("localhost", "parent"),
			childID:  "child",
			expected: New("localhost", "parent/child"),
		},
		{
			name:     "Child with empty ID",
			parent:   New("localhost", "parent"),
			childID:  "",
			expected: New("localhost", "parent/"),
		},
		{
			name:     "Child with nested ID",
			parent:   New("localhost", "parent"),
			childID:  "nested/child",
			expected: New("localhost", "parent/nested/child"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child := tt.parent.Child(tt.childID)
			if !child.Equals(tt.expected) {
				t.Errorf("Child() = %v, expected %v", child, tt.expected)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		pid      *PID
		expected string
	}{
		{
			name:     "PID with valid address and ID",
			pid:      New("localhost:8080", "parent"),
			expected: "localhost:8080/parent",
		},
		{
			name:     "PID with empty ID",
			pid:      New("localhost:8080", ""),
			expected: "localhost:8080/",
		},
		{
			name:     "PID with nested ID",
			pid:      New("localhost:8080", "parent/child"),
			expected: "localhost:8080/parent/child",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pid.String()
			if result != tt.expected {
				t.Errorf("String() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
