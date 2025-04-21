package errors

import "encoding/json"

type JSONError struct {
	Kind    Kind           `json:"kind"`
	Message string         `json:"message,omitempty"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func (e *AppError) ToJSON(debug bool) JSONError {
	msg := e.Message
	if e.Kind == KindInternal && !debug {
		msg = "Internal error"
	}

	return JSONError{
		Kind:    e.Kind,
		Message: msg,
		Fields:  e.Fields,
	}
}

func (e *AppError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.ToJSON(false))
}
