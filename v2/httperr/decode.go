// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package httperr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ory/herodot/v2"
)

// MaxDecodeSize is the number of bytes of a response body that Decode reads.
const MaxDecodeSize = 1 << 20

// Decode decodes an error response received from another service, e.g. one
// that a Writer rendered. It returns false if the response isn't a herodot
// error, see DecodeBody.
//
// Decode reads at most MaxDecodeSize bytes of the body and replaces resp.Body
// with what it read, so callers can still read it. It doesn't close the body.
func Decode(resp *http.Response) (herodot.Error, bool) {
	if resp == nil || resp.Body == nil || resp.StatusCode < 400 {
		return herodot.Error{}, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxDecodeSize))
	resp.Body = readCloser{io.MultiReader(bytes.NewReader(body), resp.Body), resp.Body}
	if err != nil {
		return herodot.Error{}, false
	}
	return decode(body, resp.StatusCode)
}

// DecodeBody decodes an AIP-193 error response body, e.g. one returned by a
// generated API client. It returns false if the body isn't a herodot error:
// not the AIP-193 shape, no ErrorInfo, or an invalid domain, reason or code.
//
// The Error has the received identity, so catalog definitions match it, and
// returning it forwards the error unchanged. Its code is the one that maps to
// the received status and HTTP status code, or the canonical code if this
// version of herodot doesn't know that combination. Its message, metadata and
// violations are the received ones, and its cause holds the body.
func DecodeBody(body []byte) (herodot.Error, bool) {
	return decode(body, 0)
}

func decode(body []byte, httpStatus int) (herodot.Error, bool) {
	var b struct {
		Error struct {
			Code    int               `json:"code"`
			Status  string            `json:"status"`
			Message string            `json:"message"`
			Details []json.RawMessage `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return herodot.Error{}, false
	}
	if b.Error.Code != 0 {
		httpStatus = b.Error.Code
	}

	canonical, ok := herodot.CanonicalCode(b.Error.Status)
	if !ok {
		return herodot.Error{}, false
	}
	code, ok := herodot.LookupCode(canonical, httpStatus)
	if !ok {
		code = canonical
	}

	var (
		info       *ErrorInfo
		violations []herodot.Option
	)
	for _, raw := range b.Error.Details {
		var t struct {
			Type string `json:"@type"`
		}
		if json.Unmarshal(raw, &t) != nil {
			continue
		}
		switch t.Type {
		case typePrefix + "ErrorInfo":
			var d ErrorInfo
			if info == nil && json.Unmarshal(raw, &d) == nil {
				info = &d
			}
		case typePrefix + "BadRequest":
			var d BadRequest
			if json.Unmarshal(raw, &d) == nil {
				for _, v := range d.FieldViolations {
					violations = append(violations, herodot.Violation(v.Field, v.Description))
				}
			}
		}
	}
	if info == nil {
		return herodot.Error{}, false
	}

	def, err := herodot.NewDefinition(info.Domain, info.Reason, code, b.Error.Message)
	if err != nil {
		return herodot.Error{}, false
	}
	opts := []herodot.Option{herodot.Cause(&ResponseError{StatusCode: httpStatus, Body: body})}
	for k, v := range info.Metadata {
		opts = append(opts, herodot.Metadata(k, v))
	}
	return def.New(append(opts, violations...)...), true
}

// ResponseError is the cause of a decoded Error: the received response.
type ResponseError struct {
	StatusCode int
	Body       []byte
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("received HTTP %d: %s", e.StatusCode, bytes.TrimSpace(e.Body))
}

type readCloser struct {
	io.Reader
	io.Closer
}
