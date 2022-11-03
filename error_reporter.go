// Copyright © 2022 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

type stdReporter struct{}

var _ ErrorReporter = (*stdReporter)(nil)

func (s *stdReporter) ReportError(r *http.Request, code int, err error, args ...interface{}) {
	fmt.Printf("ERROR: %s\n  Request: %v\n  Response Code: %d\n  Further Info: %v\n", err, r, code, args)
}
