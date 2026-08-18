# Interactive prompter implementation plan

## Status

This document is the agreed implementation plan. It deliberately permits
destructive refactoring: there are no external compatibility commitments at
this point.

## Product contract

`console/prompt` is an opinionated, sequential CLI interaction layer. A caller
asks a question and receives the typed value; the prompter itself owns the
temporary UI and the permanent transcript. Callers do not print answers.

Every active question renders its input on a line of its own:

```text
Provide foo:
(dim) Multiple values are allowed
> value

```

On success, its active frame is replaced by a settled transcript:

```text
Foo:
  value

```

`Summary` is optional. When absent, the original question message is used.
All settled questions have a summary line, indented answer line(s), and one
trailing blank line. An empty accepted answer is displayed as `(empty)`;
an allowed empty collection as `(none)`.

The caller owns spacing before a prompt or block. Prompts own their trailing
blank line, so adjacent prompts stack cleanly without layout jumps.

## Console package structure

```text
console/
  logger/    # structured application logging
  printer/   # permanent semantic human output
  prompt/    # question API, state machines, rendering, decoding
  term/      # TTY/session/cursor/size primitives
validate/    # shared value-validator contract and small reusable validators
```

Move the current top-level `term` implementation to `console/term` and update
all repository imports. Do not retain a top-level compatibility package.

Remove `printer.Prompter`, `printer.NewPrompter`, `Prompt`, and `Continue`.
They are line-oriented APIs with no error channel and are replaced by
`console/prompt`.

`prompt.New(printer.Printer)` is the public construction path. It inherits the
chosen printer's normal-output stream, style/color policy, write lock, and
indent. Internally, `printer` exposes a narrow terminal-interaction capability
backed by its concrete printer; `prompt` consumes that capability. `printer`
does not import `prompt`, avoiding an import cycle.

At construction, the prompter must reject the setup with `errx.Unavailable`
unless all of the following hold:

- stdin is a terminal;
- the selected printer normal-output stream is a terminal (stdout by default,
  or stderr when routed there);
- normal printer output is visible at Info level (`Warn` and above is not a
  valid prompt environment);
- the terminal supports the cursor-position capability required for safe
  inline redraw.

This is intentionally not a pipe fallback. Callers must obtain values through
flags, environment, configuration, or another non-interactive path instead.

## Public API direction

The following is the implementation surface:

```go
type Question struct {
    Message     string        // required
    Summary     string        // optional; defaults to Message
    Description string        // optional dim secondary line
    Validate    validate.Func // optional; receives the coerced final value
}

type StrQuestion struct {
    Question
    Default  *string
    Required bool // false accepts empty input
    Secret   bool
}

type IntQuestion struct { Question; Default *int }
type StrsQuestion struct { Question; Default *[]string }
type IntsQuestion struct { Question; Default *[]int }
type DurQuestion struct { Question; Default *time.Duration }
type DateQuestion struct { Question; Default *time.Time }

type MultiStrsQuestion struct { Question }
type MultiIntsQuestion struct { Question }

type Choice struct {
    Value       string
    Label       string
    Description string
    Disabled    bool
    Locked      bool // SelectMulti only
}

type SelectQuestion struct {
    Question
    Choices      []Choice
    DefaultValue string
}

type SelectMultiQuestion struct {
    Question
    Choices       []Choice
    InitialValues []string
    AllowEmpty    bool // false by default
}

func New(out printer.Printer) (*Prompter, error)

func (p *Prompter) Ask(ctx context.Context, q StrQuestion) (string, error)
func (p *Prompter) AskStr(ctx context.Context, q StrQuestion) (string, error)
func (p *Prompter) AskInt(ctx context.Context, q IntQuestion) (int, error)
func (p *Prompter) AskStrs(ctx context.Context, q StrsQuestion) ([]string, error)
func (p *Prompter) AskInts(ctx context.Context, q IntsQuestion) ([]int, error)
func (p *Prompter) AskDur(ctx context.Context, q DurQuestion) (time.Duration, error)
func (p *Prompter) AskDate(ctx context.Context, q DateQuestion) (time.Time, error)

func (p *Prompter) AskMulti(ctx context.Context, q MultiStrsQuestion) ([]string, error)
func (p *Prompter) AskMultiStrs(ctx context.Context, q MultiStrsQuestion) ([]string, error)
func (p *Prompter) AskMultiInts(ctx context.Context, q MultiIntsQuestion) ([]int, error)

func (p *Prompter) Select(ctx context.Context, q SelectQuestion) (Choice, error)
func (p *Prompter) SelectMulti(ctx context.Context, q SelectMultiQuestion) ([]Choice, error)
func (p *Prompter) Continue(ctx context.Context, q Question) (bool, error)

func (p *Prompter) Block(ctx context.Context, fn func(*Block) error) error
```

