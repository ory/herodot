// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"sync"
	"testing"
)

// TestErrorFunctionsNoDataRace verifies that concurrent use of the error
// constructor functions does not cause data races. Each call must return an
// independent instance so that request-scoped mutations (e.g. WithDetail,
// WithReason) do not bleed between goroutines.
func TestErrorFunctionsNoDataRace(t *testing.T) {
	const goroutines = 200

	constructors := []func() *DefaultError{
		ErrNotFound,
		ErrUnauthorized,
		ErrForbidden,
		ErrInternalServerError,
		ErrBadRequest,
		ErrUnsupportedMediaType,
		ErrConflict,
		ErrMisconfiguration,
		ErrUpstreamError,
	}

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			fn := constructors[i%len(constructors)]
			err := fn().
				WithDetail("goroutine", i).
				WithReason("concurrent test")
			// Read back the details to exercise the map on both sides.
			_ = err.Details()
			_ = err.Reason()
		})
	}
	wg.Wait()
}
