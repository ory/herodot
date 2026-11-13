// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import "google.golang.org/protobuf/proto"

// Option configures an Error in Definition.New.
type Option interface {
	apply(*errorData)
}

// Detail is an Option that Annotate accepts as well. A detail adds information
// but never changes an error's identity, code, message or cause.
type Detail interface {
	Option
	detail()
}

type optionFunc func(*errorData)

func (f optionFunc) apply(d *errorData) { f(d) }

type metadataDetail struct{ key, value string }

func (m metadataDetail) apply(d *errorData) {
	if d.metadata == nil {
		d.metadata = make(map[string]string)
	}
	d.metadata[m.key] = m.value
}
func (metadataDetail) detail() {}

type violationDetail FieldViolation

func (v violationDetail) apply(d *errorData) {
	d.violations = append(d.violations, FieldViolation(v))
}
func (violationDetail) detail() {}

// Cause sets the wrapped error. It is internal: transports expose it only as
// debug output.
func Cause(err error) Option {
	return optionFunc(func(d *errorData) { d.cause = err })
}

// Metadata adds an ErrorInfo metadata entry. Keys are documented in the catalog.
// A later entry with the same key replaces the earlier one.
func Metadata(key, value string) Detail {
	return metadataDetail{key: key, value: value}
}

// Violation adds a request field violation.
func Violation(field, description string) Detail {
	return violationDetail{Field: field, Description: description}
}

type protoDetail struct{ message proto.Message }

func (p protoDetail) apply(d *errorData) {
	d.protoDetails = append(d.protoDetails, proto.Clone(p.message))
}
func (protoDetail) detail() {}

// ProtoDetail adds a protobuf detail and copies its message. It panics for nil
// messages or the identity and code types managed by Herodot: ErrorInfo and CodeInfo.
// DebugInfo is rendered only when debug output is enabled. All other details
// are public response data and must not contain internal diagnostics.
func ProtoDetail(message proto.Message) Detail {
	if message == nil || !message.ProtoReflect().IsValid() {
		panic("herodot: nil protobuf detail")
	}
	switch message.ProtoReflect().Descriptor().FullName() {
	case "google.rpc.ErrorInfo", "ory.herodot.v1.CodeInfo":
		panic("herodot: protobuf detail type is managed by Herodot")
	}
	return protoDetail{message: proto.Clone(message)}
}
