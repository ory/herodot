package herodot

import (
	stderr "errors"
	"fmt"
	"io"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pkg/errors"

	"github.com/ory/x/errorsx"
)

// swagger:model genericError
type DefaultError struct {
	// The status code
	//
	// example: 404
	CodeField int `json:"code,omitempty"`

	// The status description
	//
	// example: Not Found
	StatusField string `json:"status,omitempty"`

	// The request ID
	//
	// The request ID is often exposed internally in order to trace
	// errors across service architectures. This is often a UUID.
	//
	// example: d7ef54b1-ec15-46e6-bccb-524b82c035e6
	RIDField string `json:"request,omitempty"`

	// A human-readable reason for the error
	//
	// example: User with ID 1234 does not exist.
	ReasonField string `json:"reason,omitempty"`

	// Debug information
	//
	// This field is often not exposed to protect against leaking
	// sensitive information.
	//
	// example: SQL field "foo" is not a bool.
	DebugField string `json:"debug,omitempty"`

	// Further error details
	DetailsField map[string]interface{} `json:"details,omitempty"`

	// Error message
	//
	// The error's message.
	//
	// example: The resource could not be found
	// required: true
	ErrorField string `json:"message"`

	GRPCCodeField codes.Code `json:"-"`
	err           error
}

// StackTrace returns the error's stack trace.
func (e *DefaultError) StackTrace() (trace errors.StackTrace) {
	if e.err == e {
		return
	}

	if st := stackTracer(nil); stderr.As(e.err, &st) {
		trace = st.StackTrace()
	}

	return
}

func (e DefaultError) Unwrap() error {
	return e.err
}

func (e *DefaultError) Wrap(err error) {
	e.err = err
}

func (e DefaultError) WithWrap(err error) *DefaultError {
	e.err = err
	return &e
}

func (e *DefaultError) WithTrace(err error) *DefaultError {
	if st := stackTracer(nil); !stderr.As(e.err, &st) {
		e.Wrap(errors.WithStack(err))
	} else {
		e.Wrap(err)
	}
	return e
}

func (e DefaultError) Is(err error) bool {
	switch te := err.(type) {
	case DefaultError:
		return e.ErrorField == te.ErrorField &&
			e.StatusField == te.StatusField &&
			e.CodeField == te.CodeField
	case *DefaultError:
		return e.ErrorField == te.ErrorField &&
			e.StatusField == te.StatusField &&
			e.CodeField == te.CodeField
	default:
		return false
	}
}

func (e DefaultError) Status() string {
	return e.StatusField
}

func (e DefaultError) Error() string {
	return e.ErrorField
}

func (e DefaultError) RequestID() string {
	return e.RIDField
}

func (e DefaultError) Reason() string {
	return e.ReasonField
}

func (e DefaultError) Debug() string {
	return e.DebugField
}

func (e DefaultError) Details() map[string]interface{} {
	return e.DetailsField
}

func (e DefaultError) StatusCode() int {
	return e.CodeField
}

func (e DefaultError) GRPCStatus() *status.Status {
	s := status.New(e.GRPCCodeField, e.Error())

	st := e.StackTrace()
	var stackEntries []string
	if st != nil {
		stackEntries = make([]string, len(st))
		for i, f := range st {
			stackEntries[i] = fmt.Sprintf("%+v", f)
		}
	}

	s, err := s.WithDetails(
		&errdetails.DebugInfo{
			StackEntries: stackEntries,
			Detail:       e.Debug(),
		},
	)
	if err != nil {
		// this error only occurs if the code is broken AF
		panic(err)
	}

	return s
}

func (e DefaultError) WithReason(reason string) *DefaultError {
	e.ReasonField = reason
	return &e
}

func (e DefaultError) WithReasonf(reason string, args ...interface{}) *DefaultError {
	return e.WithReason(fmt.Sprintf(reason, args...))
}

func (e DefaultError) WithError(message string) *DefaultError {
	e.ErrorField = message
	return &e
}

func (e DefaultError) WithErrorf(message string, args ...interface{}) *DefaultError {
	return e.WithError(fmt.Sprintf(message, args...))
}

func (e DefaultError) WithDebugf(debug string, args ...interface{}) *DefaultError {
	return e.WithDebug(fmt.Sprintf(debug, args...))
}

func (e DefaultError) WithDebug(debug string) *DefaultError {
	e.DebugField = debug
	return &e
}

func (e DefaultError) WithDetail(key string, detail interface{}) *DefaultError {
	if e.DetailsField == nil {
		e.DetailsField = map[string]interface{}{}
	}
	e.DetailsField[key] = detail
	return &e
}

func (e DefaultError) WithDetailf(key string, message string, args ...interface{}) *DefaultError {
	if e.DetailsField == nil {
		e.DetailsField = map[string]interface{}{}
	}
	e.DetailsField[key] = fmt.Sprintf(message, args...)
	return &e
}

func (e DefaultError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "rid=%s\n", e.RIDField)
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
	de := &DefaultError{
		RIDField:     id,
		CodeField:    http.StatusInternalServerError,
		DetailsField: map[string]interface{}{},
		ErrorField:   err.Error(),
	}
	de.Wrap(err)

	if c := errorsx.ReasonCarrier(nil); stderr.As(err, &c) {
		de.ReasonField = c.Reason()
	}
	if c := errorsx.RequestIDCarrier(nil); stderr.As(err, &c) && c.RequestID() != "" {
		de.RIDField = c.RequestID()
	}
	if c := errorsx.DetailsCarrier(nil); stderr.As(err, &c) && c.Details() != nil {
		de.DetailsField = c.Details()
	}
	if c := errorsx.StatusCarrier(nil); stderr.As(err, &c) && c.Status() != "" {
		de.StatusField = c.Status()
	}
	if c := errorsx.StatusCodeCarrier(nil); stderr.As(err, &c) && c.StatusCode() != 0 {
		de.CodeField = c.StatusCode()
	}
	if c := errorsx.DebugCarrier(nil); stderr.As(err, &c) {
		de.DebugField = c.Debug()
	}

	if de.StatusField == "" {
		de.StatusField = http.StatusText(de.StatusCode())
	}

	return de
}
