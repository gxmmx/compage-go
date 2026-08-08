package term

import (
	"bytes"
	"testing"
)

func TestIsTerminal_Buffer(t *testing.T) {
	var buf bytes.Buffer
	if IsTerminal(&buf) {
		t.Error("expected buffer to not be a terminal")
	}
}
