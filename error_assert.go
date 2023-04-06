// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import "github.com/pkg/errors"

func coalesceError(e error) error {
	if e == nil {
		return errors.New("Error passed to WriteErrorCode is nil")
	}
	return e
}
