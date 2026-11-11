// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"errors"
	"fmt"
	"regexp"
)

// Definition identifies an error by domain and reason and fixes its code and
// message. It is an immutable value. Catalogs generate one
// package-level Definition per reason.
//
// The identity of a Definition is its domain and reason. Compare Definitions
// with SameIdentity, not ==: == also compares the code and the default
// message, which can differ between versions of a catalog.
//
// Definition deliberately doesn't implement error, so it can't be returned in
// place of an Error.
type Definition struct {
	domain, reason string
	code           Code
	message        string
}

var (
	domainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	reasonPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)
)

// MaxReasonLength is the maximum length of a reason.
const MaxReasonLength = 63

// ValidDomain reports whether domain is a valid error domain, i.e. a
// lowercase DNS name with at least two labels such as "kratos.ory.com".
func ValidDomain(domain string) bool {
	return len(domain) <= 253 && domainPattern.MatchString(domain)
}

// ValidReason reports whether reason is a valid UPPER_SNAKE_CASE reason of at
// most MaxReasonLength characters.
func ValidReason(reason string) bool {
	return len(reason) <= MaxReasonLength && reasonPattern.MatchString(reason)
}

// Define returns a Definition. Generated code calls it. It panics on an invalid
// domain, reason or code, which codegen has already ruled out.
func Define(domain, reason string, code Code, message string) Definition {
	d, err := NewDefinition(domain, reason, code, message)
	if err != nil {
		panic(err)
	}
	return d
}

// NewDefinition returns a Definition, or an error if the domain, reason or
// code is invalid. It is for decoders, which build definitions from untrusted
// input such as a received error response. Services use generated catalog
// definitions instead.
func NewDefinition(domain, reason string, code Code, message string) (Definition, error) {
	if !ValidDomain(domain) {
		return Definition{}, fmt.Errorf("herodot: invalid domain %q", domain)
	}
	if !ValidReason(reason) {
		return Definition{}, fmt.Errorf("herodot: invalid reason %q", reason)
	}
	if !code.valid() {
		return Definition{}, fmt.Errorf("herodot: invalid code %d for %s/%s", code, domain, reason)
	}
	return Definition{domain: domain, reason: reason, code: code, message: message}, nil
}

// Internal is ory.com/INTERNAL, the classification of errors that have none.
// The OSS shared catalog declares the same entry.
var Internal = Define("ory.com", "INTERNAL", CodeInternal,
	"An internal server error occurred. Please contact the system administrator.")

// Domain returns the logical owner of the reason, e.g. "kratos.ory.com".
func (d Definition) Domain() string { return d.orInternal().domain }

// Reason returns the UPPER_SNAKE_CASE reason, unique within the domain.
func (d Definition) Reason() string { return d.orInternal().reason }

// Code returns the classification that determines the transport statuses.
func (d Definition) Code() Code { return d.orInternal().code }

// Message returns the message. It is the same for every occurrence: values
// that vary go into metadata.
func (d Definition) Message() string { return d.orInternal().message }

// orInternal makes the zero Definition behave like Internal.
func (d Definition) orInternal() Definition {
	if d == (Definition{}) {
		return Internal
	}
	return d
}

// New returns an occurrence of the error. Choosing the definition is how code
// deep in the call stack decides the response.
func (d Definition) New(opts ...Option) Error {
	data := &errorData{def: d.orInternal()}
	for _, o := range opts {
		o.apply(data)
	}
	return Error{data: data}
}

// SameIdentity reports whether d and o have the same domain and reason.
func (d Definition) SameIdentity(o Definition) bool {
	d, o = d.orInternal(), o.orInternal()
	return d.domain == o.domain && d.reason == o.reason
}

// Matches reports whether err's tree contains an Error with d's identity.
// Without Matches, the stdlib equivalent for the first Error in the tree is:
//
//	var e herodot.Error
//	errors.As(err, &e) && e.Definition().SameIdentity(kratoserr.FlowExpired)
//
// Matches considers every Error in the tree, not only the first one.
func (d Definition) Matches(err error) bool {
	return errors.Is(err, Error{data: &errorData{def: d.orInternal()}})
}
