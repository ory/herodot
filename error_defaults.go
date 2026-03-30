// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

func ErrNotFound() *DefaultError {
	return &DefaultError{
		StatusField:   http.StatusText(http.StatusNotFound),
		ErrorField:    "The requested resource could not be found",
		CodeField:     http.StatusNotFound,
		GRPCCodeField: codes.NotFound,
	}
}

func ErrUnauthorized() *DefaultError {
	return &DefaultError{
		StatusField:   http.StatusText(http.StatusUnauthorized),
		ErrorField:    "The request could not be authorized",
		CodeField:     http.StatusUnauthorized,
		GRPCCodeField: codes.Unauthenticated,
	}
}

func ErrForbidden() *DefaultError {
	return &DefaultError{
		StatusField:   http.StatusText(http.StatusForbidden),
		ErrorField:    "The requested action was forbidden",
		CodeField:     http.StatusForbidden,
		GRPCCodeField: codes.PermissionDenied,
	}
}

func ErrInternalServerError() *DefaultError {
	return &DefaultError{
		StatusField:   http.StatusText(http.StatusInternalServerError),
		ErrorField:    "An internal server error occurred, please contact the system administrator",
		CodeField:     http.StatusInternalServerError,
		GRPCCodeField: codes.Internal,
	}
}

func ErrBadRequest() *DefaultError {
	return &DefaultError{
		StatusField:   http.StatusText(http.StatusBadRequest),
		ErrorField:    "The request was malformed or contained invalid parameters",
		CodeField:     http.StatusBadRequest,
		GRPCCodeField: codes.InvalidArgument,
	}
}

func ErrUnsupportedMediaType() *DefaultError {
	return &DefaultError{
		StatusField:   http.StatusText(http.StatusUnsupportedMediaType),
		ErrorField:    "The request is using an unknown content type",
		CodeField:     http.StatusUnsupportedMediaType,
		GRPCCodeField: codes.InvalidArgument,
	}
}

func ErrConflict() *DefaultError {
	return &DefaultError{
		StatusField:   http.StatusText(http.StatusConflict),
		ErrorField:    "The resource could not be created due to a conflict",
		CodeField:     http.StatusConflict,
		GRPCCodeField: codes.FailedPrecondition,
	}
}

func ErrMisconfiguration() *DefaultError {
	return &DefaultError{
		IDField:       "invalid_configuration",
		StatusField:   http.StatusText(http.StatusInternalServerError),
		ErrorField:    "Invalid configuration",
		ReasonField:   "One or more configuration values are invalid. Please report this to the system administrator.",
		CodeField:     http.StatusInternalServerError,
		GRPCCodeField: codes.Internal,
	}
}

func ErrUpstreamError() *DefaultError {
	return &DefaultError{
		IDField:     "upstream_error",
		StatusField: http.StatusText(http.StatusBadGateway),
		ErrorField:  "Upstream error",
		ReasonField: "An upstream server encountered an error or returned a malformed or unexpected response.",
		CodeField:   http.StatusBadGateway,
	}
}
