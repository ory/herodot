// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"bytes"
	"context"
	"encoding/json"
	stderr "errors"
	"net/http"

	"github.com/pkg/errors"
)

type ErrorContainer struct {
	Error *DefaultError `json:"error"`
}

type ErrorReporter interface {
	ReportError(r *http.Request, code int, err error, args ...interface{})
}

type EncoderOptions func(*json.Encoder)

// UnescapedHTML prevents HTML entities &, <, > from being unicode-escaped.
func UnescapedHTML(enc *json.Encoder) {
	enc.SetEscapeHTML(false)
}

// JSONWriter writes JSON responses (obviously).
type JSONWriter struct {
	Reporter      ErrorReporter
	ErrorEnhancer func(r *http.Request, err error) interface{}
	EnableDebug   bool
}

var _ Writer = (*JSONWriter)(nil)

func NewJSONWriter(reporter ErrorReporter) *JSONWriter {
	writer := &JSONWriter{
		Reporter:      reporter,
		ErrorEnhancer: defaultJSONErrorEnhancer,
	}
	if writer.Reporter == nil {
		writer.Reporter = &stdReporter{}
	}

	writer.ErrorEnhancer = defaultJSONErrorEnhancer
	return writer
}

type ErrorEnhancer interface {
	EnhanceJSONError() interface{}
}

func defaultJSONErrorEnhancer(r *http.Request, err error) interface{} {
	if e, ok := err.(ErrorEnhancer); ok {
		return e.EnhanceJSONError()
	}
	return &ErrorContainer{Error: ToDefaultError(err, r.Header.Get("X-Request-ID"))}
}

func Scrub5xxJSONErrorEnhancer(r *http.Request, err error) interface{} {
	payload := defaultJSONErrorEnhancer(r, err)

	if de, ok := payload.(DefaultError); ok {
		if de.StatusCode() >= 500 {
			return scrub5xxError(&de)
		}
		return payload
	}
	if ec, ok := payload.(*ErrorContainer); ok {
		if ec.Error.CodeField >= 500 {
			return scrub5xxError(ec.Error)
		}
		return payload
	}

	// We have some other error, which we always want to scrub.
	return &ErrorContainer{Error: ToDefaultError(&ErrInternalServerError, r.Header.Get("X-Request-ID"))}
}

func scrub5xxError(err *DefaultError) *ErrorContainer {
	return &ErrorContainer{Error: &DefaultError{
		IDField:       err.IDField,
		CodeField:     err.CodeField,
		StatusField:   err.StatusField,
		RIDField:      err.RIDField,
		GRPCCodeField: err.GRPCCodeField,
	}}
}

// Write a response object to the ResponseWriter with status code 200.
func (h *JSONWriter) Write(w http.ResponseWriter, r *http.Request, e interface{}, opts ...EncoderOptions) {
	h.WriteCode(w, r, http.StatusOK, e, opts...)
}

// WriteCode writes a response object to the ResponseWriter and sets a response code.
func (h *JSONWriter) WriteCode(w http.ResponseWriter, r *http.Request, code int, e interface{}, opts ...EncoderOptions) {
	bs := new(bytes.Buffer)
	enc := json.NewEncoder(bs)
	for _, opt := range opts {
		opt(enc)
	}

	err := enc.Encode(e)
	if err != nil {
		h.WriteError(w, r, errors.WithStack(err))
		return
	}

	if code == 0 {
		code = http.StatusOK
	}

	if errors.Is(r.Context().Err(), context.Canceled) {
		code = StatusClientClosedRequest
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(bs.Bytes())
}

// WriteCreated writes a response object to the ResponseWriter with status code 201 and
// the Location header set to location.
func (h *JSONWriter) WriteCreated(w http.ResponseWriter, r *http.Request, location string, e interface{}) {
	w.Header().Set("Location", location)
	h.WriteCode(w, r, http.StatusCreated, e)
}

// WriteError writes an error to ResponseWriter and tries to extract the error's status code by
// asserting statusCodeCarrier. If the error does not implement statusCodeCarrier, the status code
// is set to 500.
func (h *JSONWriter) WriteError(w http.ResponseWriter, r *http.Request, err error, opts ...Option) {
	if c := StatusCodeCarrier(nil); stderr.As(err, &c) {
		h.WriteErrorCode(w, r, c.StatusCode(), err)
	} else {
		h.WriteErrorCode(w, r, http.StatusInternalServerError, err, opts...)
	}
}

// WriteErrorCode writes an error to ResponseWriter and forces an error code.
func (h *JSONWriter) WriteErrorCode(w http.ResponseWriter, r *http.Request, code int, err error, opts ...Option) {
	o := newOptions(opts)

	if code == 0 {
		code = http.StatusInternalServerError
	}

	if errors.Is(r.Context().Err(), context.Canceled) {
		code = StatusClientClosedRequest
	}

	if !o.noLog {
		// All errors land here, so it's a really good idea to do the logging here as well!
		h.Reporter.ReportError(r, code, coalesceError(err), "An error occurred while handling a request")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	// Enhancing must happen after logging or context will be lost.
	var payload interface{} = err
	if h.ErrorEnhancer != nil {
		payload = h.ErrorEnhancer(r, err)
	}
	if de, ok := payload.(*DefaultError); ok && !h.EnableDebug {
		de2 := *de
		de2.DebugField = ""
		payload = &de2
	}
	if ec, ok := payload.(*ErrorContainer); ok && !h.EnableDebug {
		de2 := *ec.Error
		de2.DebugField = ""
		ec2 := *ec
		ec2.Error = &de2
		payload = ec2
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// There was an error, but there's actually not a lot we can do except log that this happened.
		h.Reporter.ReportError(r, code, errors.WithStack(err), "Could not write ErrorContainer to response writer")
	}
}
