package errors

type Kind string

const (
	KindAlreadyExists Kind = "AlreadyExists"
	KindNotFound      Kind = "NotFound"
	KindInvalidInput  Kind = "InvalidInput"
	KindUnauthorized  Kind = "Unauthorized"
	KindForbidden     Kind = "Forbidden"
	KindConflict      Kind = "Conflict"
	KindTimeout       Kind = "Timeout"
	KindUnavailable   Kind = "Unavailable"
	KindInternal      Kind = "Internal"
)

var (
	ErrAlreadyExists = &AppError{Kind: KindAlreadyExists}
	ErrNotFound      = &AppError{Kind: KindNotFound}
	ErrInvalidInput  = &AppError{Kind: KindInvalidInput}
	ErrUnauthorized  = &AppError{Kind: KindUnauthorized}
	ErrForbidden     = &AppError{Kind: KindForbidden}
	ErrConflict      = &AppError{Kind: KindConflict}
	ErrTimeout       = &AppError{Kind: KindTimeout}
	ErrUnavailable   = &AppError{Kind: KindUnavailable}
	ErrInternal      = &AppError{Kind: KindInternal}
)
