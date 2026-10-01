// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type grpcStatusError interface {
	error
	GRPCStatus() *status.Status
}

func unwrapGRPCStatusError(err error) error {
	if err == nil {
		return nil
	}

	var innerErr grpcStatusError
	if errors.As(err, &innerErr) {
		return innerErr
	}
	return err
}

// UnaryErrorUnwrapInterceptor is a gRPC server-side interceptor that unwraps herodot errors for Unary RPCs.
//
// Deprecated: Since grpc-go v1.55.0 (PR #6031 resolving issue #2934), gRPC natively unwraps errors
// implementing GRPCStatus() via errors.As. This interceptor is retained for backward compatibility.
func UnaryErrorUnwrapInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	return resp, unwrapGRPCStatusError(err)
}

// StreamErrorUnwrapInterceptor is a gRPC server-side interceptor that unwraps herodot errors for Streaming RPCs.
//
// Deprecated: Since grpc-go v1.55.0 (PR #6031 resolving issue #2934), gRPC natively unwraps errors
// implementing GRPCStatus() via errors.As. This interceptor is retained for backward compatibility.
func StreamErrorUnwrapInterceptor(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := handler(srv, ss)
	return unwrapGRPCStatusError(err)
}
