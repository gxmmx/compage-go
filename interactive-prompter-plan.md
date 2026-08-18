# Interactive prompter design

## Status

Discussion and implementation map only. No interactive prompt implementation
exists yet.

## Goal

Evolve the current line-oriented `Prompt` / `Continue` helper into a compact,
inline interaction layer that complements the existing printer:

- editable text input with a dim ghost default;
- Tab accepting that default into the editable buffer;
- single choice and multi-choice selection;
- arrow-key navigation, Space toggling for multi-select, and Enter confirmation;
- redraw while interacting, followed by a clean, permanent printer-style summary.

This should borrow Bubble Tea's state-update-render approach without turning
compage into a general purpose full-screen TUI framework.

## User experience model

### Text question

```text
Project name: my-app       # `my-app` is dim and is not yet input text
```

- Typing any printable rune replaces the ghost default and redraws.
- Tab copies the default into the editable buffer; it can then be edited.
- Enter accepts the default while it is still ghost, otherwise accepts the
  buffer.
- Escape and Ctrl-C cancel and return a typed error in the new API.

### Multi-select question

```text
Select services
› [x] API
  [ ] Worker
  [x] Scheduler

↑/↓ move • space toggle • enter confirm • esc cancel
```

Enter confirms the current set; it never toggles an item. On confirmation the
temporary block is removed and the printer emits a stable result, for example:

```text
✓ Selected services: API, Scheduler
```

This keeps the user's scrollback a useful transcript instead of leaving stale
menus behind.

## What Bubble Tea contributes conceptually

Bubble Tea receives a key event, applies it to a small model, then renders the
model again. Its basic multi-select tutorial uses a cursor index and a set of
selected indexes, deriving cursor and checkbox markers in the view. That is the
exact level of state needed here.

Bubble Tea itself also owns raw-mode lifecycle, input parsing, signal handling,
resize messages, an event loop, and a renderer; Bubbles adds list filtering,
pagination, generated help, and more. Those are valuable for full TUIs but are
substantially broader than a sequential CLI question. The initial compage
implementation should use only the small state-machine pattern.

