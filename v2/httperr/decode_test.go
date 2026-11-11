// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package httperr_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/herodot/v2"
	"github.com/ory/herodot/v2/httperr"
)

func render(err error) *http.Response {
	w := httptest.NewRecorder()
	(&httperr.Writer{}).WriteError(w, httptest.NewRequest("GET", "/", nil), err)
	return w.Result()
}

func TestDecode(t *testing.T) {
	// The sender's catalog words the message differently than the receiver's.
	reworded := herodot.Define("kratos.ory.com", "FLOW_EXPIRED", herodot.CodeExpired, "The flow has expired. Start a new flow.")
	sent := reworded.New(
		herodot.Metadata("flowId", "1234"),
		herodot.Violation("email", "is required"),
	)
	resp := render(sent)
	sentBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Run("case=round trip", func(t *testing.T) {
		resp := render(sent)
		received, ok := httperr.Decode(resp)
		require.True(t, ok)
		assert.True(t, flowExpired.Matches(received))
		assert.Equal(t, herodot.CodeExpired, received.Definition().Code(), "the refinement survives HTTP")
		assert.Equal(t, "The flow has expired. Start a new flow.", received.Error(), "the received message is kept")
		assert.Equal(t, map[string]string{"flowId": "1234"}, received.Metadata())
		assert.Equal(t, []herodot.FieldViolation{{Field: "email", Description: "is required"}}, received.Violations())

		var re *httperr.ResponseError
		require.ErrorAs(t, received, &re)
		assert.Equal(t, 410, re.StatusCode)
		assert.Equal(t, sentBody, re.Body)

		rest, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, sentBody, rest, "the body can still be read")

		forwarded, err := io.ReadAll(render(fmt.Errorf("forwarding: %w", received)).Body)
		require.NoError(t, err)
		assert.JSONEq(t, string(sentBody), string(forwarded), "forwarding doesn't change the response")
	})

	t.Run("case=body only", func(t *testing.T) {
		received, ok := httperr.DecodeBody(sentBody)
		require.True(t, ok)
		assert.Equal(t, herodot.CodeExpired, received.Definition().Code())
	})

	t.Run("case=unknown refinement", func(t *testing.T) {
		received, ok := httperr.DecodeBody([]byte(`{"error":{"code":418,"status":"FAILED_PRECONDITION","message":"m","details":[
			{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"TEAPOT","domain":"kratos.ory.com"}]}}`))
		require.True(t, ok)
		assert.Equal(t, herodot.CodeFailedPrecondition, received.Definition().Code(), "falls back to the canonical code")
	})

	for _, tc := range []struct{ name, body string }{
		{"not JSON", `<html>Bad Gateway</html>`},
		{"legacy shape", `{"error":{"code":404,"status":"Not Found","id":"not_found","message":"m"}}`},
		{"no ErrorInfo", `{"error":{"code":404,"status":"NOT_FOUND","message":"m"}}`},
		{"refinement as status", `{"error":{"code":410,"status":"EXPIRED","message":"m","details":[
			{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"FLOW_EXPIRED","domain":"kratos.ory.com"}]}}`},
		{"invalid reason", `{"error":{"code":404,"status":"NOT_FOUND","message":"m","details":[
			{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"not found","domain":"kratos.ory.com"}]}}`},
	} {
		t.Run("case=not a herodot error/"+tc.name, func(t *testing.T) {
			_, ok := httperr.DecodeBody([]byte(tc.body))
			assert.False(t, ok)
		})
	}

	t.Run("case=not an error response", func(t *testing.T) {
		_, ok := httperr.Decode(&http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(sentBody)))})
		assert.False(t, ok)
		_, ok = httperr.Decode(nil)
		assert.False(t, ok)
	})
}
