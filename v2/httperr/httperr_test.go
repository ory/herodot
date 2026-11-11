// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package httperr_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/herodot/v2"
	"github.com/ory/herodot/v2/httperr"
)

var flowExpired = herodot.Define("kratos.ory.com", "FLOW_EXPIRED", herodot.CodeExpired, "The self-service flow has expired.")

type stackErr struct{}

func (stackErr) Error() string { return "with stack" }
func (stackErr) StackTrace() []fmt.Stringer {
	return []fmt.Stringer{frame("main.go:1"), frame("main.go:2")}
}

type frame string

func (f frame) String() string { return string(f) }

func TestWriter(t *testing.T) {
	err := herodot.Annotate(flowExpired.New(
		herodot.Metadata("flowId", "1234"),
		herodot.Violation("email", "is required"),
		herodot.Cause(fmt.Errorf("db: %w", stackErr{})),
	), herodot.Metadata("returnTo", "/home"))

	for _, tc := range []struct {
		name     string
		writer   httperr.Writer
		header   string
		expected string
	}{{
		name:   "default",
		header: "rid-1",
		expected: `{"error":{"code":410,"status":"FAILED_PRECONDITION","message":"The self-service flow has expired.","details":[
			{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"FLOW_EXPIRED","domain":"kratos.ory.com","metadata":{"flowId":"1234","returnTo":"/home"}},
			{"@type":"type.googleapis.com/google.rpc.RequestInfo","requestId":"rid-1"},
			{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"email","description":"is required"}]}]}}`,
	}, {
		name:   "debug and custom request ID",
		writer: httperr.Writer{Debug: true, RequestID: func(*http.Request) string { return "custom" }},
		header: "rid-1",
		expected: `{"error":{"code":410,"status":"FAILED_PRECONDITION","message":"The self-service flow has expired.","details":[
			{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"FLOW_EXPIRED","domain":"kratos.ory.com","metadata":{"flowId":"1234","returnTo":"/home"}},
			{"@type":"type.googleapis.com/google.rpc.RequestInfo","requestId":"custom"},
			{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"email","description":"is required"}]},
			{"@type":"type.googleapis.com/google.rpc.DebugInfo","stackEntries":["main.go:1","main.go:2"],"detail":"db: with stack"}]}}`,
	}} {
		t.Run("case="+tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("X-Request-ID", tc.header)
			w := httptest.NewRecorder()
			tc.writer.WriteError(w, r, err)

			assert.Equal(t, 410, w.Code)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
			assert.JSONEq(t, tc.expected, w.Body.String())
		})
	}

	t.Run("case=unclassified error", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&httperr.Writer{}).WriteError(w, httptest.NewRequest("GET", "/", nil), fmt.Errorf("secret detail"))
		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":{"code":500,"status":"INTERNAL","message":"An internal server error occurred. Please contact the system administrator.","details":[
			{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"INTERNAL","domain":"ory.com"}]}}`, w.Body.String())
	})

	t.Run("case=custom renderer", func(t *testing.T) {
		w := httptest.NewRecorder()
		writer := httperr.Writer{Renderer: httperr.RendererFunc(func(w http.ResponseWriter, _ *http.Request, e herodot.Error, o httperr.Options) {
			w.WriteHeader(e.Definition().Code().HTTPStatus())
			_ = json.NewEncoder(w).Encode(map[string]string{"id": e.Definition().Reason(), "rid": o.RequestID})
		})}
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-Request-ID", "r")
		writer.WriteError(w, r, flowExpired.New())
		assert.Equal(t, 410, w.Code)
		require.JSONEq(t, `{"id":"FLOW_EXPIRED","rid":"r"}`, w.Body.String())
	})
}
