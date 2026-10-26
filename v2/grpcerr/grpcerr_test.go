// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package grpcerr_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/ory/herodot/v2"
	"github.com/ory/herodot/v2/grpcerr"
	"github.com/ory/herodot/v2/internal/herodotpb"
)

var flowExpired = herodot.Define("kratos.ory.com", "FLOW_EXPIRED", herodot.CodeExpired, "The self-service flow has expired.")

func TestToStatus(t *testing.T) {
	assert.Nil(t, grpcerr.ToStatus(context.Background(), nil, false))

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "rid-1"))
	err := flowExpired.New(
		herodot.Metadata("flowId", "1234"),
		herodot.Violation("email", "is required"),
		herodot.Cause(errors.New("root cause")),
	)

	t.Run("case=without debug", func(t *testing.T) {
		s := grpcerr.ToStatus(ctx, err, false)
		assert.Equal(t, codes.FailedPrecondition, s.Code())
		assert.Equal(t, "The self-service flow has expired.", s.Message())
		assertDetails(t, s, []proto.Message{
			&errdetails.ErrorInfo{Reason: "FLOW_EXPIRED", Domain: "kratos.ory.com", Metadata: map[string]string{"flowId": "1234"}},
			&herodotpb.CodeInfo{Code: herodotpb.Code_EXPIRED},
			&errdetails.RequestInfo{RequestId: "rid-1"},
			&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: "email", Description: "is required"}}},
		})
	})

	t.Run("case=with debug", func(t *testing.T) {
		s := grpcerr.ToStatus(context.Background(), err, true)
		assertDetails(t, s, []proto.Message{
			&errdetails.ErrorInfo{Reason: "FLOW_EXPIRED", Domain: "kratos.ory.com", Metadata: map[string]string{"flowId": "1234"}},
			&herodotpb.CodeInfo{Code: herodotpb.Code_EXPIRED},
			&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: "email", Description: "is required"}}},
			&errdetails.DebugInfo{Detail: "root cause"},
		})
	})

	t.Run("case=unclassified", func(t *testing.T) {
		s := grpcerr.ToStatus(context.Background(), errors.New("secret"), false)
		assert.Equal(t, codes.Internal, s.Code())
		assert.Equal(t, herodot.Internal.Message(), s.Message())
	})
}

func TestFromStatus(t *testing.T) {
	// The sender's catalog words the message differently than the receiver's.
	reworded := herodot.Define("kratos.ory.com", "FLOW_EXPIRED", herodot.CodeExpired, "The flow has expired. Start a new flow.")
	sent := reworded.New(
		herodot.Metadata("flowId", "1234"),
		herodot.Violation("email", "is required"),
	)
	s := grpcerr.ToStatus(context.Background(), sent, false)

	t.Run("case=round trip", func(t *testing.T) {
		received, ok := grpcerr.FromError(fmt.Errorf("calling kratos: %w", s.Err()))
		require.True(t, ok)
		assert.True(t, flowExpired.Matches(received))
		assert.Equal(t, herodot.CodeExpired, received.Definition().Code(), "the refinement survives gRPC")
		assert.Equal(t, "The flow has expired. Start a new flow.", received.Error(), "the received message is kept")
		assert.Equal(t, map[string]string{"flowId": "1234"}, received.Metadata())
		assert.Equal(t, []herodot.FieldViolation{{Field: "email", Description: "is required"}}, received.Violations())
		assert.Equal(t, s.Err().Error(), received.Unwrap().Error())

		forwarded := grpcerr.ToStatus(context.Background(), fmt.Errorf("forwarding: %w", received), false)
		assert.True(t, proto.Equal(s.Proto(), forwarded.Proto()), "forwarding doesn't change the status")
	})

	t.Run("case=canonical code", func(t *testing.T) {
		notFound := herodot.Define("kratos.ory.com", "IDENTITY_NOT_FOUND", herodot.CodeNotFound, "The identity could not be found.")
		received, ok := grpcerr.FromStatus(grpcerr.ToStatus(context.Background(), notFound.New(), false))
		require.True(t, ok)
		assert.Equal(t, herodot.CodeNotFound, received.Definition().Code())
	})

	t.Run("case=contradicting CodeInfo", func(t *testing.T) {
		s, err := status.New(codes.NotFound, "msg").WithDetails(
			&errdetails.ErrorInfo{Reason: "FLOW_EXPIRED", Domain: "kratos.ory.com"},
			&herodotpb.CodeInfo{Code: herodotpb.Code_EXPIRED},
		)
		require.NoError(t, err)
		received, ok := grpcerr.FromStatus(s)
		require.True(t, ok)
		assert.Equal(t, herodot.CodeNotFound, received.Definition().Code())
	})

	for _, tc := range []struct {
		name string
		s    *status.Status
	}{
		{"nil", nil},
		{"ok", status.New(codes.OK, "")},
		{"no ErrorInfo", status.New(codes.NotFound, "not found")},
		{"invalid code", status.New(codes.Code(42), "")},
		{"invalid reason", must(status.New(codes.NotFound, "").WithDetails(&errdetails.ErrorInfo{Reason: "not found", Domain: "kratos.ory.com"}))},
		{"invalid domain", must(status.New(codes.NotFound, "").WithDetails(&errdetails.ErrorInfo{Reason: "NOT_FOUND"}))},
	} {
		t.Run("case=not a herodot error/"+tc.name, func(t *testing.T) {
			_, ok := grpcerr.FromStatus(tc.s)
			assert.False(t, ok)
		})
	}

	_, ok := grpcerr.FromError(errors.New("not a status"))
	assert.False(t, ok)
	_, ok = grpcerr.FromError(nil)
	assert.False(t, ok)
}

