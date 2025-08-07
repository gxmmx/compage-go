package errors

import (
	"fmt"
	"runtime"
)

// -----------------------------------------------------------------------------
// Controllers
// -----------------------------------------------------------------------------

type Controller struct {
	kind     Kind
	class    string
	msg      string
	err      error
	fields   map[string]any
	caller   string
	exitcode int
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func New(kind Kind, class string, message string, err error) ApplicationError {
	_, file, line, ok := runtime.Caller(1)
	caller := "unknown:0"
	if ok {
		caller = fmt.Sprintf("%s:%d", file, line)
	}
	appErr := &Controller{
		kind:     kind,
		class:    class,
		msg:      message,
		err:      err,
		fields:   make(map[string]any),
		caller:   caller,
		exitcode: defaultExitCode,
	}
	if err == nil {
		return appErr
	}

	if e, ok := err.(ApplicationError); ok {
		appErr.err = e
		for k, v := range e.Fields() {
			if _, exists := appErr.fields[k]; !exists {
				appErr.fields[k] = v
			}
		}
	}

	return appErr
}

// -----------------------------------------------------------------------------
// Controller methods
// -----------------------------------------------------------------------------

func (ctl *Controller) Error() string {
	msg := string(ctl.kind)
	if ctl.class != "" {
		msg += " (" + ctl.class + ")"
	}
	if ctl.msg != "" {
		msg += ": " + ctl.msg
	}
	return msg
}

func (ctl *Controller) Chain() string {
	msg := string(ctl.kind)
	if ctl.class != "" {
		msg += " (" + ctl.class + ")"
	}
	if ctl.msg != "" {
		msg += ": " + ctl.msg
	}

	err := ctl.err
	depth := 0
	for err != nil && depth < maxChainDepth {
		msg += " " + chainSeparator + " " + err.Error()

		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = unwrapper.Unwrap()
		depth++
	}

	if depth >= maxChainDepth && err != nil {
		msg += ": ... (max depth reached)"
	}

	return msg
}

func (ctl *Controller) Caller() string {
	return ctl.caller
}

func (ctl *Controller) ExitCode() int {
	return ctl.exitcode
}

func (ctl *Controller) Fields() map[string]any {
	merged := make(map[string]any)
	current := ctl
	for current != nil {
		for k, v := range current.fields {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
		if next, ok := current.err.(*Controller); ok {
			current = next
		} else {
			current = nil
		}
	}
	return merged
}

func (ctl *Controller) Unwrap() error {
	return ctl.err
}

func (ctl *Controller) Is(target error) bool {
	te, ok := target.(*Controller)
	return ok && ctl.kind == te.kind && ctl.class == te.class
}

func (ctl *Controller) IsKind(kind Kind) bool {
	return ctl.kind == kind
}

func (ctl *Controller) IsClass(class string) bool {
	return ctl.class == class
}

func (ctl *Controller) WithClass(class string) ApplicationError {
	ctl.class = class
	return ctl
}

func (ctl *Controller) WithField(key string, value any) ApplicationError {
	ctl.fields[key] = value
	return ctl
}

func (ctl *Controller) WithFields(fields map[string]any) ApplicationError {
	for k, v := range fields {
		ctl.fields[k] = v
	}
	return ctl
}

func (ctl *Controller) WithExitCode(c int) ApplicationError {
	ctl.exitcode = c
	return ctl
}
