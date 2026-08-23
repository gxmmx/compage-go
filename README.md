# compage-go

Foundational Go packages for building applications. compage-go provides common
program infrastructure for logging, configuration, terminal output, host
integration, certificates, and operating-system services without imposing a
framework or runtime model.

## Info

This repository is a Go module targeting Go 1.26. The packages are independent
where practical and can be adopted individually.

## Packages

| Package | Purpose |
|---------|---------|
| [`account`](./account/) | Looks up and reconciles local operating-system accounts |
| [`certs`](./certs/) | Manages a product-local mTLS certificate authority, certificate bundles, CSRs, and persistence |
| [`config`](./config/) | Loads typed configuration from defaults, files, environment variables, and flags with source tracking and persistence |
| [`configpflag`](./configpflag/) | Adapts `pflag` flag sets to the `config` package |
| [`console`](./console/) | Provides shared console primitives |
| [`console/logger`](./console/logger/) | Provides structured JSON logging with stable program and unit identity |
| [`console/printer`](./console/printer/) | Provides human-facing, level-gated terminal output |
| [`errx`](./errx/) | Provides transport-neutral semantic error classification |
| [`host`](./host/) | Reports read-only facts about the current host and network |
| [`service`](./service/) | Manages a program's operating-system service |
| [`style`](./style/) | Applies ANSI terminal styling when appropriate |
| [`term`](./term/) | Detects terminal-backed streams |
| [`text`](./text/) | Provides utilities for sanitizing and manipulating text |

## Usage

Add the module as a dependency with:

```bash
go get github.com/gxmmx/compage-go
```

Import the package that provides the capability your application needs. Each
package contains a `doc.go` file with package-level API information. Packages
with additional usage guidance have their own README:

- [`account/README.md`](./account/README.md)
- [`certs/README.md`](./certs/README.md)
- [`config/README.md`](./config/README.md)
- [`configpflag/README.md`](./configpflag/README.md)
- [`console/README.md`](./console/README.md)
- [`console/logger/README.md`](./console/logger/README.md)
- [`console/printer/README.md`](./console/printer/README.md)
- [`errx/README.md`](./errx/README.md)
- [`host/README.md`](./host/README.md)
- [`style/README.md`](./style/README.md)
- [`text/README.md`](./text/README.md)

## Development

### Setup

Install [Mise](https://mise.jdx.dev/), then install the pinned project tools:

```bash
mise install
```

This installs Go 1.26 and golangci-lint 2.13.0.

### Workflow

Use `mise tasks` to discover available tasks. The current verification entry
point is:

```bash
mise run check
```

Run all tests with `mise run test`. Pass a repository-relative directory after
`--` to target one package, such as `mise run test -- config` or
`mise run test:race -- config`.

### Testing

`mise run check` verifies formatting, tidy module metadata, golangci-lint,
uncached unit tests, race tests, and the config fuzz smoke tests. Coverage
output is written under `.dist/coverage/` by
`mise run test:coverage:config`.

Reusable test and lint entry points live under [`tests/`](./tests/): unit tests
are handled by [`tests/unit/run.sh`](./tests/unit/run.sh), and linting uses the
configuration in [`tests/lint/.golangci.yml`](./tests/lint/.golangci.yml).

## Design documents

Current implementation plans are kept under [`.design/`](./.design/):

- [`configv2-plan.md`](./.design/configv2-plan.md)
- [`ca-implementation-plan.md`](./.design/ca-implementation-plan.md)
- [`interactive-prompter-plan.md`](./.design/interactive-prompter-plan.md)

## License

MIT
