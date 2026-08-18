# Prompter TTY validation

## Status

Implementation plan only. No code changes are described as completed here.

## Problem

`printer.NewPrompter` currently returns a usable-looking `Prompter` regardless
of whether standard input is interactive. `Prompt` then scans stdin and can
block on a pipe, consume unintended piped data, or silently return its fallback
at EOF. The package documentation tells every caller to perform its own
`term.IsTerminal(os.Stdin)` check. That is a leaky and easy-to-forget contract.

The prompter owns interactive input, so it must own this precondition too.

## Decision

Validate standard input when constructing the prompter and return a classified
error when it is unavailable:

```go
pr, err := printer.NewPrompter()
if err != nil {
    // choose flags, config, or another non-interactive path
    return err
}
name := pr.Prompt("Project name", "my-app")
```

The new signature is deliberately:

```go
func NewPrompter(opts ...Option) (Prompter, error)
```

This is a breaking change, but it is the only honest constructor contract. The
existing `Prompt(string, string) string` and `Continue(string) bool` methods
cannot report a failed terminal precondition. A constructor that still returns
only `Prompter` would either hide the failure or make later behavior ambiguous.

On a valid terminal, `Prompt` and `Continue` retain their current line-input
semantics and return values. This change only rejects a prompter that could
never work interactively.

## Failure contract

- Check the actual input stream selected by construction; by default this is
  `os.Stdin`.
- Use `term.IsTerminal` from this repository's `term` package. Callers pass no
  terminal metadata and perform no preliminary check.
- If the default input is not a terminal, return:

  ```go
  errx.New(
      "printer: interactive input unavailable: stdin is not a terminal; supply the required value with a flag or configuration",
      errx.WithKind(errx.Unavailable),
  )
  ```

  Exact wording can be tuned, but it must identify the failed capability and
  offer an actionable non-interactive alternative. `errx.Unavailable` is the
  appropriate existing classification: a required runtime capability is absent,
  rather than the user's value being invalid.
- The constructor must not write a warning, prompt, or partial prompt before
  returning this error. The application chooses the fallback and presentation.
- Keep the original system error as a cause only when there is one. A negative
  TTY check has no underlying error, so it should be a clean classified error.

Callers can handle the specific condition with `errx.IsKind(err,
errx.Unavailable)` or use the error text directly in a CLI error path.

## Input overrides and tests

There is currently no exported `WithInput`; `withReader` is an unexported test
hook. Do not turn arbitrary `io.Reader` injection into a public way to bypass
the capability check by accident.

- Keep an unexported reader override for deterministic unit tests.
- Add an unexported terminal-check override for those tests, defaulting to
  `term.IsTerminal`. Tests using `strings.Reader` or `bytes.Buffer` can then
  assert prompt formatting without pretending those values are terminals.
- If a public input override is later needed, accept an `*os.File` (or a small
  input-source interface that explicitly exposes terminal capability), validate
  it with `term.IsTerminal`, and document it as advanced embedding support.
  Do not accept an arbitrary `io.Reader` as a normal interactive input source.

This retains a narrow production contract while keeping tests independent of a
pseudo-terminal.

## Implementation steps

1. Refactor construction so options are evaluated once and both the printer and
   prompter share the resolved configuration. `NewPrompter` currently applies
   options once through `newPrinter` and a second time to find its input reader.
2. Resolve the input source, then run the terminal check before allocating or
   returning a `prompter`.
3. Add the `errx` and local `term` imports to the constructor path and return a
   clear `errx.Unavailable` error for a non-terminal default input.
4. Change the public constructor signature, package documentation, README, and
   all examples to handle the error. Remove language saying callers must check
   stdin themselves.
5. Update the existing prompter tests for the new result signature and add:
   - default non-terminal input fails with `errx.Unavailable` and writes nothing;
   - an injected terminal check permits the existing input/fallback cases;
   - the error contains an actionable indication to use flags/configuration;
   - a terminal input source constructs successfully (covered through the
     injected checker; no real terminal required in CI).
6. Run the repository formatter, unit tests, and documentation examples/build
   checks used by the project.

## Boundaries for this change

This plan intentionally does **not** introduce raw mode, key handling, redraw,
or output-terminal validation. Those belong to the interactive-prompt design in
`interactive-prompter-plan.md`.

Current prompts are line-oriented, so requiring an input TTY is the correct and
sufficient immediate safety guarantee. A future redraw-capable prompter must
also require an interactive output stream because it needs to control the
terminal on which it renders.