func must(s *status.Status, err error) *status.Status {
	if err != nil {
		panic(err)
	}
	return s
}

func assertDetails(t *testing.T, s *status.Status, expected []proto.Message) {
	t.Helper()
	actual := s.Details()
	require.Len(t, actual, len(expected))
	for i, d := range actual {
		m, ok := d.(proto.Message)
		require.True(t, ok, "%T", d)
		assert.True(t, proto.Equal(expected[i], m), "detail %d: expected %v, got %v", i, expected[i], m)
	}
}

func TestInterceptors(t *testing.T) {
	handlerErr := flowExpired.New()

	_, err := grpcerr.UnaryServerInterceptor(false)(context.Background(), nil, &grpc.UnaryServerInfo{},
		func(context.Context, any) (any, error) { return nil, handlerErr })
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	_, err = grpcerr.UnaryServerInterceptor(false)(context.Background(), nil, &grpc.UnaryServerInfo{},
		func(context.Context, any) (any, error) { return "ok", nil })
	assert.NoError(t, err)

	err = grpcerr.StreamServerInterceptor(false)(nil, stream{}, &grpc.StreamServerInfo{},
		func(any, grpc.ServerStream) error { return handlerErr })
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

type stream struct{ grpc.ServerStream }

func (stream) Context() context.Context { return context.Background() }

type malformedCause struct{}

func (malformedCause) Error() string        { return "cause\xff" }
func (malformedCause) StackTrace() []string { return []string{"frame\xff"} }

func TestToStatusInvalidUTF8(t *testing.T) {
	def := herodot.Define("ory.com", "INVALID_INPUT", herodot.CodeInvalidArgument, "Message\xff")
	e := def.New(
		herodot.Metadata("key\xff", "value\xff"),
		herodot.Violation("field\xff", "description\xff"),
		herodot.Cause(malformedCause{}),
	)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "request\xff"))
	s := grpcerr.ToStatus(ctx, e, true)
	assert.Equal(t, codes.InvalidArgument, s.Code())
	assert.Equal(t, "Message\uFFFD", s.Message())
	assertDetails(t, s, []proto.Message{
		&errdetails.ErrorInfo{Reason: "INVALID_INPUT", Domain: "ory.com", Metadata: map[string]string{"key\uFFFD": "value\uFFFD"}},
		&errdetails.RequestInfo{RequestId: "request\uFFFD"},
		&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: "field\uFFFD", Description: "description\uFFFD"}}},
		&errdetails.DebugInfo{Detail: "cause\uFFFD", StackEntries: []string{"frame\uFFFD"}},
	})
	_, err := proto.Marshal(s.Proto())
	require.NoError(t, err, "the complete status must be serializable")
	assert.Equal(t, map[string]string{"key\xff": "value\xff"}, e.Metadata(), "rendering must not modify the error")
}
