package herodot

import (
	"net/http"
)

// Writer is a helper to write arbitrary data to a ResponseWriter
type Writer interface {
	// Write a response object to the ResponseWriter with status code 200.
	Write(w http.ResponseWriter, r *http.Request, e interface{})

	// WriteCode writes a response object to the ResponseWriter and sets a response code.
	WriteCode(w http.ResponseWriter, r *http.Request, code int, e interface{})

	// WriteCreated writes a response object to the ResponseWriter with status code 201 and
	// the Location header set to location.
	WriteCreated(w http.ResponseWriter, r *http.Request, location string, e interface{})

	// WriteError writes an error to ResponseWriter and tries to extract the error's status code by
	// asserting StatusCodeCarrier. If the error does not implement StatusCodeCarrier, the status code
	// is set to 500.
	WriteError(w http.ResponseWriter, r *http.Request, err error)

	// WriteErrorCode writes an error to ResponseWriter and forces an error code.
	WriteErrorCode(w http.ResponseWriter, r *http.Request, code int, err error)
}
