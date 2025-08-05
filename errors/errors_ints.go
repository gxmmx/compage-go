package errors

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type ApplicationError interface {
	// Returns error message with kind and class prefix
	Error() string
	// Returns the error message chain
	Chain() string
	// Returns the error caller file:line
	Caller() string
	// Returns a map of error fields
	Fields() map[string]any
	// Returns the next nested error
	Unwrap() error

	// Checks if the error is of a specific kind and class
	Is(target error) bool
	// Checks if the error is of a specific kind
	IsKind(kind Kind) bool
	// Checks if the error is of a specific class
	IsClass(class string) bool

	// Adds a class to the error
	WithClass(class string) ApplicationError
	// Adds a field to the error
	WithField(key string, value any) ApplicationError
	// Adds multiple fields to the error
	WithFields(fields map[string]any) ApplicationError

	// Returns the fields need to log as an slog message
	// Call example:
	// msg, args := ctl.Slog()
	// slog.Logger.Error(msg, args...)
	Slog() (string, []any)
}
