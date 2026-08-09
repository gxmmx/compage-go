# style

Terminal text styling with init-time gating. Create a `Styler` once per output
destination — it checks TTY, `NO_COLOR`, and a suppress flag at creation. All
subsequent calls apply codes or return text unchanged without per-call checks.

## Usage

```go
import "github.com/gxmmx/compage-go/style"

// Create a Styler for your output stream
s := style.New(noColorFlag, os.Stdout)

// One-off styling
fmt.Println(s.Apply("deployed", style.Green))
fmt.Println(s.Apply("deprecated", style.Yellow, style.Bold))
fmt.Println(s.Apply("subtle", style.NoColor, style.Dim))

// Reusable styles — define once, apply many times
success := s.Build(style.Green)
warn := s.Build(style.Yellow, style.Bold)
heading := s.Build(style.BrightCyan, style.Bold, style.Underline)

fmt.Println(success.Apply("All checks passed"))
fmt.Println(warn.Apply("Endpoint deprecated"))
fmt.Println(heading.Apply("Results"))
```

## Styler construction

```go
// Normal: checks TTY + NO_COLOR + suppress
s := style.New(false, os.Stdout)

// Suppressed (e.g. --no-color flag)
s := style.New(true, os.Stdout)

// Forced active (testing, or codes are unconditionally needed)
s := style.New(false, nil)
```

Priority: `NO_COLOR` env (always wins) > suppress flag > TTY detection on writer.

Zero value `Styler{}` is inactive — safe default if forgotten.

## Colors

```go
style.NoColor        // no color (zero value, safe no-op)
style.Red
style.Green
style.Yellow
style.Blue
style.Magenta
style.Cyan
style.White
style.BrightRed
style.BrightGreen
style.BrightYellow
style.BrightBlue
style.BrightMagenta
style.BrightCyan
style.BrightWhite
```

## Modifiers

```go
style.Bold           // increased intensity
style.Dim            // decreased intensity
style.Italic         // italic text
style.Underline      // underlined text
```

Modifiers combine freely with each other and with a color:

```go
s.Apply("text", style.Red, style.Bold, style.Underline)
s.Apply("text", style.NoColor, style.Dim, style.Italic)
```

## Raw

For edge cases where you need ANSI codes without a Styler (e.g. passing styled
strings to external tooling):

```go
styled := style.Raw("error", style.Red, style.Bold)
```

`Raw` always wraps — no gating. Not the recommended path for normal usage.
