package console

// Hint keys form the slog-attribute contract shared between the logger and
// printer subpackages. They are namespaced under HintPrefix so the logger can
// strip them from JSON output while the printer consumes them as rendering
// directives (indentation, success styling). Callers do not normally set these
// directly; they exist here as the single source of truth for both sides.
const (
	// HintPrefix namespaces all console hint attributes.
	HintPrefix = "console."

	// HintIndent carries an integer indent delta for printer output.
	HintIndent = "console.indent"

	// HintSuccess marks an info-level record as a success for the printer.
	HintSuccess = "console.success"
)
