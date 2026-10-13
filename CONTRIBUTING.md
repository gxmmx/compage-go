# Contributing

Bug reports, focused fixes, and feature proposals are welcome. Please check
existing issues first. For a larger change, open an issue before starting
implementation so the proposed behavior can be discussed.

## Set up and check your change

The project uses Go and golangci-lint versions pinned in `mise.toml`.

1. Fork and clone the repository.
2. Install the pinned tools with `mise install`.
3. Create a branch for your change.
4. Make a focused change. Add or update package tests and documentation where
   appropriate.
5. Run the relevant checks. Use `mise run check` for fast verification and
   `mise run check:all` for full verification. For focused work, pass the
   package path after `--`, for example `mise run test -- config`.
6. Push your branch to your fork and open a pull request. Describe the user
   impact and list the checks you ran.

Keep pull requests focused. If a change affects package behavior, include tests
next to the Go files they exercise. Keep generated output under `.dist/` and
out of commits.
