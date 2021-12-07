package herodot

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

var ErrNotFound = DefaultError{
	StatusField:    http.StatusText(http.StatusNotFound),
	ErrorField:     "The requested resource could not be found",
	CodeField:      http.StatusNotFound,
	TemporaryField: false,
	GRPCCodeField:  codes.NotFound,
}

var ErrUnauthorized = DefaultError{
	StatusField:    http.StatusText(http.StatusUnauthorized),
	ErrorField:     "The request could not be authorized",
	CodeField:      http.StatusUnauthorized,
	TemporaryField: false,
	GRPCCodeField:  codes.Unauthenticated,
}

var ErrForbidden = DefaultError{
	StatusField:    http.StatusText(http.StatusForbidden),
	ErrorField:     "The requested action was forbidden",
	CodeField:      http.StatusForbidden,
	TemporaryField: false,
	GRPCCodeField:  codes.PermissionDenied,
}

var ErrInternalServerError = DefaultError{
	StatusField:    http.StatusText(http.StatusInternalServerError),
	ErrorField:     "An internal server error occurred, please contact the system administrator",
	CodeField:      http.StatusInternalServerError,
	TemporaryField: true,
	GRPCCodeField:  codes.Internal,
}

var ErrBadRequest = DefaultError{
	StatusField:    http.StatusText(http.StatusBadRequest),
	ErrorField:     "The request was malformed or contained invalid parameters",
	CodeField:      http.StatusBadRequest,
	TemporaryField: false,
	GRPCCodeField:  codes.FailedPrecondition,
}

var ErrUnsupportedMediaType = DefaultError{
	StatusField:    http.StatusText(http.StatusUnsupportedMediaType),
	ErrorField:     "The request is using an unknown content type",
	CodeField:      http.StatusUnsupportedMediaType,
	TemporaryField: false,
	GRPCCodeField:  codes.InvalidArgument,
}

var ErrConflict = DefaultError{
	StatusField:    http.StatusText(http.StatusConflict),
	ErrorField:     "The resource could not be created due to a conflict",
	CodeField:      http.StatusConflict,
	TemporaryField: false,
	GRPCCodeField:  codes.FailedPrecondition,
}
