// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import "strconv"

//go:generate env GOWORK=off go tool -modfile=tools/go.mod buf generate

// Code classifies an error independently of the transport. It is generated
// from ory/herodot/v1/herodot.proto, and the set is closed.
//
// Canonical codes (1-16) equal google.rpc.Code. Refinements, numbered from
// 100, map to exactly one canonical code and exactly one HTTP status.
type Code int32

type codeInfo struct {
	name       string
	canonical  Code
	httpStatus int
}

func (c Code) valid() bool {
	_, ok := codeTable[c]
	return ok
}

// Canonical returns the google.rpc.Code that c maps to. A canonical code
// returns itself. An invalid code returns CodeUnknown.
func (c Code) Canonical() Code {
	if i, ok := codeTable[c]; ok {
		return i.canonical
	}
	return CodeUnknown
}

// HTTPStatus returns the HTTP status that c maps to. An invalid code returns
// 500.
func (c Code) HTTPStatus() int {
	if i, ok := codeTable[c]; ok {
		return i.httpStatus
	}
	return 500
}

// String returns the code's name, e.g. "EXPIRED".
func (c Code) String() string {
	if i, ok := codeTable[c]; ok {
		return i.name
	}
	return "CODE(" + strconv.Itoa(int(c)) + ")"
}

// IsCanonical reports whether c is one of the canonical codes 1-16.
func (c Code) IsCanonical() bool { return c.valid() && c.Canonical() == c }

// LookupCode returns the code that maps to the canonical code and the HTTP
// status, which is unique. It is for decoders: an HTTP error response carries
// only these two. It returns false if canonical isn't a canonical code or no
// code maps to the pair.
func LookupCode(canonical Code, httpStatus int) (Code, bool) {
	if !canonical.IsCanonical() {
		return 0, false
	}
	for c, i := range codeTable {
		if i.canonical == canonical && i.httpStatus == httpStatus {
			return c, true
		}
	}
	return 0, false
}

// CanonicalCode returns the canonical code with the name, e.g.
// "FAILED_PRECONDITION" as in the "status" field of an AIP-193 response. It
// returns false for names of refinements and unknown names.
func CanonicalCode(name string) (Code, bool) {
	for c, i := range codeTable {
		if i.name == name && c.IsCanonical() {
			return c, true
		}
	}
	return 0, false
}