`Ask` aliases `AskStr`; `AskMulti` aliases `AskMultiStrs`. Typed defaults are
typed pointers so a valid zero value is distinct from no default. Typed
defaults are formatted canonically by the prompter.

`AskDate` returns `time.Time` normalized to midnight UTC. It accepts
`YYYY-MM-DD` through `time.Parse("2006-01-02", input)` and `YY-MM-DD` through
`time.Parse("06-01-02", input)`, including Go's standard two-digit-year
mapping. Date-time parsing belongs in a later, separate `AskTime` API.
`AskDur` uses `time.ParseDuration`. `AskStrs` uses the same
pflag-compatible CSV syntax as configuration string slices; `AskInts` applies
decimal integer coercion to each CSV element. The implementation may initially
share this behavior rather than move config parsing in this change.

Question definitions are validated before raw interaction starts. Required
messages, non-empty unique choice values, non-empty unique labels, no embedded
newlines/control sequences, valid defaults, and valid state combinations are
developer errors returned as `errx.Invalid`.

## Text input

For normal string questions, a default is shown dim after `>`. It is a ghost:
typing a printable rune replaces it, and Tab materializes it into the editable
buffer. Enter accepts the ghost default or the buffer. Once text is
materialized it is rendered normally.

Secret questions reject defaults and never offer Tab completion. They show no
live rune echo, following normal password-input convention. Their settled
answer is always exactly six asterisks (`******`), independent of actual input
length.

The initial text editor supports printable runes, Left/Right, Home/End,
Backspace, Delete, Tab where applicable, Enter, Escape, and Ctrl-C.

## Selection input

`Select` and `SelectMulti` show a cursor, checkbox/radio markers, and an
optional dim description below each choice label. A missing description means
the choice consumes one line.

- Arrow navigation stops at the first/last enabled item; it does not wrap.
- Disabled choices are dimmed, skipped by navigation, and cannot be selected
  or initially selected.
- `Locked` is valid only for `SelectMulti`: it is dimmed, preselected, and
  cannot be toggled off. It cannot be combined with `Disabled`.
- `Select` starts at `DefaultValue`, or the first enabled choice.
- `SelectMulti` starts with `InitialValues` plus every locked choice.
- Space toggles a multi-select item; Enter confirms, never toggles.
- Empty multi-select confirmation is rejected unless `AllowEmpty` is true.

Settled selections display labels, one per indented line. They never expose
internal `Choice.Value` identifiers.

`Continue` is a compact two-choice selector with No selected by default; Yes
is the other choice. It returns a boolean and retains the same settled-summary
rules as other prompts.

## Append-only multi-value input

`AskMultiStrs` and `AskMultiInts` collect arbitrary values one line at a time:

```text
Provide foos:
(dim) Multiple values are allowed
> first
> second
>
  Done
```

Enter commits a non-empty current value and opens the next entry. An empty
entry finishes the question. `Done` is always a visible final row that can be
selected with Down and Enter. Previously committed entries are append-only in
the first version; there are no initial entries, deletion, or editing of a
committed line. The final `[]string` or `[]int` is then validated as a whole.

## Validation and errors

Add root package `validate` in this work:

```go
package validate

type Func func(any) error

func NonEmpty(value any) error
```

It is intentionally independent of `config` for now. `config` keeps its
existing contextual validator API and can adapt to `validate.Func` in later
work. `NonEmpty` accepts strings and slices, rejects their empty values, and
returns an error for unsupported value types.

Each question has at most one validator. On Enter, the prompter first coerces
the input into the method's result type and then invokes that validator with
the final value (`Choice` for `Select`, `[]Choice` for `SelectMulti`, and the
completed slice for append-only input). Validators run for accepted defaults
and initial selections too.

Coercion failure and validator failure keep the question active. The original
description is temporarily replaced by the user-safe `error.Error()` text;
the next edit restores the original description. Text buffers and selection
state remain intact so the user can correct them. Validators are responsible
for returning human-readable, safe error messages.

