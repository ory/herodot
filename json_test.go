package herodot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	exampleError = &statusCodeError{
		statusCode: http.StatusNotFound,
		error:      errors.New("foo"),
	}
)

type statusCodeError struct {
	error
	statusCode int
}

func (e *statusCodeError) StatusCode() int {
	return e.statusCode
}

func TestWriteError(t *testing.T) {
	for _, tc := range []error{
		exampleError,
		errors.WithStack(exampleError),
	} {
		var j jsonError

		h := NewJSONWriter(nil)
		r := mux.NewRouter()
		r.HandleFunc("/do", func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", "foo")
			h.WriteError(w, r, tc)
		})
		ts := httptest.NewServer(r)

		resp, err := http.Get(ts.URL + "/do")
		require.Nil(t, err)

		require.Nil(t, json.NewDecoder(resp.Body).Decode(&j))
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, "foo", j.RequestID)
	}
}

func TestWriteErrorCode(t *testing.T) {
	var j jsonError

	h := NewJSONWriter(nil)
	r := mux.NewRouter()
	r.HandleFunc("/do", func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Request-ID", "foo")
		h.WriteErrorCode(w, r, http.StatusBadRequest, errors.Wrap(exampleError, ""))
	})
	ts := httptest.NewServer(r)

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	require.Nil(t, json.NewDecoder(resp.Body).Decode(&j))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "foo", j.RequestID)
}

func TestWriteJSON(t *testing.T) {
	foo := map[string]string{"foo": "bar"}

	h := NewJSONWriter(nil)
	r := mux.NewRouter()
	r.HandleFunc("/do", func(w http.ResponseWriter, r *http.Request) {
		h.Write(w, r, &foo)
	})
	ts := httptest.NewServer(r)

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	result := map[string]string{}
	require.Nil(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, foo["foo"], result["foo"])
}

func TestWriteCreatedJSON(t *testing.T) {
	foo := map[string]string{"foo": "bar"}

	h := NewJSONWriter(nil)
	r := mux.NewRouter()
	r.HandleFunc("/do", func(w http.ResponseWriter, r *http.Request) {
		h.WriteCreated(w, r, "/new", &foo)
	})
	ts := httptest.NewServer(r)

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
	r := mux.NewRouter()
	r.HandleFunc("/do", func(w http.ResponseWriter, r *http.Request) {
		h.WriteCode(w, r, 400, &foo)
	})
	ts := httptest.NewServer(r)

	resp, err := http.Get(ts.URL + "/do")
	require.Nil(t, err)

	result := map[string]string{}
	require.Nil(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, foo["foo"], result["foo"])
	assert.Equal(t, 400, resp.StatusCode)
}
