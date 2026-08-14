package errx

// Kind is a transport-neutral semantic classification for an error.
type Kind uint8

const (
	// Unknown means that no single semantic classification is available.
	Unknown Kind = iota
	// Invalid means input could not be decoded, parsed, or interpreted.
	Invalid
	// Validation means understood input violates a domain rule.
	Validation
	// Unauthenticated means valid authentication is missing or failed.
	Unauthenticated
	// Forbidden means the caller is known but not permitted to act.
	Forbidden
	// NotFound means the requested domain resource does not exist.
	NotFound
	// Conflict means the operation conflicts with current state or invariants.
	Conflict
	// RateLimited means work was rejected because an enforced rate was exceeded.
	RateLimited
	// Unavailable means a required service or resource is temporarily unavailable.
	Unavailable
	// Timeout means the operation exceeded its deadline.
	Timeout
	// Internal means an explicitly classified unexpected implementation or system failure.
	Internal
)

// Valid reports whether k is a declared Kind value.
func (k Kind) Valid() bool {
	return k <= Internal
}

// String returns the stable lowercase name of k. Unknown numeric values are
// rendered as "unknown".
func (k Kind) String() string {
	switch k {
	case Unknown:
		return "unknown"
	case Invalid:
		return "invalid"
	case Validation:
		return "validation"
	case Unauthenticated:
		return "unauthenticated"
	case Forbidden:
		return "forbidden"
	case NotFound:
		return "not_found"
	case Conflict:
		return "conflict"
	case RateLimited:
		return "rate_limited"
	case Unavailable:
		return "unavailable"
	case Timeout:
		return "timeout"
	case Internal:
		return "internal"
	default:
		return "unknown"
	}
}

// Classified is implemented by errors with a semantic Kind. Implementations
// must return a declared Kind value.
type Classified interface {
	error
	Kind() Kind
}
