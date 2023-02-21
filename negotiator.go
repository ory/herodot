// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"net/http"

	"github.com/ory/herodot/httputil"
)

// NegotiationHandler automatically negotiates the content type with the request client.
type NegotiationHandler struct {
	json  *JSONWriter
	plain *TextWriter
	html  *TextWriter
	types []string
}

// NewNegotiationHandler creates a new NewNegotiationHandler.
func NewNegotiationHandler(reporter ErrorReporter) *NegotiationHandler {
	return &NegotiationHandler{
		json:  NewJSONWriter(reporter),
		plain: NewTextWriter(reporter, "plain"),
		html:  NewTextWriter(reporter, "html"),
		types: []string{
			"application/json",
		},
	}
}

// Write a response object to the ResponseWriter with status code 200.
func (h *NegotiationHandler) Write(w http.ResponseWriter, r *http.Request, e interface{}) {
	switch httputil.NegotiateContentType(r, []string{}, "application/json") {
	case "text/html":
		h.html.Write(w, r, e)
		return
	case "text/plain":
		h.plain.Write(w, r, e)
		return
	case "application/json":
		h.json.Write(w, r, e)
		return
	default:
		h.json.Write(w, r, e)
		return
	}
}

// WriteCode writes a response object to the ResponseWriter and sets a response code.
func (h *NegotiationHandler) WriteCode(w http.ResponseWriter, r *http.Request, code int, e interface{}) {
	switch httputil.NegotiateContentType(r, []string{}, "application/json") {
	case "text/html":
		h.html.WriteCode(w, r, code, e)
		return
	case "text/plain":
		h.plain.WriteCode(w, r, code, e)
		return
	case "application/json":
		h.json.WriteCode(w, r, code, e)
		return
	default:
		h.json.WriteCode(w, r, code, e)
		return
	}
}

// WriteCreated writes a response object to the ResponseWriter with status code 201 and
// the Location header set to location.
func (h *NegotiationHandler) WriteCreated(w http.ResponseWriter, r *http.Request, location string, e interface{}) {
	switch httputil.NegotiateContentType(r, []string{}, "application/json") {
	case "text/html":
		h.html.WriteCreated(w, r, location, e)
		return
	case "text/plain":
		h.plain.WriteCreated(w, r, location, e)
		return
	case "application/json":
		h.json.WriteCreated(w, r, location, e)
		return
	default:
		h.json.WriteCreated(w, r, location, e)
		return
	}
}

// WriteError writes an error to ResponseWriter and tries to extract the error's status code by
// asserting statusCodeCarrier. If the error does not implement statusCodeCarrier, the status code
// is set to 500.
func (h *NegotiationHandler) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	switch httputil.NegotiateContentType(r, []string{}, "application/json") {
	case "text/html":
		h.html.WriteError(w, r, err)
		return
	case "text/plain":
		h.plain.WriteError(w, r, err)
		return
	case "application/json":
		h.json.WriteError(w, r, err)
		return
	default:
		h.json.WriteError(w, r, err)
		return
	}
}

// WriteErrorCode writes an error to ResponseWriter and forces an error code.
func (h *NegotiationHandler) WriteErrorCode(w http.ResponseWriter, r *http.Request, code int, err error) {
	switch httputil.NegotiateContentType(r, []string{}, "application/json") {
	case "text/html":
		h.html.WriteErrorCode(w, r, code, err)
		return
	case "text/plain":
		h.plain.WriteErrorCode(w, r, code, err)
		return
	case "application/json":
		h.json.WriteErrorCode(w, r, code, err)
		return
	default:
		h.json.WriteErrorCode(w, r, code, err)
		return
	}
}
