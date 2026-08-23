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
- Platform-specific implementations use Go filename suffixes such as
  `_darwin` and `_linux`; preserve those boundaries when changing packages.
- The root README documents the package inventory and current workflows.

## Development workflow

`Taskfile.yml` is the current workflow entry point. Mise is not configured yet.
Use `task --list` to discover tasks and `task check` for the complete
verification workflow.

Useful focused tasks include:

- `task test:<package>` for uncached tests in one package;
- `task test:race:<package>` for one package with the race detector; and
- `task test:coverage:config` for config coverage output under `.dist/`.

Keep package tests next to the Go files they exercise. Run the relevant Task
checks after code or workflow changes.

## Constraints

- Keep changes scoped to the package or documentation they affect.
- Do not add CI, release, or publishing workflows unless explicitly requested.
- Keep generated artifacts under `.dist/` and do not commit them.
- Preserve the existing Task-based workflow until a Mise workflow is designed.
