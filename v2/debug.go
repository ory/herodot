// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"errors"
	"fmt"
	"reflect"
)

// DebugInfo is the debug information of an Error: its cause chain and a stack
// trace. It is the content of google.rpc.DebugInfo. Transports render it only
// when they are configured to expose debug information.
type DebugInfo struct {
	// StackEntries is the innermost stack trace found in the cause chain.
	StackEntries []string
	// Detail is the cause chain, formatted with %v.
	Detail string
}

// DebugInfo returns the debug information of e, or false if e has no cause.
//
// A stack trace is any error in the cause chain with a StackTrace method that
// returns a slice, as github.com/pkg/errors provides. Each frame is formatted
// with %+v.
func (e Error) DebugInfo() (DebugInfo, bool) {
	cause := e.Unwrap()
	if cause == nil {
		return DebugInfo{}, false
	}
	return DebugInfo{StackEntries: stackEntries(cause), Detail: fmt.Sprintf("%v", cause)}, true
}

// stackEntries returns the innermost stack trace in err's chain. It detects
// stack traces by method, without depending on a specific package.
func stackEntries(err error) (entries []string) {
	for ; err != nil; err = errors.Unwrap(err) {
		m := reflect.ValueOf(err).MethodByName("StackTrace")
		if !m.IsValid() || m.Type().NumIn() != 0 || m.Type().NumOut() != 1 || m.Type().Out(0).Kind() != reflect.Slice {
			continue
		}
		frames := m.Call(nil)[0]
		if frames.Len() == 0 {
			continue
		}
		entries = make([]string, frames.Len())
		for i := range entries {
			entries[i] = fmt.Sprintf("%+v", frames.Index(i).Interface())
		}
	}
	return entries
}
