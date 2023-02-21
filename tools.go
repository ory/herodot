// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

//go:build tools
// +build tools

package herodot

import (
	_ "github.com/jandelgado/gcov2lcov"
	_ "golang.org/x/tools/cmd/goimports"

	_ "github.com/ory/go-acc"
)
