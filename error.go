// Copyright © 2022 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

// statusCodeCarrier can be implemented by an error to support setting http status codes in the error itself.
type statusCodeCarrier interface {
	// StatusCode returns the status code of this error.
	StatusCode() int
}
