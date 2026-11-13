// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/ory/herodot/herodotpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/ory/herodot/v2"
	"github.com/ory/herodot/v2/grpcerr"
	"github.com/ory/herodot/v2/httperr"
)

func TestProtoDetails(t *testing.T) {
	t.Parallel()
	retry := &errdetails.RetryInfo{RetryDelay: durationpb.New(time.Second)}
	option := herodot.ProtoDetail(retry)
	retry.RetryDelay.Seconds = 99
	original := herodot.Define("keto.ory.com", "UNAVAILABLE", herodot.CodeUnavailable, "Service unavailable").New(option)
	got := original.ProtoDetails()
	require.Len(t, got, 1)
	require.EqualValues(t, 1, got[0].(*errdetails.RetryInfo).RetryDelay.Seconds)
	got[0].(*errdetails.RetryInfo).RetryDelay.Seconds = 42
	resource := &errdetails.ResourceInfo{ResourceType: "keto.ory.com/Namespace", ResourceName: "namespaces/example"}
	e := herodot.From(herodot.Annotate(original, herodot.ProtoDetail(resource)))
	require.Len(t, original.ProtoDetails(), 1)
	require.Len(t, e.ProtoDetails(), 2)
	for _, transport := range []string{"grpc", "http"} {
		t.Run(transport, func(t *testing.T) {
			t.Parallel()
			var decoded herodot.Error
			var ok bool
			if transport == "grpc" {
				decoded, ok = grpcerr.FromStatus(grpcerr.ToStatus(t.Context(), e, false))
			} else {
				body, err := json.Marshal(httperr.NewBody(e, httperr.Options{}))
				require.NoError(t, err)
				require.Contains(t, string(body), `"retryDelay":"1s"`)
				decoded, ok = httperr.DecodeBody(body)
			}
			require.True(t, ok)
			require.Len(t, decoded.ProtoDetails(), 2)
			for i, detail := range e.ProtoDetails() {
				require.True(t, proto.Equal(detail, decoded.ProtoDetails()[i]))
			}
		})
	}
}

func TestProtoDetailValidation(t *testing.T) {
	t.Parallel()
	for _, message := range []proto.Message{nil, (*errdetails.RetryInfo)(nil), &errdetails.ErrorInfo{}, &herodotpb.CodeInfo{}} {
		t.Run(fmt.Sprintf("%T", message), func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() { herodot.ProtoDetail(message) })
		})
	}
}

func TestProtoDetailRequestID(t *testing.T) {
	t.Parallel()
	e := herodot.Internal.New(herodot.ProtoDetail(&errdetails.RequestInfo{RequestId: "original"}))
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-request-id", "current"))
	s := grpcerr.ToStatus(ctx, e, false)
	require.Len(t, s.Details(), 2)
	require.Equal(t, "current", s.Details()[1].(*errdetails.RequestInfo).RequestId)
	body, err := json.Marshal(httperr.NewBody(e, httperr.Options{RequestID: "current"}))
	require.NoError(t, err)
	decoded, ok := httperr.DecodeBody(body)
	require.True(t, ok)
	require.Len(t, decoded.ProtoDetails(), 1)
	require.Equal(t, "current", decoded.ProtoDetails()[0].(*errdetails.RequestInfo).RequestId)
}

func TestProtoDetailUnknownHTTP(t *testing.T) {
	t.Parallel()
	for _, detail := range []string{
		`{"@type":"type.googleapis.com/example.Unknown","value":1}`,
		`{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"invalid"}`,
	} {
		body := `{"error":{"code":500,"status":"INTERNAL","message":"Internal server error","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","domain":"ory.com","reason":"INTERNAL"},` + detail + `]}}`
		decoded, ok := httperr.DecodeBody([]byte(body))
		require.True(t, ok)
		require.Empty(t, decoded.ProtoDetails())
	}
}

func TestProtoDetailMalformed(t *testing.T) {
	t.Parallel()
	e := herodot.Internal.New(herodot.ProtoDetail(&errdetails.ResourceInfo{ResourceName: string([]byte{0xff})}))
	s := grpcerr.ToStatus(t.Context(), e, false)
	require.Len(t, s.Details(), 1)
	require.IsType(t, &errdetails.ErrorInfo{}, s.Details()[0])
	body, err := json.Marshal(httperr.NewBody(e, httperr.Options{}))
	require.NoError(t, err)
	decoded, ok := httperr.DecodeBody(body)
	require.True(t, ok)
	require.Empty(t, decoded.ProtoDetails())
}

func TestProtoDetailDebugRendering(t *testing.T) {
	t.Parallel()
	debug := &errdetails.DebugInfo{Detail: "internal diagnostics", StackEntries: []string{"frame"}}
	e := herodot.Internal.New(herodot.ProtoDetail(debug))
	for _, transport := range []string{"grpc", "http"} {
		t.Run(transport, func(t *testing.T) {
			render := func(e herodot.Error, enabled bool) herodot.Error {
				var decoded herodot.Error
				var ok bool
				if transport == "grpc" {
					decoded, ok = grpcerr.FromStatus(grpcerr.ToStatus(t.Context(), e, enabled))
				} else {
					body, err := json.Marshal(httperr.NewBody(e, httperr.Options{Debug: enabled}))
					require.NoError(t, err)
					decoded, ok = httperr.DecodeBody(body)
				}
				require.True(t, ok)
				return decoded
			}
			require.Empty(t, render(e, false).ProtoDetails())
			decoded := render(e, true)
			require.Len(t, decoded.ProtoDetails(), 1)
			require.True(t, proto.Equal(debug, decoded.ProtoDetails()[0]))
			// Forwarding a decoded error must apply the local render policy.
			require.Empty(t, render(decoded, false).ProtoDetails())
		})
	}
}

func TestProtoDetailBadRequest(t *testing.T) {
	t.Parallel()
	detail := &errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: "name", Description: "required"}}}
	e := herodot.Internal.New(herodot.ProtoDetail(detail))
	require.True(t, proto.Equal(detail, e.ProtoDetails()[0]))
	grpcDecoded, ok := grpcerr.FromStatus(grpcerr.ToStatus(t.Context(), e, false))
	require.True(t, ok)
	require.Equal(t, []herodot.FieldViolation{{Field: "name", Description: "required"}}, grpcDecoded.Violations())
	body, err := json.Marshal(httperr.NewBody(e, httperr.Options{}))
	require.NoError(t, err)
	httpDecoded, ok := httperr.DecodeBody(body)
	require.True(t, ok)
	require.Equal(t, grpcDecoded.Violations(), httpDecoded.Violations())
}

func TestProtoDetailDebugAlternateTypeURL(t *testing.T) {
	t.Parallel()
	body := `{"error":{"code":500,"status":"INTERNAL","message":"Internal server error","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","domain":"ory.com","reason":"INTERNAL"},{"@type":"example.com/google.rpc.DebugInfo","detail":"remote diagnostics"}]}}`
	decoded, ok := httperr.DecodeBody([]byte(body))
	require.True(t, ok)
	require.Len(t, decoded.ProtoDetails(), 1)
	rendered, err := json.Marshal(httperr.NewBody(decoded, httperr.Options{}))
	require.NoError(t, err)
	require.NotContains(t, string(rendered), "remote diagnostics")
}
