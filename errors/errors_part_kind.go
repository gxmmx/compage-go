package errors

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type Kind string

// -----------------------------------------------------------------------------
// Constants & Variables
// -----------------------------------------------------------------------------

const (
	KindAlreadyExists      Kind = "AlreadyExists"
	KindNotFound           Kind = "NotFound"
	KindInvalidInput       Kind = "InvalidInput"
	KindValidationFailed   Kind = "ValidationFailed"
	KindUnauthorized       Kind = "Unauthorized"
	KindForbidden          Kind = "Forbidden"
	KindConflict           Kind = "Conflict"
	KindTimeout            Kind = "Timeout"
	KindUnavailable        Kind = "Unavailable"
	KindMethodNotAllowed   Kind = "MethodNotAllowed"
	KindCancelled          Kind = "Cancelled"
	KindNotImplemented     Kind = "NotImplemented"
	KindPreconditionFailed Kind = "PreconditionFailed"
	KindRateLimited        Kind = "RateLimited"
	KindBadGateway         Kind = "BadGateway"
	KindDependencyFailed   Kind = "DependencyFailed"
	KindCorruptedData      Kind = "CorruptedData"
	KindNotSupported       Kind = "NotSupported"
	KindAssertionFailed    Kind = "AssertionFailed"
	KindInternal           Kind = "Internal"
)

var allKinds = []Kind{
	KindAlreadyExists,
	KindNotFound,
	KindInvalidInput,
	KindValidationFailed,
	KindUnauthorized,
	KindForbidden,
	KindConflict,
	KindTimeout,
	KindUnavailable,
	KindMethodNotAllowed,
	KindCancelled,
	KindNotImplemented,
	KindPreconditionFailed,
	KindRateLimited,
	KindBadGateway,
	KindDependencyFailed,
	KindCorruptedData,
	KindNotSupported,
	KindAssertionFailed,
	KindInternal,
}

// -----------------------------------------------------------------------------
// Convenience Constructors
// -----------------------------------------------------------------------------

func NewAlreadyExists(message string, err error) ApplicationError {
	return New(KindAlreadyExists, string(KindAlreadyExists), message, err)
}

func NewNotFound(message string, err error) ApplicationError {
	return New(KindNotFound, string(KindNotFound), message, err)
}

func NewInvalidInput(message string, err error) ApplicationError {
	return New(KindInvalidInput, string(KindInvalidInput), message, err)
}

func NewValidationFailed(message string, err error) ApplicationError {
	return New(KindValidationFailed, string(KindValidationFailed), message, err)
}

func NewUnauthorized(message string, err error) ApplicationError {
	return New(KindUnauthorized, string(KindUnauthorized), message, err)
}

func NewForbidden(message string, err error) ApplicationError {
	return New(KindForbidden, string(KindForbidden), message, err)
}

func NewConflict(message string, err error) ApplicationError {
	return New(KindConflict, string(KindConflict), message, err)
}

func NewTimeout(message string, err error) ApplicationError {
	return New(KindTimeout, string(KindTimeout), message, err)
}

func NewUnavailable(message string, err error) ApplicationError {
	return New(KindUnavailable, string(KindUnavailable), message, err)
}

func NewMethodNotAllowed(message string, err error) ApplicationError {
	return New(KindMethodNotAllowed, string(KindMethodNotAllowed), message, err)
}

func NewCancelled(message string, err error) ApplicationError {
	return New(KindCancelled, string(KindCancelled), message, err)
}

func NewNotImplemented(message string, err error) ApplicationError {
	return New(KindNotImplemented, string(KindNotImplemented), message, err)
}

func NewPreconditionFailed(message string, err error) ApplicationError {
	return New(KindPreconditionFailed, string(KindPreconditionFailed), message, err)
}

func NewRateLimited(message string, err error) ApplicationError {
	return New(KindRateLimited, string(KindRateLimited), message, err)
}

func NewBadGateway(message string, err error) ApplicationError {
	return New(KindBadGateway, string(KindBadGateway), message, err)
}

func NewDependencyFailed(message string, err error) ApplicationError {
	return New(KindDependencyFailed, string(KindDependencyFailed), message, err)
}

func NewCorruptedData(message string, err error) ApplicationError {
	return New(KindCorruptedData, string(KindCorruptedData), message, err)
}

func NewNotSupported(message string, err error) ApplicationError {
	return New(KindNotSupported, string(KindNotSupported), message, err)
}

func NewAssertionFailed(message string, err error) ApplicationError {
	return New(KindAssertionFailed, string(KindAssertionFailed), message, err)
}

func NewInternal(message string, err error) ApplicationError {
	return New(KindInternal, string(KindInternal), message, err)
}
