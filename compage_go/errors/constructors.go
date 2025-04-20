package errors

func New(kind Kind, msg string, err error) *AppError {
	return &AppError{
		Kind:    kind,
		Message: msg,
		Err:     err,
		Fields:  nil,
	}
}

// Convenience functions
func AlreadyExists(err error, msg string) *AppError {
	return New(KindAlreadyExists, msg, err)
}

func NotFound(err error, msg string) *AppError {
	return New(KindNotFound, msg, err)
}

func InvalidInput(err error, msg string) *AppError {
	return New(KindInvalidInput, msg, err)
}

func Unauthorized(err error, msg string) *AppError {
	return New(KindUnauthorized, msg, err)
}

func Forbidden(err error, msg string) *AppError {
	return New(KindForbidden, msg, err)
}

func Conflict(err error, msg string) *AppError {
	return New(KindConflict, msg, err)
}

func Timeout(err error, msg string) *AppError {
	return New(KindTimeout, msg, err)
}

func Unavailable(err error, msg string) *AppError {
	return New(KindUnavailable, msg, err)
}

func Internal(err error, msg string) *AppError {
	return New(KindInternal, msg, err)
}
