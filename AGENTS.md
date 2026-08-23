# Repository instructions

## Project

compage-go is a collection of foundational Go packages for application
infrastructure, including configuration, logging, terminal output, host
integration, certificates, and operating-system services.

Top-level directories are independent Go packages. `console/logger` and
`console/printer` are subpackages of `console`. Each package has authoritative
package documentation in `doc.go`; some packages also have a README with
extended guidance.

## Repository structure

- `.design/` contains active implementation plans and design documents.
- `.dist/` contains disposable generated output and is ignored by Git.
- `tests/unit/run.sh` is the shared entry point for package unit tests, race
  tests, coverage, and fuzz smoke tests.
- `tests/lint/run.sh` runs golangci-lint using `tests/lint/.golangci.yml`.
- Platform-specific implementations use Go filename suffixes such as
  `_darwin` and `_linux`; preserve those boundaries when changing packages.
- The root README documents the package inventory and current workflows.

## Development workflow

`mise.toml` is the current workflow entry point. Run `mise install` to install
the pinned Go and golangci-lint versions. Use `mise tasks` to discover tasks and
`mise run check` for the complete verification workflow.

Useful focused tasks include:

- `mise run test -- <package>` for uncached tests in one package;
- `mise run test:race -- <package>` for one package with the race detector; and
- `mise run test:coverage:config` for config coverage output under `.dist/`; and
- `mise run lint` for the configured golangci-lint checks.

Keep package tests next to the Go files they exercise. Run the relevant Mise
checks after code or workflow changes. The repository uses package-level Go
unit tests, with shared runners under `tests/` rather than separate integration
or end-to-end suites.

## Constraints

- Keep changes scoped to the package or documentation they affect.
- Do not add CI, release, or publishing workflows unless explicitly requested.
- Keep generated artifacts under `.dist/` and do not commit them.
- Keep `mise.toml` and its tool versions as the authoritative workflow
  configuration.
