package herodot

import (
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"

	"github.com/ory/x/errorsx"
)

type DefaultError struct {
	CodeField    int                      `json:"code,omitempty"`
	StatusField  string                   `json:"status,omitempty"`
	RIDField     string                   `json:"request,omitempty"`
	ReasonField  string                   `json:"reason,omitempty"`
	DebugField   string                   `json:"debug,omitempty"`
	DetailsField map[string][]interface{} `json:"details,omitempty"`
	ErrorField   string                   `json:"message"`

	trace errors.StackTrace
}

// StackTrace returns the error's stack trace.
func (e *DefaultError) StackTrace() errors.StackTrace {
	return e.trace
}

func (e *DefaultError) WithTrace(err error) *DefaultError {
	out := *e
	if t, ok := err.(stackTracer); ok {
		out.trace = t.StackTrace()
	} else {
		out.trace = errors.New("").(stackTracer).StackTrace()
	}

	return &out
}

func (e *DefaultError) Status() string {
	return e.StatusField
}

func (e *DefaultError) Error() string {
	return e.ErrorField
}

func (e *DefaultError) RequestID() string {
	return e.RIDField
}

func (e *DefaultError) Reason() string {
	return e.ReasonField
}

func (e *DefaultError) Debug() string {
	return e.DebugField
}

func (e *DefaultError) Details() map[string][]interface{} {
	return e.DetailsField
}

func (e *DefaultError) StatusCode() int {
	return e.CodeField
}

func (e *DefaultError) WithReason(reason string) *DefaultError {
	err := *e
	err.ReasonField = reason
	return &err
}

func (e *DefaultError) WithReasonf(reason string, args ...interface{}) *DefaultError {
	return e.WithReason(fmt.Sprintf(reason, args...))
}

func (e *DefaultError) WithError(message string) *DefaultError {
	err := *e
	err.ErrorField = message
	return &err
}

func (e *DefaultError) WithErrorf(message string, args ...interface{}) *DefaultError {
	return e.WithError(fmt.Sprintf(message, args...))
}

func (e *DefaultError) WithDebugf(debug string, args ...interface{}) *DefaultError {
	return e.WithDebug(fmt.Sprintf(debug, args...))
}

func (e *DefaultError) WithDebug(debug string) *DefaultError {
	err := *e
	err.DebugField = debug
	return &err
}

func (e *DefaultError) WithDetail(key string, value ...interface{}) *DefaultError {
	err := *e
	if err.DetailsField == nil {
		err.DetailsField = map[string][]interface{}{}
	}
	err.DetailsField[key] = append(err.DetailsField[key], value...)
	return &err
}

func (e *DefaultError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "error=%s\n", e.ErrorField)
			fmt.Fprintf(s, "reason=%s\n", e.ReasonField)
			fmt.Fprintf(s, "details=%+v\n", e.DetailsField)
			fmt.Fprintf(s, "debug=%s\n", e.DebugField)
			e.StackTrace().Format(s, verb)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, e.ErrorField)
	case 'q':
		fmt.Fprintf(s, "%q", e.ErrorField)
	}
}

func ToDefaultError(err error, id string) *DefaultError {
	var trace []errors.Frame
	var reason, status, debug string

	if e, ok := err.(stackTracer); ok {
		trace = e.StackTrace()
	}

	statusCode := http.StatusInternalServerError
	details := map[string][]interface{}{}
	rid := id

	if e, ok := errorsx.Cause(err).(statusCodeCarrier); ok {
		statusCode = e.StatusCode()
	}

	if e, ok := errorsx.Cause(err).(reasonCarrier); ok {
		reason = e.Reason()
	}

	if e, ok := errorsx.Cause(err).(requestIDCarrier); ok && e.RequestID() != "" {
		rid = e.RequestID()
	}

	if e, ok := errorsx.Cause(err).(detailsCarrier); ok && e.Details() != nil {
		details = e.Details()
	}

	if e, ok := errorsx.Cause(err).(statusCarrier); ok {
		status = e.Status()
	}

	if e, ok := errorsx.Cause(err).(debugCarrier); ok {
		debug = e.Debug()
	}

	return &DefaultError{
		CodeField:    statusCode,
		ReasonField:  reason,
		RIDField:     rid,
		ErrorField:   err.Error(),
		DetailsField: details,
		StatusField:  status,
		DebugField:   debug,
		trace:        trace,
	}
}