References: [Bubble Tea tutorial](https://github.com/charmbracelet/bubbletea),
[Bubble Tea event loop](https://github.com/charmbracelet/bubbletea/blob/main/tea.go),
and [Bubbles list scope](https://github.com/charmbracelet/bubbles).

## Technical model

```text
question config → state model → render frame
                         ▲          │
raw key input → decode ───┘          └→ erase prior owned frame + write next frame
```

### Terminal session

- Start only after the prompter has verified that **both** its input and its
  interactive output stream are terminals. The immediate TTY-fix plan validates
  input; redraw adds the output requirement.
- Use the existing `golang.org/x/term` dependency's `MakeRaw` and `Restore` on
  the input file descriptor.
- Always restore terminal state with a defer, including on input error,
  cancellation, and panic. Ensure Ctrl-C is handled deliberately in raw mode.
- Decode printable runes, Enter, Tab, Backspace/Delete, Escape/Ctrl-C, and the
  common ANSI sequences for Up/Down. Isolate this decoder behind a byte-source
  interface so it can be unit-tested without a terminal.

### Rendering

- Render only the prompt's own lines in the normal screen buffer; do not use an
  alternate screen for sequential questions.
- Track the number of physical lines in the last frame. Before every redraw,
  move to its first line, erase each owned line, and write the new frame.
- Use ANSI-aware display width and wrapping before moving by line count. Plain
  byte length is wrong for styles, Unicode, and narrow terminals.
- On confirm or cancel, clear the active frame. On confirm, use printer output
  for the settled summary; on cancel, leave a predictable cancelled/error path.
- Start with a fixed, non-wrapping list height. Add resize handling and scrolling
  only when a real caller needs long choice lists.

### State models

- `textState`: message, default, ghost/default-active flag, editable rune
  buffer, cursor position, completion/cancel status.
- `selectState`: message, choices, cursor, selected set keyed by stable choice
  identity, optional disabled choices, status.
- Rendering is a pure function of state. Key handling updates state and returns
  either another frame, a result, or a cancellation/error.

Use stable choice values rather than indexes in public results so callers can
reorder display choices without changing meaning.

## Proposed API direction

Keep existing methods as compatibility helpers while adding error-returning
interactive APIs. Raw terminal setup and user cancellation are genuine failure
paths and must not be flattened into `""` or `false`.

```go
type Choice struct {
    Value       string
    Label       string
    Description string
    Disabled    bool
}

type TextQuestion struct {
    Message string
    Default string
}

func (p *Prompter) Ask(ctx context.Context, q TextQuestion) (string, error)
func (p *Prompter) Select(ctx context.Context, message string, choices []Choice) (Choice, error)
func (p *Prompter) MultiSelect(ctx context.Context, message string, choices []Choice) ([]Choice, error)
```

Exact exported names and whether `Prompter` remains an interface should be
settled before implementation. The important contracts are explicit result
types, stable values, context-aware cancellation, and errors for unavailable
terminal capability / aborted interaction / I/O failure.

`Prompt` may internally use the text engine once it is reliable, retaining its
fallback behavior. `Continue` may later use a compact yes/no selector while
retaining its boolean result. Neither compatibility method can expose
cancellation errors, so they should not be the primary API for new code.

## Package boundary options

### Settled package location: `console/term`

Move the current top-level `term` package to `console/term` as part of the
interactive-prompt work (or as a small preparatory change). It currently owns a
single console-specific capability helper, and terminal detection is a concern
shared by printer and prompt rather than by the rest of the module.

The resulting console area is intentionally cohesive:

```text
console/
  logger/    # structured logging for console applications
  printer/   # semantic terminal output and presentation
  prompt/    # interactive input and question state (future)
  term/      # terminal capability and terminal-session helpers
```

Existing imports of `github.com/gxmmx/compage-go/term` will need a mechanical
migration to `github.com/gxmmx/compage-go/console/term`; update package docs,
READMEs, and all internal imports together. Keep `console/term` small and
focused on reusable terminal primitives, not prompt-specific state or rendering.

This location decision is settled. It does not decide the public interactive
API, whether compatibility constructors remain in `printer`, or the exact
printer/prompt adapter shape.

### Recommended: sibling `console/prompt`

```text
console/
  printer/   # semantic output, styles, tables, slog handler
  prompt/    # terminal session, input decoder, state, rendering
  term/      # terminal capability and session helpers
```

`prompt` owns interactive input and depends on a small printer-owned interactive
capability for presentation. `prompt` may import its `printer` sibling; the
dependency is one-way and `printer` must not import `prompt`. The capability
must preserve the chosen printer's stream routing, color policy, write lock, and
indentation for both temporary redraw frames and final semantic output. The
current `Printer` interface is intentionally too small for that purpose, so its
exact companion interface and construction path remain to be designed.

`prompt` should not attempt to implement `printer.Printer`; input interaction is
a separate responsibility.

This leaves the printer focused and avoids making prompt internals dominate its
source directory as raw-mode, rendering, and tests grow.

### Alternative: `console/printer/prompt`

The namespace is intuitive, but it becomes awkward if `printer.NewPrompter`
must remain a compatibility facade: a child package importing `printer` and a
parent importing the child form an import cycle. It only works cleanly if the
subpackage owns an independent API and the parent does not wrap it.

### Alternative: retain `console/printer`

This is acceptable for the immediate TTY safety fix. It becomes less attractive
once there are separate terminal session, key decoder, renderer, question, and
test files. It is a reasonable transitional location, not the desired final
home for a substantial interactive subsystem.

## Delivery sequence

1. Complete `prompter-tty-fix.md` so construction is safe and errors are
   handleable.
2. Decide the final package boundary and output adapter before exposing a new
   API.
3. Build a private terminal-session and key-decoder layer with byte-stream tests.
4. Build text input with ghost default and Tab materialization; test frame output
   and result state independently of an actual terminal.
5. Add single-select and multi-select with default selections, disabled items,
   confirmation, and final printer summaries.
6. Add resizing, scrolling, filtering, mouse support, or async updates only in
   response to concrete product requirements.

## Non-goals for the first version

- Full-screen/alternate-screen UI.
- A generic Bubble Tea-compatible model/runtime or a Bubble Tea dependency.
- Mouse handling, fuzzy filtering, arbitrary nested forms, async task views, or
  a component marketplace.
- Prompting into pipes or silently falling back when interaction is unavailable.
