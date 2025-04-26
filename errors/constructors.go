package errors

// -----------------------------------------------------------------------------
// New Constructor
// -----------------------------------------------------------------------------

func New(kind Kind, msg string, err error) *AppError {
	appErr := &AppError{
		Kind:    kind,
		Message: msg,
		Fields:  make(map[string]any),
	}

	if err == nil {
		return appErr
	}

	switch e := err.(type) {
	case *AppError:
		// Preserve the inner AppError
		appErr.Err = e
		// Merge fields (outer overrides inner if same key)
		for k, v := range e.AllFields() {
			if _, exists := appErr.Fields[k]; !exists {
				appErr.Fields[k] = v
			}
		}
	default:
		// Regular error
		appErr.Err = e
	}

	return appErr
}

// -----------------------------------------------------------------------------
// Convenience constructors
// -----------------------------------------------------------------------------

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
