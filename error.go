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
	"net/http"

	"github.com/pkg/errors"
)

// StatusCodeCarrier can be implemented by an error to support setting status codes in the error itself.
type StatusCodeCarrier interface {
	// StatusCode returns the status code of this error.
	StatusCode() int
}

// ErrorContextCarrier can be implemented by an error to support error contexts.
type ErrorContextCarrier interface {
	error
	StatusCodeCarrier

	// RequestID returns the ID of the request that caused the error, if applicable.
	RequestID() string

	// Reason returns the reason for the error, if applicable.
	Reason() string

	// ID returns the error id, if applicable.
	Status() string

	// Details returns details on the error, if applicable.
	Details() []map[string]interface{}
}

type DefaultError struct {
	CodeField    int                      `json:"code,omitempty"`
	StatusField  string                   `json:"status,omitempty"`
	RIDField     string                   `json:"request,omitempty"`
	ReasonField  string                   `json:"reason,omitempty"`
	DetailsField []map[string]interface{} `json:"details,omitempty"`
	ErrorField   string                   `json:"message"`
}

func (e *DefaultError) Status() string {
	return e.StatusField
}

func (e *DefaultError) Error() string {
	return e.ErrorField
}

func (e *DefaultError) RequestID() string {
	return e.RIDField
}

func (e *DefaultError) Reason() string {
	return e.ReasonField
}

func (e *DefaultError) Details() []map[string]interface{} {
	return e.DetailsField
}

func (e *DefaultError) StatusCode() int {
	return e.CodeField
}

func (e *DefaultError) setFallbackRequestID(request string) {
	if e.RIDField != "" {
		return
	}

	e.RIDField = request
}

func assertRichError(err error) *DefaultError {
	if e, ok := errors.Cause(err).(ErrorContextCarrier); ok {
		return &DefaultError{
			CodeField:    e.StatusCode(),
			ReasonField:  e.Reason(),
			RIDField:     e.RequestID(),
			ErrorField:   err.Error(),
			DetailsField: e.Details(),
			StatusField:  e.Status(),
		}
	} else if e, ok := err.(ErrorContextCarrier); ok {
		return &DefaultError{
			CodeField:    e.StatusCode(),
			ReasonField:  e.Reason(),
			RIDField:     e.RequestID(),
			ErrorField:   err.Error(),
			DetailsField: e.Details(),
			StatusField:  e.Status(),
		}
	} else if e, ok := err.(StatusCodeCarrier); ok {
		return &DefaultError{
			CodeField:    e.StatusCode(),
			ErrorField:   err.Error(),
			DetailsField: []map[string]interface{}{},
		}
	} else if e, ok := errors.Cause(err).(StatusCodeCarrier); ok {
		return &DefaultError{
			CodeField:    e.StatusCode(),
			ErrorField:   err.Error(),
			DetailsField: []map[string]interface{}{},
		}
	}

	return &DefaultError{
		ErrorField:   err.Error(),
		CodeField:    http.StatusInternalServerError,
		DetailsField: []map[string]interface{}{},
	}
}
