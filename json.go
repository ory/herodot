/*
 * Copyright © 2015-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @author		Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @copyright 	2015-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @license 	Apache-2.0
 */
package herodot

import (
	"bytes"
	"encoding/json"
	stderr "errors"
	"net/http"

	"github.com/pkg/errors"

	"github.com/ory/x/logrusx"
)

type jsonError struct {
	Error *DefaultError `json:"error"`
}

type reporter func(logger *logrusx.Logger, args ...interface{}) func(w http.ResponseWriter, r *http.Request, code int, err error)

type EncoderOptions func(*json.Encoder)

// UnescapedHTML prevents HTML entities &, <, > from being unicode-escaped.
func UnescapedHTML(enc *json.Encoder) {
	enc.SetEscapeHTML(false)
}

// json outputs JSON.
type JSONWriter struct {
	logger        *logrusx.Logger
	Reporter      reporter
	ErrorEnhancer func(r *http.Request, err error) interface{}
}

// NewJSONWriter returns a json
func NewJSONWriter(logger *logrusx.Logger) *JSONWriter {
	writer := &JSONWriter{logger: logger}

	writer.Reporter = DefaultErrorReporter
	writer.ErrorEnhancer = defaultJSONErrorEnhancer
	return writer
}

func defaultJSONErrorEnhancer(r *http.Request, err error) interface{} {
	return &jsonError{Error: ToDefaultError(err, r.Header.Get("X-Request-ID"))}
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
	if c := statusCodeCarrier(nil); stderr.As(err, &c) {
		h.WriteErrorCode(w, r, c.StatusCode(), err)
		return
	}

	h.WriteErrorCode(w, r, http.StatusInternalServerError, err, opts...)
	return
}

// WriteErrorCode writes an error to ResponseWriter and forces an error code.
func (h *JSONWriter) WriteErrorCode(w http.ResponseWriter, r *http.Request, code int, err error, opts ...Option) {
	o := newOptions(opts)

	if code == 0 {
		code = http.StatusInternalServerError
	}

	if !o.noLog {
		// All errors land here, so it's a really good idea to do the logging here as well!
		h.Reporter(h.logger, "An error occurred while handling a request")(w, r, code, toError(err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	// Enhancing must happen after logging or context will be lost.
	var payload interface{} = err
	if h.ErrorEnhancer != nil {
		payload = h.ErrorEnhancer(r, err)
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// There was an error, but there's actually not a lot we can do except log that this happened.
		h.Reporter(h.logger, "Could not write jsonError to response writer")(w, r, code, errors.WithStack(err))
	}
}
