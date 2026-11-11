// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

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

// Metadata adds an ErrorInfo metadata entry. Keys are lowerCamelCase and
// documented in the catalog. A later entry with the same key replaces an
// earlier one.
func Metadata(key, value string) Detail {
	return metadataDetail{key: key, value: value}
}

// Violation adds a request field violation.
func Violation(field, description string) Detail {
	return violationDetail{Field: field, Description: description}
}
