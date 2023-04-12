// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	stderr "errors"
	"fmt"
	"io"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// swagger:ignore
type DefaultError struct {
	// The error ID
	//
	// Useful when trying to identify various errors in application logic.
	IDField string `json:"id,omitempty"`

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

func (e DefaultError) WithID(id string) *DefaultError {
	e.IDField = id
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
			e.IDField == te.IDField &&
			e.CodeField == te.CodeField
	case *DefaultError:
		return e.ErrorField == te.ErrorField &&
			e.StatusField == te.StatusField &&
			e.IDField == te.IDField &&
			e.CodeField == te.CodeField
	default:
		return false
	}
}

func (e DefaultError) Status() string {
	return e.StatusField
}

func (e DefaultError) ID() string {
	return e.IDField
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

	details := []proto.Message{}

	if e.DebugField != "" || st != nil {
		details = append(details, &errdetails.DebugInfo{
			StackEntries: stackEntries,
			Detail:       e.Debug(),
		})
	}

	if e.ReasonField != "" {
		details = append(details, &errdetails.ErrorInfo{
			Reason: e.Reason(),
		})
	}

	if e.RequestID() != "" {
		details = append(details, &errdetails.RequestInfo{
			RequestId: e.RequestID(),
		})
	}

	if e.GRPCCodeField == codes.InvalidArgument && e.err != nil {
		if fvs := e.fieldViolations(); len(fvs) > 0 {
			details = append(details, &errdetails.BadRequest{
				FieldViolations: fvs,
			})
		}
	}

	s, err := s.WithDetails(details...)
	if err != nil {
		// this error only occurs if the code is broken AF
		panic(err)
	}

	return s
}

// fieldViolationError is an interface implemented by proto-gen-validate.
type fieldViolationError interface {
	Field() string
	Reason() string
	Cause() error
}
type multiError interface {
	AllErrors() []error
}

func rootCauses(err fieldViolationError) []fieldViolationError {
	if err == nil {
		return []fieldViolationError{}
	}

	switch e := err.Cause().(type) {
	case fieldViolationError:
		return rootCauses(e)

	case multiError:
		var causes []fieldViolationError
		for _, e := range e.AllErrors() {
			if fvErr, ok := e.(fieldViolationError); ok {
				causes = append(causes, rootCauses(fvErr)...)
			}
		}
		return causes
	}
	return []fieldViolationError{err}
}

func (e DefaultError) fieldViolations() (fv []*errdetails.BadRequest_FieldViolation) {
	err, ok := e.err.(multiError)
	if !ok {
		return
	}
	for _, e := range err.AllErrors() {
		if fvErr, ok := e.(fieldViolationError); ok {
			// We only want to show the root cause of the error.
			for _, cause := range rootCauses(fvErr) {
				fv = append(fv, &errdetails.BadRequest_FieldViolation{
					Field:       cause.Field(),
					Description: cause.Reason(),
				})
			}
		}
	}

	return
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
			_, _ = fmt.Fprintf(s, "id=%s\n", e.IDField)
			_, _ = fmt.Fprintf(s, "rid=%s\n", e.RIDField)
			_, _ = fmt.Fprintf(s, "error=%s\n", e.ErrorField)
			_, _ = fmt.Fprintf(s, "reason=%s\n", e.ReasonField)
			_, _ = fmt.Fprintf(s, "details=%+v\n", e.DetailsField)
			_, _ = fmt.Fprintf(s, "debug=%s\n", e.DebugField)
			e.StackTrace().Format(s, verb)
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, e.ErrorField)
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.ErrorField)
	}
}

func ToDefaultError(err error, requestID string) *DefaultError {
	de := &DefaultError{
		RIDField:     requestID,
		CodeField:    http.StatusInternalServerError,
		DetailsField: map[string]interface{}{},
		ErrorField:   err.Error(),
	}
	de.Wrap(err)

	if c := ReasonCarrier(nil); stderr.As(err, &c) {
		de.ReasonField = c.Reason()
	}
	if c := RequestIDCarrier(nil); stderr.As(err, &c) && c.RequestID() != "" {
		de.RIDField = c.RequestID()
	}
	if c := DetailsCarrier(nil); stderr.As(err, &c) && c.Details() != nil {
		de.DetailsField = c.Details()
	}
	if c := StatusCarrier(nil); stderr.As(err, &c) && c.Status() != "" {
		de.StatusField = c.Status()
	}
	if c := StatusCodeCarrier(nil); stderr.As(err, &c) && c.StatusCode() != 0 {
		de.CodeField = c.StatusCode()
	}
	if c := DebugCarrier(nil); stderr.As(err, &c) {
		de.DebugField = c.Debug()
	}
	if c := IDCarrier(nil); stderr.As(err, &c) {
		de.IDField = c.ID()
	}

	if de.StatusField == "" {
		de.StatusField = http.StatusText(de.StatusCode())
	}

	return de
}

// StatusCodeCarrier can be implemented by an error to support setting status codes in the error itself.
type StatusCodeCarrier interface {
	// StatusCode returns the status code of this error.
	StatusCode() int
}

// RequestIDCarrier can be implemented by an error to support error contexts.
type RequestIDCarrier interface {
	// RequestID returns the ID of the request that caused the error, if applicable.
	RequestID() string
}

// ReasonCarrier can be implemented by an error to support error contexts.
type ReasonCarrier interface {
	// Reason returns the reason for the error, if applicable.
	Reason() string
}

// DebugCarrier can be implemented by an error to support error contexts.
type DebugCarrier interface {
	// Debug returns debugging information for the error, if applicable.
	Debug() string
}

// StatusCarrier can be implemented by an error to support error contexts.
type StatusCarrier interface {
	// ID returns the error id, if applicable.
	Status() string
}

// DetailsCarrier can be implemented by an error to support error contexts.
type DetailsCarrier interface {
	// Details returns details on the error, if applicable.
	Details() map[string]interface{}
}

// IDCarrier can be implemented by an error to support error contexts.
type IDCarrier interface {
	// ID returns application error ID on the error, if applicable.
	ID() string
}
