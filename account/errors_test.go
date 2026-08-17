package account

import (
	"errors"
	"testing"

	"github.com/gxmmx/compage-go/errx"
)

func TestErrorKindsAndCauses(t *testing.T) {
	t.Parallel()
	cause := errors.New("cause")
	for _, test := range []struct {
		name string
		err  error
		kind errx.Kind
	}{
		{"not found", &NotFoundError{Key: "worker", Cause: cause}, errx.NotFound},
		{"privilege", &PrivilegeError{Capability: "create", Cause: cause}, errx.Forbidden},
		{"drift", &DriftError{}, errx.Conflict},
		{"unsupported", &UnsupportedError{Capability: "windows"}, errx.Unavailable},
		{"validation", &ValidationError{Message: "bad", Cause: cause}, errx.Validation},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if !errx.IsKind(test.err, test.kind) {
				t.Errorf("kind = %v, want %v", errx.KindsOf(test.err), test.kind)
			}
		})
	}
	if !errors.Is(&NotFoundError{Key: "worker", Cause: cause}, cause) {
		t.Fatal("NotFoundError does not preserve its cause")
	}
}
