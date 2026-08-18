# account

`account` looks up and idempotently reconciles local operating-system accounts,
groups, and optional home directories. Mutations require root; `Check` plans
the same work without changing the host.

```go
spec := account.Spec{
    Name: "worker", Group: "worker",
    Home: "/var/lib/worker", HomePolicy: account.EnsureHome,
    Shell: account.NoLoginShell, Existing: account.Reconcile,
}

plan, err := account.Check(ctx, spec) // no mutation
if err != nil { return err }

result, err := account.Ensure(ctx, spec)
if err != nil { return err } // result.Changed retains completed work on failure
_ = plan
_ = result
```

`Verify` is the default existing-account policy and returns a typed drift error
instead of changing a present account. `EnsureHome` creates only the target
directory; its parent must already exist. `Hidden` is an optional macOS policy;
Linux intentionally treats it as a no-op.

Use `Lookup` or `LookupID` for read-only inspection.
