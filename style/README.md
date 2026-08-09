# style

Terminal text styling with separate color and modifier types. Stateless and
composable — colors and modifiers combine freely without conflating concepts.

## Quick start

```go
import "github.com/gxmmx/compage-go/style"

// One-off styling
fmt.Println(style.Apply("success", style.Green))
fmt.Println(style.Apply("warning", style.Yellow, style.Bold))
fmt.Println(style.Apply("subtle", style.NoColor, style.Dim))

// Reusable styles
var (
    heading = style.Build(style.Cyan, style.Bold, style.Underline)
    muted   = style.Build(style.NoColor, style.Dim)
)

fmt.Println(heading.Apply("Configuration"))
fmt.Println(muted.Apply("no changes detected"))
```

## Types

### Color

Foreground text colors. `NoColor` is the zero value — produces no color output.

```go
style.NoColor  // no foreground color (safe zero value)
style.Red
style.Green
style.Yellow
style.Blue
style.Magenta
style.Cyan
style.White
```

### Mod

Text style modifiers. Combine freely with each other and with a color.

```go
style.Bold       // increased intensity
style.Dim        // decreased intensity (faint)
style.Italic     // italic text
style.Underline  // underlined text
```

## API

### Apply

One-off convenience. First argument after text is always a `Color`, followed by
zero or more `Mod` values.

```go
style.Apply("text", style.Red)                    // red
style.Apply("text", style.Green, style.Bold)      // bold green
style.Apply("text", style.NoColor, style.Dim)     // dim, no color
style.Apply("text", style.Blue, style.Dim, style.Italic)  // dim italic blue
```

Returns text unchanged when `NoColor` is passed with no modifiers.

### Build

Creates a reusable `Style`. Use when the same combination is applied repeatedly.

```go
s := style.Build(style.Red, style.Bold)
s.Apply("error one")
s.Apply("error two")
```

### Enabled

Determines whether styled output should be active for a given writer. The caller
uses this to gate `Apply` calls — `Apply` itself is pure and always emits codes.

```go
active := style.Enabled(suppressFlag, os.Stdout)

if active {
    fmt.Println(style.Apply("colored", style.Green))
} else {
    fmt.Println("colored")
}
```

Priority: `NO_COLOR` env var (always wins) > explicit suppress > TTY auto-detect.

## Design

- **Stateless.** `Apply` and `Build` are pure functions. No global state, no
  init, no singletons.
- **Caller gates output.** `Apply` always produces ANSI codes. The caller checks
  `Enabled` once and decides whether to style. This keeps styling logic testable
  without environment manipulation.
- **Colors and modifiers are distinct types.** Prevents passing a modifier where
  a color is expected and vice versa. The compiler catches misuse.
- **Zero value is safe.** `NoColor` and uninitialized `Color` variables produce
  no output rather than defaulting to a visible color.
