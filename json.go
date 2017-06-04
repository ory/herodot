package herodot

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/pkg/errors"
)

type jsonError struct {
	Error *richError `json:"error"`
}

type reporter func(w http.ResponseWriter, r *http.Request, code int, err error)

// JSONWriter outputs JSON.
type JSONWriter struct {
	logger   logrus.FieldLogger
	Reporter func(args ...interface{}) reporter
}

// NewJSONWriter returns a JSONWriter
func NewJSONWriter(logger logrus.FieldLogger) *JSONWriter {
	writer := &JSONWriter{
		logger: logger,
	}

	writer.Reporter = writer.reporter
	return writer
}

func (h *JSONWriter) reporter(args ...interface{}) reporter {
	return func(w http.ResponseWriter, r *http.Request, code int, err error) {
		if h.logger == nil {
			h.logger = logrus.StandardLogger()
			h.logger.Warning("No logger was set in JSONWriter, defaulting to standard logger.")
		}

		richError := assertRichError(err)
		h.logger.
			WithField("request-id", r.Header.Get("X-Request-ID")).
			WithField("writer", "JSON").
			WithField("trace", getErrorTrace(err)).
			WithField("code", code).
			WithField("reason", richError.Reason()).
			WithField("details", richError.Details()).
			WithField("status", richError.Status()).
			WithError(err).
			Error(args...)
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
		h.WriteError(w, r, errors.WithStack(err))
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

	h.WriteErrorCode(w, r, assertRichError(err).StatusCode(), err)
	return
}

// WriteErrorCode writes an error to ResponseWriter and forces an error code.
func (h *JSONWriter) WriteErrorCode(w http.ResponseWriter, r *http.Request, code int, err error) {
	richError := assertRichError(err)
	richError.setFallbackRequestID(r.Header.Get("X-Request-ID"))

	if code == 0 {
		code = richError.CodeField
	}

	// All errors land here, so it's a really good idea to do the logging here as well!
	h.Reporter("An error occurred while handling a request")(w, r, code, err)

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(&jsonError{Error: richError}); err != nil {
		// There was an error, but there's actually not a lot we can do except log that this happened.
		h.Reporter("Could not write jsonError to response writer")(w, r, code, errors.WithStack(err))
	}
}
