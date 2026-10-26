// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

// Package herodot defines errors that are identified by a domain and a reason,
// classified by a Code, and rendered by transports. It imports neither
// net/http nor gRPC.
//
// Reasons are declared in proto catalogs and turned into Definitions by
// protoc-gen-herodot. Code deep in the call stack decides the response by
// choosing a Definition:
//
//	return kratoserr.FlowExpired.New(herodot.Metadata("flowId", id))
//
// Transports, such as the httperr and grpcerr packages, classify any error with
// From and render it.
package herodot