Escape and Ctrl-C return package sentinel `prompt.ErrCanceled`. In a block,
they cancel the entire block. A canceled context returns `ctx.Err()` and must
dismiss an idle prompt immediately. The prompter never calls `os.Exit`; the
application decides whether to end, clean up, or reprompt.

## Blocks

`Block` groups related prompts. It holds the inherited printer's interaction
lock for the complete callback, so printer/log output from other goroutines
waits and cannot corrupt the active frame. It owns only its own lines and does
not clear, rewrite, or otherwise manipulate earlier terminal output.

The callback receives a constrained `*Block`, not a printer or writer. It may
ask the same question types as `Prompter`, derive indentation with
`WithIndent(n)`, and call `SetSummary(string)`. It cannot print arbitrary
content. This keeps rendering owned by the prompter while allowing conditional
later prompts based on answers returned by earlier calls.

Each accepted nested question immediately redraws the block's active frame as
a settled answer plus the next question. There is no block nesting. On normal
completion, the settled transcript remains. A non-empty `SetSummary` replaces
the complete block transcript at completion; an empty summary is a no-op. On
cancellation or any callback error, the entire owned block frame is cleared.

Callers should not invoke the original printer from inside a block callback;
the block deliberately owns the printer interaction lock. Separate ordinary
output and prompt sections into separate blocks.

## Terminal session and rendering

Start a raw-mode session only after construction validation. `console/term`
owns the small reusable terminal primitives:

- terminal-backed file detection;
- raw-mode `MakeRaw`/`Restore`, including restoration on every return and
  panic;
- terminal-size lookup;
- cursor-position capability query with a short bounded deadline;
- a testable byte-source/key-decoder boundary.

Raw decoding recognizes printable UTF-8 runes, Enter, Tab, Backspace, Delete,
Left/Right, Home/End, Up/Down, Space, Escape, Ctrl-C, and the common ANSI key
sequences. Escape-sequence ambiguity and cursor-report responses are handled
inside the decoder/session rather than in question state.

Rendering uses the ordinary terminal buffer, never an alternate screen. It
tracks only lines it owns: before a frame update it returns to the owned frame
origin, erases that frame, and writes the new frame. Confirm, cancellation,
and errors clear the temporary frame before either permanent transcript output
or return.

Frame measurement must be ANSI-aware and Unicode-width-aware. Terminal width,
wrapping, descriptions, and the selection/multi-value viewport all contribute
to physical line count. Long choice lists use a scrollable viewport sized to
the terminal; full-screen clearing is not permitted. Cursor-position support
lets the renderer reserve a safe inline frame without guessing about content
printed before the prompt. Construction also rejects a terminal too narrow or
too short to render the minimum prompt frame safely, using `errx.Unavailable`
rather than risking scrollback corruption.

The session always restores terminal state. If restoration fails, return that
failure (joined with any primary interaction failure where appropriate).

## Test plan

Unit-test without a real terminal:

- question-definition validation and unavailable construction cases;
- all coercion/default/required/secret behavior;
- CSV, integer, duration, and date parsing;
- validator replacement/restoration behavior and final values;
- text cursor editing and key decoder sequences;
- selection navigation, disabled/locked/default/empty states;
- append-only multi-value input and Done behavior;
- physical-frame rendering, ANSI/Unicode width, viewport, and stable
  summaries;
- block lock ownership, transcript replacement, cancellation clearing, and
  indentation;
- cancellation, context dismissal, I/O errors, and terminal restoration.

Add focused integration coverage using a pseudo-terminal for raw lifecycle,
cursor-position capability, scrolling viewport, and concurrent printer writes.

## Delivery order

1. Move `term` to `console/term`; update `style`, `printer`, docs, and tests.
2. Add `validate.Func` and `validate.NonEmpty` without changing `config`.
3. Refactor `printer` to remove the legacy prompter and expose its narrow
   interaction capability.
4. Build `prompt` construction/session/key decoding and terminal capability
   tests.
5. Implement typed single-value questions and their settled rendering.
6. Implement `Select`, `SelectMulti`, and `Continue` with viewport support.
7. Implement append-only multi-value questions.
8. Implement blocks, transcript collapse, locking, documentation, and
   pseudo-terminal integration tests.

## Out of scope

- Alternate-screen/full-screen TUI behavior or a Bubble Tea dependency.
- Mouse input, fuzzy filtering, generic forms, arbitrary writes inside blocks,
  nested blocks, and an editable multi-value list.
- `AskTime`, multi-date/duration input, config's validator migration, and
  top-level `term` compatibility.
