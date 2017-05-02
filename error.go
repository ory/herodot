package herodot

import (
	"net/http"

	"github.com/pkg/errors"
)

// StatusCodeCarrier can be implemented by an error to support setting status codes in the error itself.
type StatusCodeCarrier interface {
	// StatusCode returns the status code of this error.
	StatusCode() int
}

// ErrorContextCarrier can be implemented by an error to support error contexts.
type ErrorContextCarrier interface {
	error
	StatusCodeCarrier

	// RequestID returns the ID of the request that caused the error, if applicable.
	RequestID() string

	// Reason returns the reason for the error, if applicable.
	Reason() string

	// ID returns the error id, if applicable.
	Status() string

	// Details returns details on the error, if applicable.
	Details() []map[string]interface{}
}

type richError struct {
	CodeField    int                      `json:"code,omitempty"`
	StatusField  string                   `json:"status,omitempty"`
	RIDField     string                   `json:"request,omitempty"`
	ReasonField  string                   `json:"reason,omitempty"`
	DetailsField []map[string]interface{} `json:"details,omitempty"`
	ErrorField   string                   `json:"message"`
}

func (e *richError) Status() string {
	return e.StatusField
}

func (e *richError) Error() string {
	return e.ErrorField
}

func (e *richError) RequestID() string {
	return e.RIDField
}

func (e *richError) Reason() string {
	return e.ReasonField
}

func (e *richError) Details() []map[string]interface{} {
	return e.DetailsField
}

func (e *richError) StatusCode() int {
	return e.CodeField
}

func (e *richError) setFallbackRequestID(request string) {
	if e.RIDField != "" {
		return
	}

	e.RIDField = request
}

func assertRichError(err error) *richError {
	if e, ok := errors.Cause(err).(ErrorContextCarrier); ok {
		return &richError{
			CodeField:    e.StatusCode(),
			ReasonField:  e.Reason(),
			RIDField:     e.RequestID(),
			ErrorField:   err.Error(),
			DetailsField: e.Details(),
			StatusField:  e.Status(),
		}
	} else if e, ok := err.(ErrorContextCarrier); ok {
		return &richError{
			CodeField:    e.StatusCode(),
			ReasonField:  e.Reason(),
			RIDField:     e.RequestID(),
			ErrorField:   err.Error(),
			DetailsField: e.Details(),
			StatusField:  e.Status(),
		}
	} else if e, ok := err.(StatusCodeCarrier); ok {
		return &richError{
			CodeField:    e.StatusCode(),
			ErrorField:   err.Error(),
			DetailsField: []map[string]interface{}{},
		}
	} else if e, ok := errors.Cause(err).(StatusCodeCarrier); ok {
		return &richError{
			CodeField:    e.StatusCode(),
			ErrorField:   err.Error(),
			DetailsField: []map[string]interface{}{},
		}
	}

	return &richError{
		ErrorField:   err.Error(),
		CodeField:    http.StatusInternalServerError,
		DetailsField: []map[string]interface{}{},
	}
}
