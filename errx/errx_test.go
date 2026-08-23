package errx

import (
	"errors"
	"testing"
)

type classifiedError struct {
	kind  Kind
	cause error
}

func (e *classifiedError) Error() string { return "classified" }
func (e *classifiedError) Kind() Kind    { return e.kind }
func (e *classifiedError) Unwrap() error { return e.cause }

type cyclicError struct {
	kind Kind
	next error
}

func (e *cyclicError) Error() string { return "cycle" }
func (e *cyclicError) Kind() Kind    { return e.kind }
func (e *cyclicError) Unwrap() error { return e.next }

func TestKind(t *testing.T) {
	if !Internal.Valid() || Kind(99).Valid() {
		t.Fatal("Kind.Valid returned an unexpected result")
	}
	if got, want := NotFound.String(), "not_found"; got != want {
		t.Fatalf("NotFound.String() = %q, want %q", got, want)
	}
	if got, want := Kind(99).String(), "unknown"; got != want {
		t.Fatalf("unknown Kind.String() = %q, want %q", got, want)
	}
}

func TestKindOf(t *testing.T) {
	notFound := &classifiedError{kind: NotFound}
	unavailable := &classifiedError{kind: Unavailable}

	tests := []struct {
		name string
		err  error
		want Kind
		ok   bool
	}{
		{name: "nil"},
		{name: "unclassified", err: errors.New("plain")},
		{name: "explicit internal", err: &classifiedError{kind: Internal}, want: Internal, ok: true},
		{name: "unknown continues inward", err: &classifiedError{kind: Unknown, cause: notFound}, want: NotFound, ok: true},
		{name: "invalid continues inward", err: &classifiedError{kind: Kind(99), cause: notFound}, want: NotFound, ok: true},
		{name: "outer wins", err: &classifiedError{kind: Unavailable, cause: notFound}, want: Unavailable, ok: true},
		{name: "same joined kinds", err: errors.Join(notFound, &classifiedError{kind: NotFound}), want: NotFound, ok: true},
		{name: "conflicting joined kinds", err: errors.Join(notFound, unavailable)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := KindOf(test.err)
			if got != test.want || ok != test.ok {
				t.Fatalf("KindOf() = (%v, %v), want (%v, %v)", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestKindsOf(t *testing.T) {
	err := &classifiedError{
		kind: Unavailable,
		cause: errors.Join(
			&classifiedError{kind: NotFound},
			&classifiedError{kind: Validation},
			&classifiedError{kind: NotFound},
		),
	}
	want := []Kind{Unavailable, NotFound, Validation}
	got := KindsOf(err)
	if len(got) != len(want) {
		t.Fatalf("KindsOf() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("KindsOf() = %v, want %v", got, want)
		}
	}
	if !IsKind(err, Unavailable) || IsKind(err, NotFound) {
		t.Fatal("IsKind did not use the effective outer kind")
	}
}

func TestInspectionHandlesCycles(t *testing.T) {
	cycle := &cyclicError{kind: Unknown}
	cycle.next = cycle
	if kind, ok := KindOf(cycle); kind != Unknown || ok {
		t.Fatalf("KindOf(cycle) = (%v, %v), want (Unknown, false)", kind, ok)
	}
	if got := KindsOf(cycle); len(got) != 0 {
		t.Fatalf("KindsOf(cycle) = %v, want no kinds", got)
	}
}

func TestError(t *testing.T) {
	cause := errors.New("database unavailable")
	err := New("save profile", WithKind(Unavailable), WithCause(cause))
	if got, want := err.Error(), "save profile: database unavailable"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) || !IsKind(err, Unavailable) {
		t.Fatal("wrapped error was not inspectable")
	}

	hidden := New("authentication failed", WithKind(Unauthenticated), WithCause(cause), WithoutUnwrap())
	if got, want := hidden.Error(), "authentication failed"; got != want {
		t.Fatalf("hidden Error() = %q, want %q", got, want)
	}
	if errors.Is(hidden, cause) {
		t.Fatal("WithoutUnwrap exposed the cause")
	}

	onlyCause := New("", WithCause(cause))
	if got, want := onlyCause.Error(), cause.Error(); got != want {
		t.Fatalf("cause-only Error() = %q, want %q", got, want)
	}
}

func TestNewPanicsForInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		call func()
	}{
		{name: "empty", call: func() { _ = New("") }},
		{name: "invalid kind", call: func() { _ = New("x", WithKind(Kind(99))) }},
		{name: "nil cause", call: func() { _ = New("x", WithCause(nil)) }},
		{name: "nil option", call: func() { _ = New("x", nil) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("New did not panic")
				}
			}()
			test.call()
		})
	}
}
