// Copyright © 2022 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"net/http"
)

// Writer is a helper to write arbitrary data to a ResponseWriter
type Writer interface {
	// Write a response object to the ResponseWriter with status code 200.
	Write(w http.ResponseWriter, r *http.Request, e interface{}, opts ...EncoderOptions)

	// WriteCode writes a response object to the ResponseWriter and sets a response code.
	WriteCode(w http.ResponseWriter, r *http.Request, code int, e interface{}, opts ...EncoderOptions)

	// WriteCreated writes a response object to the ResponseWriter with status code 201 and
	// the Location header set to location.
	WriteCreated(w http.ResponseWriter, r *http.Request, location string, e interface{})

	// WriteError writes an error to ResponseWriter and tries to extract the error's status code by
	// asserting statusCodeCarrier. If the error does not implement statusCodeCarrier, the status code
	// is set to 500.
	WriteError(w http.ResponseWriter, r *http.Request, err error, opts ...Option)

	// WriteErrorCode writes an error to ResponseWriter and forces an error code.
	WriteErrorCode(w http.ResponseWriter, r *http.Request, code int, err error, opts ...Option)
}
