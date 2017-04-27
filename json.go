package herodot

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

type jsonError struct {
	RequestID    string `json:"request_id"`
	ErrorMessage string `json:"error_message"`
}

// JSONWriter outputs JSON.
type JSONWriter struct {
	logger logrus.FieldLogger
}

// NewJSONWriter returns a JSONWriter
func NewJSONWriter(logger logrus.FieldLogger) *JSONWriter {
	return &JSONWriter{
		logger: logger,
	}
}

// Write a response object to the ResponseWriter with status code 200.
func (h *JSONWriter) Write(w http.ResponseWriter, r *http.Request, e interface{}) {
	h.WriteCode(w, r, http.StatusOK, e)
}

// WriteCode writes a response object to the ResponseWriter and sets a response code.
func (h *JSONWriter) WriteCode(w http.ResponseWriter, r *http.Request, code int, e interface{}) {
	js, err := json.Marshal(e)
	if err != nil {
		h.WriteError(w, r, err)
		return
	}

	if code == 0 {
		code = http.StatusOK
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(js)
}

// WriteCreated writes a response object to the ResponseWriter with status code 201 and
// the Location header set to location.
func (h *JSONWriter) WriteCreated(w http.ResponseWriter, r *http.Request, location string, e interface{}) {
	w.Header().Set("Location", location)
	h.WriteCode(w, r, http.StatusCreated, e)
}

// WriteError writes an error to ResponseWriter and tries to extract the error's status code by
// asserting StatusCodeCarrier. If the error does not implement StatusCodeCarrier, the status code
// is set to 500.
func (h *JSONWriter) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	// If our top-level error is a statusCodeCarrier
	if s, ok := err.(StatusCodeCarrier); ok {
		h.WriteErrorCode(w, r, s.StatusCode(), err)
		return

		// Or if pkg/error was used to wrap it
	} else if s, ok := errors.Cause(err).(StatusCodeCarrier); ok {
		h.WriteErrorCode(w, r, s.StatusCode(), err)
		return
	}

	// Otherwise it's 500 per default
	h.WriteErrorCode(w, r, http.StatusInternalServerError, err)
	return
}

// WriteErrorCode writes an error to ResponseWriter and forces an error code.
func (h *JSONWriter) WriteErrorCode(w http.ResponseWriter, r *http.Request, code int, err error) {
	if code == 0 {
		code = http.StatusInternalServerError
	}

	if h.logger == nil {
		h.logger = logrus.StandardLogger()
		h.logger.Warning("No logger was set in JSONWriter, defaulting to standard logger.")
	}

	// All errors land here, so it's a really good idea to do the logging here as well!
	h.logger.
		WithField("request-id", r.Header.Get("X-Request-ID")).
		WithField("writer", "JSON").
		WithField("trace", getErrorTrace(err)).Error(err.Error())

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(&jsonError{
		// This need to be set by API Gateway
		RequestID:    r.Header.Get("X-Request-ID"),
		ErrorMessage: err.Error(),
	}); err != nil {
		// There was an error, but there's actually not a lot we can do except log that this happened.
		h.logger.
			WithField("request-id", r.Header.Get("X-Request-ID")).
			WithField("writer", "JSON").
			WithField("trace", getErrorTrace(err)).
			WithError(err).
			Error("Could not write jsonError to response writer.")
	}
}
