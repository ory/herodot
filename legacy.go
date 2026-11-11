// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"encoding/json"
	stderr "errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"google.golang.org/grpc/codes"

	herodotv2 "github.com/ory/herodot/v2"
)

// LegacyDomain is the domain of errors that herodotv2.From derives from a
// DefaultError.
const LegacyDomain = "legacy.ory.com"

// Legacy returns outer with inner as its wrapped error. outer is otherwise
// unchanged, so legacy responses stay byte-equal. Only for the migration:
//
//	return herodot.Legacy(
//		herodot.ErrNotFound().WithReasonf("Unable to locate identity %s", id),
//		kratoserr.IdentityNotFound.New(herodotv2.Cause(err)),
//	)
//
// Code that inspects *DefaultError keeps seeing outer, while
// herodotv2.From and Definition.Matches find inner. An error previously
// wrapped by outer is replaced; pass it to inner with herodotv2.Cause.
func Legacy(outer *DefaultError, inner herodotv2.Error) *DefaultError {
	outer.err = inner
	return outer
}

var _ herodotv2.Classifier = (*DefaultError)(nil)

// HerodotError implements herodotv2.Classifier, so that herodotv2.From can
// classify a DefaultError. The error maps to the LegacyDomain domain:
//
//   - The code is GRPCCodeField if set, otherwise derived from CodeField.
//   - The reason is IDField in UPPER_SNAKE_CASE if present, otherwise the
//     code's name.
//   - The message is ErrorField, the metadata is DetailsField, and the cause
//     is e itself.
//
// herodotv2.From only consults it if the error's tree contains no
// herodotv2.Error.
func (e *DefaultError) HerodotError() (herodotv2.Error, bool) {
	if e == nil {
		return herodotv2.Error{}, false
	}
	code := e.legacyCode()
	def := herodotv2.Define(LegacyDomain, legacyReason(e.IDField, code), code, e.ErrorField)

	opts := []herodotv2.Option{herodotv2.Cause(e)}
	for k, v := range e.DetailsField {
		opts = append(opts, herodotv2.Metadata(k, metadataValue(v)))
	}
	return def.New(opts...), true
}

// httpStatusCodes maps HTTP statuses to codes for DefaultErrors without a gRPC
// code.
var httpStatusCodes = map[int]herodotv2.Code{
	http.StatusBadRequest:            herodotv2.CodeInvalidArgument,
	http.StatusUnauthorized:          herodotv2.CodeUnauthenticated,
	http.StatusForbidden:             herodotv2.CodePermissionDenied,
	http.StatusNotFound:              herodotv2.CodeNotFound,
	http.StatusConflict:              herodotv2.CodeAlreadyExists,
	http.StatusGone:                  herodotv2.CodeGone,
	http.StatusRequestEntityTooLarge: herodotv2.CodePayloadTooLarge,
	http.StatusUnsupportedMediaType:  herodotv2.CodeUnsupportedMediaType,
	http.StatusUnprocessableEntity:   herodotv2.CodeUnprocessable,
	http.StatusTooManyRequests:       herodotv2.CodeResourceExhausted,
	StatusClientClosedRequest:        herodotv2.CodeCancelled,
	http.StatusInternalServerError:   herodotv2.CodeInternal,
	http.StatusNotImplemented:        herodotv2.CodeUnimplemented,
	http.StatusBadGateway:            herodotv2.CodeBadGateway,
	http.StatusServiceUnavailable:    herodotv2.CodeUnavailable,
	http.StatusGatewayTimeout:        herodotv2.CodeDeadlineExceeded,
}

func (e *DefaultError) legacyCode() herodotv2.Code {
	if c := e.GRPCCodeField; c >= 1 && c <= 16 {
		return herodotv2.Code(c)
	}
	if c, ok := httpStatusCodes[e.CodeField]; ok {
		return c
	}
	if e.CodeField >= 400 && e.CodeField < 500 {
		return herodotv2.CodeFailedPrecondition
	}
	return herodotv2.CodeInternal
}

// legacyReason converts a legacy error ID such as "invalid_configuration" or
// "sessionInactive" to a reason. It falls back to the code's name if the ID is
// empty or can't be converted.
func legacyReason(id string, code herodotv2.Code) string {
	var b strings.Builder
	prev := '_'
	for _, r := range id {
		switch {
		case unicode.IsUpper(r) && unicode.IsLower(prev):
			b.WriteRune('_')
		case r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r)):
			if prev != '_' {
				b.WriteRune('_')
			}
			prev = '_'
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
		prev = r
	}
	if reason := strings.TrimRight(b.String(), "_"); herodotv2.ValidReason(reason) {
		return reason
	}
	return code.String()
}

func metadataValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprint(v)
}

// toV2Error returns the herodotv2.Error to render for err if err's tree
// contains one but no StatusCodeCarrier, such as a *DefaultError. Such
// errors only exist for reasons added during the migration; everything else
// keeps rendering as before.
func toV2Error(err error) (herodotv2.Error, bool) {
	if c := StatusCodeCarrier(nil); stderr.As(err, &c) {
		return herodotv2.Error{}, false
	}
	if e := (herodotv2.Error{}); !stderr.As(err, &e) {
		var p *herodotv2.Error
		if !stderr.As(err, &p) || p == nil {
			return herodotv2.Error{}, false
		}
	}
	return herodotv2.From(err), true
}

// fromV2Error renders e in the legacy shape: the ID is the reason in lower
// snake case, code and status come from the code, details hold the metadata,
// and there is no reason. The debug field holds the cause, which the
// JSONWriter only exposes when debugging is enabled.
func fromV2Error(e herodotv2.Error, err error, requestID string) *DefaultError {
	code := e.Definition().Code()
	de := &DefaultError{
		IDField:       strings.ToLower(e.Definition().Reason()),
		CodeField:     code.HTTPStatus(),
		StatusField:   statusText(code.HTTPStatus()),
		RIDField:      requestID,
		DetailsField:  map[string]interface{}{},
		ErrorField:    e.Error(),
		GRPCCodeField: grpcCode(code),
	}
	for k, v := range e.Metadata() {
		de.DetailsField[k] = v
	}
	if cause := e.Unwrap(); cause != nil {
		de.DebugField = cause.Error()
	}
	de.Wrap(err)
	return de
}

func statusText(code int) string {
	if code == StatusClientClosedRequest {
		return "Client Closed Request"
	}
	return http.StatusText(code)
}

func grpcCode(c herodotv2.Code) codes.Code {
	return codes.Code(c.Canonical())
}
