// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextWriterOryErrorIDHeader(t *testing.T) {
	for k, tc := range []struct {
		name           string
		err            error
		expectedHeader string
	}{
		{
			name:           "error with ID sets header",
			err:            &ErrMisconfiguration,
			expectedHeader: "invalid_configuration",
		},
		{
			name:           "error without ID does not set header",
			err:            &ErrNotFound,
			expectedHeader: "",
		},
		{
			name: "custom error with ID sets header",
			err: &DefaultError{
				IDField:     "custom_text_error_id",
				CodeField:   http.StatusBadRequest,
				StatusField: http.StatusText(http.StatusBadRequest),
				ErrorField:  "custom error",
			},
			expectedHeader: "custom_text_error_id",
		},
		{
			name:           "upstream error sets header",
			err:            &ErrUpstreamError,
			expectedHeader: "upstream_error",
		},
	} {
		t.Run(fmt.Sprintf("case=%d/%s", k, tc.name), func(t *testing.T) {
			h := NewTextWriter(&stdReporter{}, "plain")
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				h.WriteError(w, r, tc.err)
			}))
			t.Cleanup(ts.Close)

			resp, err := http.Get(ts.URL + "/do")
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedHeader, resp.Header.Get("Ory-Error-Id"))
		})
	}
}
