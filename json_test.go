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
	"fmt"
	"io"
	"io/ioutil"
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
			var j jsonError

			h := NewJSONWriter(nil)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Header.Set("X-Request-ID", "foo")
				h.WriteError(w, r, tc.err)
			}))
			defer ts.Close()

			resp, err := http.Get(ts.URL + "/do")
			require.Nil(t, err)
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)

			require.Nil(t, json.NewDecoder(bytes.NewBuffer(body)).Decode(&j), "%s", body)
			assert.Equal(t, tc.expect.StatusCode(), resp.StatusCode, "%s", body)
			assert.Equal(t, "foo", j.Error.RequestID(), "%s", body)
			assert.Equal(t, tc.expect.Status(), j.Error.Status(), "%s", body)
			assert.Equal(t, tc.expect.StatusCode(), j.Error.StatusCode(), "%s", body)
			assert.Equal(t, tc.expect.Reason(), j.Error.Reason(), "%s", body)
			assert.Equal(t, tc.expect.Error(), j.Error.Error(), "%s", body)
		})
	}
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
	require.Nil(t, err)
	body, err := ioutil.ReadAll(resp.Body)

	assert.EqualValues(t, `{"foo":"foo","bar":"bar"}
`, string(body))
}

func TestWriteErrorCode(t *testing.T) {
	var j jsonError

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
