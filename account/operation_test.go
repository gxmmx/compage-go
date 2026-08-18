package account

import (
	"context"
	"errors"
	"testing"

	"github.com/gxmmx/compage-go/errx"
	"github.com/gxmmx/compage-go/host"
)

func TestCheckPlansCreationWithoutMutation(t *testing.T) {
	t.Parallel()
	b := &fakeBackend{lookupErr: &NotFoundError{Key: "worker"}}
	got, err := testOperation(b, true).run(context.Background(), Spec{Name: "worker", Group: "worker", Existing: Reconcile}, false)
	if err != nil || !got.Created || b.applies != 0 {
		t.Fatalf("Check = %#v, %v; applies = %d", got, err, b.applies)
	}
}
func TestCheckReportsDriftWithoutMutation(t *testing.T) {
	t.Parallel()
	b := &fakeBackend{record: Record{Name: "worker", Group: "staff"}}
	_, err := testOperation(b, true).run(context.Background(), Spec{Name: "worker", Group: "ops", Existing: Verify}, false)
	var drift *DriftError
	if !errors.As(err, &drift) || !errx.IsKind(err, errx.Conflict) || b.applies != 0 {
		t.Fatalf("Check error = %v, applies = %d", err, b.applies)
	}
}
func TestCheckPerformsNonMutatingBackendPreflight(t *testing.T) {
	t.Parallel()
	cause := errors.New("capability unavailable")
	b := &fakeBackend{record: Record{Name: "worker"}, preflightErr: cause}
	_, err := testOperation(b, true).run(context.Background(), Spec{Name: "worker", Existing: Reconcile}, false)
	if !errors.Is(err, cause) || b.preflights != 1 || b.applies != 0 {
		t.Fatalf("Check error = %v, preflights = %d, applies = %d", err, b.preflights, b.applies)
	}
}
func TestEnsureAppliesAndReturnsObservedAccount(t *testing.T) {
	t.Parallel()
	b := &fakeBackend{record: Record{Name: "worker", Group: "staff"}, after: Record{Name: "worker", Group: "ops"}}
	got, err := testOperation(b, true).run(context.Background(), Spec{Name: "worker", Group: "ops", Existing: Reconcile}, true)
	if err != nil || b.applies != 1 || got.Account.Group != "ops" {
		t.Fatalf("Ensure = %#v, %v; applies = %d", got, err, b.applies)
	}
}
func TestRequireHomeAndCancellation(t *testing.T) {
	t.Parallel()
	b := &fakeBackend{record: Record{Name: "worker", Home: "/var/lib/worker"}}
	_, err := testOperation(b, true).run(context.Background(), Spec{Name: "worker", Home: "/var/lib/worker", HomePolicy: RequireHome, Existing: Reconcile}, false)
	var drift *DriftError
	if !errors.As(err, &drift) || b.applies != 0 {
		t.Fatalf("Check error = %v, applies = %d", err, b.applies)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = testOperation(b, true).run(ctx, Spec{Name: "worker"}, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Ensure cancellation = %v", err)
	}
}
func TestEnsureReturnsOnlyCompletedChangesAfterPartialFailure(t *testing.T) {
	t.Parallel()
	b := &fakeBackend{
		lookupErr: &NotFoundError{Key: "worker"},
		completed: []Change{{Field: "group", Before: "absent", After: "worker"}},
		applyErr:  errors.New("user creation failed"),
	}
	got, err := testOperation(b, true).run(context.Background(), Spec{Name: "worker", Group: "worker", Existing: Reconcile}, true)
	if !errors.Is(err, b.applyErr) || len(got.Changed) != 1 || got.Changed[0].Field != "group" {
		t.Fatalf("Ensure = %#v, %v; want completed group mutation and cause", got, err)
	}
}
func TestEnsureCreationRequiresDeclaredHome(t *testing.T) {
	t.Parallel()
	b := &fakeBackend{lookupErr: &NotFoundError{Key: "worker"}, after: Record{Name: "worker", Home: "/srv/worker"}}
	_, err := testOperation(b, true).run(context.Background(), Spec{Name: "worker", Group: "worker", Home: "/srv/worker", HomePolicy: RequireHome, Existing: Reconcile}, true)
	var drift *DriftError
	if !errors.As(err, &drift) || b.applies != 1 {
		t.Fatalf("Ensure error = %v, applies = %d", err, b.applies)
	}
}
func TestValidate(t *testing.T) {
	t.Parallel()
	for _, s := range []Spec{{Name: ""}, {Name: "bad name"}, {Name: "-option"}, {Name: "worker", Groups: []string{"-option"}}, {Name: "worker", HomePolicy: EnsureHome, Home: "relative"}, {Name: "worker", UID: intPtr(-1)}} {
		if err := validate(s); err == nil {
			t.Errorf("validate(%#v) succeeded", s)
		}
	}
}
func testOperation(b backend, root bool) operation {
	return operation{deps: dependencies{platform: host.PlatformInfo{OS: host.Linux}, root: root, backend: b}}
}
func intPtr(n int) *int { return &n }

type fakeBackend struct {
	record, after    Record
	lookupErr        error
	lookups, applies int
	homePresent      bool
	completed        []Change
	applyErr         error
	preflights       int
	preflightErr     error
}

func (f *fakeBackend) lookup(context.Context, string, bool) (Record, error) {
	f.lookups++
	if f.lookupErr != nil && f.applies == 0 {
		return Record{}, f.lookupErr
	}
	if f.applies > 0 {
		return f.after, nil
	}
	return f.record, nil
}
func (f *fakeBackend) homeExists(context.Context, string) (bool, error) { return f.homePresent, nil }
func (f *fakeBackend) preflight(context.Context, Spec, bool) error {
	f.preflights++
	return f.preflightErr
}
func (f *fakeBackend) apply(context.Context, Spec, *Record) ([]Change, error) {
	f.applies++
	return f.completed, f.applyErr
}
