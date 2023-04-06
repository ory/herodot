// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

// TextWriter outputs plain text
type TextWriter struct {
	Reporter    ErrorReporter
	contentType string
}

var _ Writer = (*TextWriter)(nil)

// NewTextWriter returns a plain text writer
func NewTextWriter(reporter ErrorReporter, contentType string) *TextWriter {
	if contentType == "" {
		contentType = "plain"
	}

	writer := &TextWriter{
		Reporter:    reporter,
		contentType: "text/" + contentType,
	}

	return writer
}

// Write a response object to the ResponseWriter with status code 200.
func (h *TextWriter) Write(w http.ResponseWriter, r *http.Request, e interface{}, _ ...EncoderOptions) {
	h.WriteCode(w, r, http.StatusOK, e)
}

// WriteCode writes a response object to the ResponseWriter and sets a response code.
func (h *TextWriter) WriteCode(w http.ResponseWriter, r *http.Request, code int, e interface{}, _ ...EncoderOptions) {
	if code == 0 {
		code = http.StatusOK
	}

	w.Header().Set("Content-Type", h.contentType)
	w.WriteHeader(code)
	fmt.Fprintf(w, "%s", e)
}

// WriteCreated writes a response object to the ResponseWriter with status code 201 and
// the Location header set to location.
func (h *TextWriter) WriteCreated(w http.ResponseWriter, r *http.Request, location string, e interface{}) {
	w.Header().Set("Location", location)
	h.WriteCode(w, r, http.StatusCreated, e)
}

// WriteError writes an error to ResponseWriter and tries to extract the error's status code by
// asserting statusCodeCarrier. If the error does not implement statusCodeCarrier, the status code
// is set to 500.
func (h *TextWriter) WriteError(w http.ResponseWriter, r *http.Request, err error, _ ...Option) {
	if s, ok := errors.Cause(coalesceError(err)).(StatusCodeCarrier); ok {
		h.WriteErrorCode(w, r, s.StatusCode(), err)
		return
	} else {
		h.WriteErrorCode(w, r, http.StatusInternalServerError, err)
	}
}

// WriteErrorCode writes an error to ResponseWriter and forces an error code.
func (h *TextWriter) WriteErrorCode(w http.ResponseWriter, r *http.Request, code int, err error, _ ...Option) {
	err = coalesceError(err)

	if code == 0 {
		code = http.StatusInternalServerError
	}

	// All errors land here, so it's a really good idea to do the logging here as well!
	h.Reporter.ReportError(r, code, err, "An error occurred while handling a request")

	w.Header().Set("Content-Type", h.contentType)
	w.WriteHeader(code)
	fmt.Fprintf(w, "%s", err)
}
