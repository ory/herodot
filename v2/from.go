// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import "slices"

// Classifier lets error types this package doesn't know, such as herodot v1's
// DefaultError, classify themselves. Only From consults it; errors.Is and
// errors.As don't.
type Classifier interface {
	// HerodotError returns the classification, or false if there is none.
	HerodotError() (Error, bool)
}

type annotation struct {
	err     error
	details []Detail
}

func (a *annotation) Error() string { return a.err.Error() }
func (a *annotation) Unwrap() error { return a.err }

// Annotate wraps err with details, to add context above the place where the
// error was classified. The result keeps err's classification; From merges
// the details. The result unwraps to err, so errors.Is and errors.As see
// through it; they return the Error without the annotations' details.
//
// Annotate returns nil if err is nil, and err if there are no details.
func Annotate(err error, details ...Detail) error {
	if err == nil || len(details) == 0 {
		return err
	}
	return &annotation{err: err, details: details}
}

// From returns the Error that transports render for err:
//
//  1. the first Error or *Error in err's tree, searched like errors.As;
//  2. otherwise the classification of the first Classifier that has one;
//  3. otherwise Internal, with err as the cause.
//
// The result carries the details of the annotations on the path from err to
// the chosen error. On a metadata key conflict, the outer annotation wins;
// violations are appended. Annotations below the chosen error, in its cause,
// are ignored. In case 3, all annotations are dropped.
//
// From(nil) and a nil *Error return the zero Error.
func From(err error) Error {
	if err == nil {
		return Error{}
	}
	if e, path, ok := find(err, nil, asError); ok {
		return merge(e, path)
	}
	if e, path, ok := find(err, nil, classify); ok {
		return merge(e, path)
	}
	return Internal.New(Cause(err))
}

func asError(err error) (Error, bool) {
	switch e := err.(type) {
	case Error:
		return e, true
	case *Error:
		if e == nil {
			return Error{}, true
		}
		return *e, true
	case interface{ As(any) bool }:
		var t Error
		if e.As(&t) {
			return t, true
		}
		var p *Error
		if e.As(&p) {
			if p == nil {
				return Error{}, true
			}
			return *p, true
		}
	}
	return Error{}, false
}

func classify(err error) (Error, bool) {
	if c, ok := err.(Classifier); ok {
		return c.HerodotError()
	}
	return Error{}, false
}

// find walks err's tree depth-first in pre-order, as errors.As does, and
// returns the first match together with the annotations on the path to it,
// outermost first.
func find(err error, path []*annotation, match func(error) (Error, bool)) (Error, []*annotation, bool) {
	for err != nil {
		if e, ok := match(err); ok {
			return e, path, true
		}
		switch x := err.(type) {
		case *annotation:
			path = append(path, x)
			err = x.err
		case interface{ Unwrap() error }:
			err = x.Unwrap()
		case interface{ Unwrap() []error }:
			for _, child := range x.Unwrap() {
				if e, p, ok := find(child, slices.Clip(path), match); ok {
					return e, p, true
				}
			}
			return Error{}, nil, false
		default:
			return Error{}, nil, false
		}
	}
	return Error{}, nil, false
}

func merge(e Error, path []*annotation) Error {
	if len(path) == 0 {
		return e
	}
	src := e.d()
	data := &errorData{
		def:        src.def,
		cause:      src.cause,
		violations: slices.Clone(src.violations),
	}
	if len(src.metadata) > 0 {
		data.metadata = make(map[string]string, len(src.metadata))
		for k, v := range src.metadata {
			data.metadata[k] = v
		}
	}
	// Apply the innermost annotation first so that outer ones win.
	for i := len(path) - 1; i >= 0; i-- {
		for _, d := range path[i].details {
			d.apply(data)
		}
	}
	return Error{data: data}
}
