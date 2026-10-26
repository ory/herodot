// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

// Package httperr renders herodot errors as HTTP responses.
package httperr

import (
	"encoding/json"
	"net/http"

	"github.com/ory/herodot/v2"
)

// Options are decided by the server, not by the error.
type Options struct {
	RequestID string // rendered as RequestInfo
	Debug     bool   // render DebugInfo (cause chain, stack trace), e.g. with --dev
}

// Renderer writes an error response, including the status, which is
// e.Definition().Code().HTTPStatus(). Consumers implement it for their own
// wire format, e.g. the legacy Ory shape.
type Renderer interface {
	Render(w http.ResponseWriter, r *http.Request, e herodot.Error, o Options)
}

// RendererFunc adapts a function to the Renderer interface.
type RendererFunc func(w http.ResponseWriter, r *http.Request, e herodot.Error, o Options)

func (f RendererFunc) Render(w http.ResponseWriter, r *http.Request, e herodot.Error, o Options) {
	f(w, r, e, o)
}

// AIP renders the AIP-193 JSON shape.
var AIP Renderer = RendererFunc(renderAIP)

// Writer classifies errors with herodot.From and renders them with the status
// From(err).Definition().Code().HTTPStatus().
type Writer struct {
	Renderer  Renderer                   // defaults to AIP
	Debug     bool                       // see Options.Debug
	RequestID func(*http.Request) string // defaults to the X-Request-ID header
}

// WriteError writes the response for err.
func (w *Writer) WriteError(rw http.ResponseWriter, r *http.Request, err error) {
	o := Options{Debug: w.Debug}
	if w.RequestID != nil {
		o.RequestID = w.RequestID(r)
	} else {
		o.RequestID = r.Header.Get("X-Request-ID")
	}
	renderer := w.Renderer
	if renderer == nil {
		renderer = AIP
	}
	renderer.Render(rw, r, herodot.From(err), o)
}

// Body is the AIP-193 JSON shape of an error response.
type Body struct {
	Error Status `json:"error"`
}

// Status is the JSON representation of google.rpc.Status, as AIP-193 renders
// it for HTTP.
type Status struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Details []any  `json:"details,omitempty"`
}

// ErrorInfo is the JSON representation of google.rpc.ErrorInfo.
type ErrorInfo struct {
	Type     string            `json:"@type"`
	Reason   string            `json:"reason"`
	Domain   string            `json:"domain"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RequestInfo is the JSON representation of google.rpc.RequestInfo.
type RequestInfo struct {
	Type      string `json:"@type"`
	RequestID string `json:"requestId"`
}

// BadRequest is the JSON representation of google.rpc.BadRequest.
type BadRequest struct {
	Type            string           `json:"@type"`
	FieldViolations []FieldViolation `json:"fieldViolations"`
}

// FieldViolation is the JSON representation of
// google.rpc.BadRequest.FieldViolation.
type FieldViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

// DebugInfo is the JSON representation of google.rpc.DebugInfo.
type DebugInfo struct {
	Type         string   `json:"@type"`
	StackEntries []string `json:"stackEntries,omitempty"`
	Detail       string   `json:"detail,omitempty"`
}

const typePrefix = "type.googleapis.com/google.rpc."

// NewBody returns the AIP-193 body for e.
func NewBody(e herodot.Error, o Options) Body {
	def := e.Definition()
	details := []any{ErrorInfo{
		Type:     typePrefix + "ErrorInfo",
		Reason:   def.Reason(),
		Domain:   def.Domain(),
		Metadata: e.Metadata(),
	}}
	if o.RequestID != "" {
		details = append(details, RequestInfo{Type: typePrefix + "RequestInfo", RequestID: o.RequestID})
	}
	if vs := e.Violations(); len(vs) > 0 {
		br := BadRequest{Type: typePrefix + "BadRequest", FieldViolations: make([]FieldViolation, len(vs))}
		for i, v := range vs {
			br.FieldViolations[i] = FieldViolation(v)
		}
		details = append(details, br)
	}
	if o.Debug {
		if info, ok := e.DebugInfo(); ok {
			details = append(details, DebugInfo{Type: typePrefix + "DebugInfo", StackEntries: info.StackEntries, Detail: info.Detail})
		}
	}
	return Body{Error: Status{
		Code:    def.Code().HTTPStatus(),
		Status:  def.Code().Canonical().String(),
		Message: e.Error(),
		Details: details,
	}}
}

func renderAIP(w http.ResponseWriter, _ *http.Request, e herodot.Error, o Options) {
	b, err := json.Marshal(NewBody(e, o))
	if err != nil {
		// Only strings and slices of strings are encoded, so this can't
		// happen.
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.Definition().Code().HTTPStatus())
	_, _ = w.Write(append(b, '\n'))
}
