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
	got, err := testOperation(b, true).run(context.Background(), Spec{Name: "worker", Existing: Reconcile}, false)
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
func TestValidate(t *testing.T) {
	t.Parallel()
	for _, s := range []Spec{{Name: ""}, {Name: "bad name"}, {Name: "worker", HomePolicy: EnsureHome, Home: "relative"}, {Name: "worker", UID: intPtr(-1)}} {
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
}

func (f *fakeBackend) lookup(context.Context, string, bool) (Record, error) {
	f.lookups++
	if f.lookupErr != nil {
		return Record{}, f.lookupErr
	}
	if f.applies > 0 {
		return f.after, nil
	}
	return f.record, nil
}
func (f *fakeBackend) homeExists(context.Context, string) (bool, error) { return f.homePresent, nil }
func (f *fakeBackend) apply(context.Context, Spec, *Record) error       { f.applies++; return nil }
