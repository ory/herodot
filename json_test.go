// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"bytes"
	"encoding/json"
	stderr "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	exampleError = &DefaultError{
		CodeField:   http.StatusNotFound,
		ErrorField:  "foo",
		ReasonField: "some-reason",
		StatusField: "some-status",
		DetailsField: map[string]interface{}{
			"foo": "bar",
		},
	}
	onlyStatusCodeError = &statusCodeError{statusCode: http.StatusNotFound, error: errors.New("foo")}
)

type statusCodeError struct {
	statusCode int
	error
}

func (s *statusCodeError) StatusCode() int {
	return s.statusCode
}

func TestWriteError(t *testing.T) {
	tracedErr := errors.New("err")
	for k, tc := range []struct {
		err    error
		expect *DefaultError
	}{
		{err: exampleError, expect: exampleError},
		{err: errors.WithStack(exampleError), expect: exampleError},
		{err: onlyStatusCodeError, expect: &DefaultError{StatusField: http.StatusText(http.StatusNotFound), CodeField: http.StatusNotFound, ErrorField: "foo"}},
		{err: errors.WithStack(onlyStatusCodeError), expect: &DefaultError{StatusField: http.StatusText(http.StatusNotFound), CodeField: http.StatusNotFound, ErrorField: "foo"}},
		{err: errors.New("foo"), expect: &DefaultError{StatusField: http.StatusText(http.StatusInternalServerError), CodeField: http.StatusInternalServerError, ErrorField: "foo"}},
		{err: errors.WithStack(errors.New("foo1")), expect: &DefaultError{StatusField: http.StatusText(http.StatusInternalServerError), CodeField: http.StatusInternalServerError, ErrorField: "foo1"}},
		{err: stderr.New("foo1"), expect: &DefaultError{StatusField: http.StatusText(http.StatusInternalServerError), CodeField: http.StatusInternalServerError, ErrorField: "foo1"}},
		{
			err: ErrInternalServerError.WithTrace(tracedErr).WithReasonf("Unable to prepare JSON Schema for HTTP Post Body Form parsing: %s", tracedErr).WithDebugf("%+v", tracedErr),
			expect: &DefaultError{
				ReasonField: fmt.Sprintf("Unable to prepare JSON Schema for HTTP Post Body Form parsing: %s", tracedErr),
				StatusField: http.StatusText(http.StatusInternalServerError),
				CodeField:   http.StatusInternalServerError,
				ErrorField:  "An internal server error occurred, please contact the system administrator",
				DebugField:  fmt.Sprintf("%+v", tracedErr),
			},
		},
	} {
		t.Run(fmt.Sprintf("case=%d", k), func(t *testing.T) {
			var j ErrorContainer

			h := NewJSONWriter(nil)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Header.Set("X-Request-ID", "foo")
				h.WriteError(w, r, tc.err)
			}))
			defer ts.Close()

			resp, err := http.Get(ts.URL + "/do")
			require.Nil(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			require.Nil(t, json.NewDecoder(bytes.NewBuffer(body)).Decode(&j), "%s", body)
			assert.Equal(t, tc.expect.StatusCode(), resp.StatusCode, "%s", body)
			assert.Equal(t, "foo", j.Error.RequestID(), "%s", body)
			assert.Equal(t, tc.expect.Status(), j.Error.Status(), "%s", body)
			assert.Equal(t, tc.expect.StatusCode(), j.Error.StatusCode(), "%s", body)
			assert.Equal(t, tc.expect.Reason(), j.Error.Reason(), "%s", body)
			assert.Equal(t, tc.expect.Error(), j.Error.Error(), "%s", body)
		})
	}

	t.Run("case=debug flag", func(t *testing.T) {
		for _, tc := range []struct {
			isDebug bool
			desc    string
		}{
			{
				isDebug: true,
				desc:    "should be set",
			},
			{
				isDebug: false,
				desc:    "should not be set",
			},
		} {
			t.Run(tc.desc, func(t *testing.T) {
				h := NewJSONWriter(nil)
				h.EnableDebug = tc.isDebug
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					h.WriteError(w, r, &DefaultError{DebugField: "foo"})
				}))
				defer ts.Close()

				resp, err := http.Get(ts.URL + "/do")
				require.NoError(t, err)
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)

				var j ErrorContainer
				require.NoError(t, json.NewDecoder(bytes.NewBuffer(body)).Decode(&j), "%s", body)
				assert.Equal(t, tc.isDebug, j.Error.Debug() != "", "%s", body)
			})
		}
	})
}

type testError struct {
	Foo string `json:"foo"`
	Bar string `json:"bar"`
}

func (e *testError) Error() string {
	return e.Foo
}

func TestWriteErrorNoEnrichment(t *testing.T) {
	h := NewJSONWriter(nil)
	h.ErrorEnhancer = nil
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Request-ID", "foo")
		h.WriteErrorCode(w, r, 0, &testError{
			Foo: "foo", Bar: "bar",
		})
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/do")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.EqualValues(t, `{"foo":"foo","bar":"bar"}
`, string(body))
}

func TestWriteErrorCode(t *testing.T) {
	var j ErrorContainer

	h := NewJSONWriter(nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Request-ID", "foo")
		h.WriteErrorCode(w, r, 0, errors.Wrap(exampleError, ""))
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	require.Nil(t, json.NewDecoder(resp.Body).Decode(&j))
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "foo", j.Error.RequestID())
}

func TestWriteJSON(t *testing.T) {
	foo := map[string]string{"foo": "bar"}

	h := NewJSONWriter(nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Write(w, r, &foo)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	result := map[string]string{}
	require.Nil(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, foo["foo"], result["foo"])
}

func TestWriteCreatedJSON(t *testing.T) {
	foo := map[string]string{"foo": "bar"}

	h := NewJSONWriter(nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.WriteCreated(w, r, "/new", &foo)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	result := map[string]string{}
	require.Nil(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, foo["foo"], result["foo"])
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "/new", resp.Header.Get("Location"))
}

func TestWriteCodeJSON(t *testing.T) {
	foo := map[string]string{"foo": "bar"}

	h := NewJSONWriter(nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.WriteCode(w, r, 400, &foo)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	result := map[string]string{}
	require.Nil(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, foo["foo"], result["foo"])
	assert.Equal(t, 400, resp.StatusCode)
}

func TestWriteCodeJSONDefault(t *testing.T) {
	foo := map[string]string{"foo": "bar"}

	h := NewJSONWriter(nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.WriteCode(w, r, 0, &foo)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	result := map[string]string{}
	require.Nil(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, foo["foo"], result["foo"])
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWriteCodeJSONUnescapedHTML(t *testing.T) {
	foo := "b&r"

	h := NewJSONWriter(nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.WriteCode(w, r, 0, &foo, UnescapedHTML)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	result, err := io.ReadAll(resp.Body)
	require.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("\"%s\"\n", foo), string(result))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
