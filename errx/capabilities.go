package errx

// Retryable is implemented by errors that can state whether retrying is
// appropriate for the specific failed operation.
type Retryable interface {
	error
	Retryable() bool
}

// PublicMessenger is implemented by errors with a message safe for untrusted
// clients. It must not default to raw cause text.
type PublicMessenger interface {
	error
	PublicMessage() string
}

// Fieldser is implemented by errors that expose structured diagnostic fields.
// Implementations must not return mutable internal state.
type Fieldser interface {
	error
	Fields() map[string]any
}
