// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
)

// Error is an occurrence of a Definition. It is immutable: its only field
// points to data that is never modified after creation. Copies are cheap,
// sharing is safe, and Error is comparable. The zero Error behaves like
// Internal.New().
type Error struct {
	data *errorData
}

type errorData struct {
	def          Definition
	metadata     map[string]string
	cause        error
	badRequest   *errdetails.BadRequest
	protoDetails []proto.Message
}

var zeroData = &errorData{def: Internal}

func (e Error) d() *errorData {
	if e.data == nil {
		return zeroData
	}
	return e.data
}

// Definition returns the definition the error is an occurrence of.
func (e Error) Definition() Definition { return e.d().def }

// Error returns the message of the definition.
func (e Error) Error() string { return e.d().def.message }

// Metadata returns a copy of the ErrorInfo metadata. It is nil if there is
// none.
func (e Error) Metadata() map[string]string { return maps.Clone(e.d().metadata) }

// ProtoDetails returns deep copies of the additional protobuf details.
// The merged BadRequest, if present, comes first, followed by other details in
// attachment order.
func (e Error) ProtoDetails() []proto.Message {
	var details []proto.Message
	if e.d().badRequest != nil {
		details = append(details, proto.Clone(e.d().badRequest))
	}
	for _, d := range e.d().protoDetails {
		details = append(details, proto.Clone(d))
	}
	return details
}

// Unwrap returns the cause.
func (e Error) Unwrap() error { return e.d().cause }

// Is reports whether target is an Error or non-nil *Error with the same identity (domain and
// reason), so that errors.Is works between Errors. The code, messages, details
// and cause are ignored.
// errors.As(err, &e) with var e herodot.Error needs no method.
func (e Error) Is(target error) bool {
	switch t := target.(type) {
	case Error:
		return t.Definition().SameIdentity(e.Definition())
	case *Error:
		return t != nil && t.Definition().SameIdentity(e.Definition())
	default:
		return false
	}
}

// Format implements fmt.Formatter. %s and %v print the message, %q the quoted
// message. %+v prints the identity, metadata, violations and the cause with
// %+v, which includes stack traces of causes that carry one.
func (e Error) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			d := e.d()
			_, _ = fmt.Fprintf(s, "%s/%s (%s): %s", d.def.domain, d.def.reason, d.def.code, d.def.message)
			keys := slices.Collect(maps.Keys(d.metadata))
			sort.Strings(keys)
			for _, k := range keys {
				_, _ = fmt.Fprintf(s, "\nmetadata %s=%s", k, d.metadata[k])
			}
			for _, v := range d.badRequest.GetFieldViolations() {
				_, _ = fmt.Fprintf(s, "\nviolation %s: %s", v.GetField(), v.GetDescription())
			}
			if d.cause != nil {
				_, _ = fmt.Fprintf(s, "\ncause: %+v", d.cause)
			}
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}
