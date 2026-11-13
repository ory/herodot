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
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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
	detail := &errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{
		Field: "name", Description: "required", Reason: "MISSING_NAME",
		LocalizedMessage: &errdetails.LocalizedMessage{Locale: "de", Message: "Name erforderlich"},
	}}}
	e := herodot.Internal.New(herodot.ProtoDetail(detail))
	require.True(t, proto.Equal(detail, e.ProtoDetails()[0]))
	grpcDecoded, ok := grpcerr.FromStatus(grpcerr.ToStatus(t.Context(), e, false))
	require.True(t, ok)
	require.True(t, proto.Equal(detail, grpcDecoded.ProtoDetails()[0]))
	body, err := json.Marshal(httperr.NewBody(e, httperr.Options{}))
	require.NoError(t, err)
	httpDecoded, ok := httperr.DecodeBody(body)
	require.True(t, ok)
	require.True(t, proto.Equal(grpcDecoded.ProtoDetails()[0], httpDecoded.ProtoDetails()[0]))
	for _, decoded := range []herodot.Error{grpcDecoded, httpDecoded} {
		require.Len(t, decoded.ProtoDetails(), 1)
		require.True(t, proto.Equal(detail, decoded.ProtoDetails()[0]))
		require.Len(t, grpcerr.ToStatus(t.Context(), decoded, false).Details(), 2)
		forwarded, err := json.Marshal(httperr.NewBody(decoded, httperr.Options{}))
		require.NoError(t, err)
		roundtrip, ok := httperr.DecodeBody(forwarded)
		require.True(t, ok)
		require.Len(t, roundtrip.ProtoDetails(), 1)
		require.True(t, proto.Equal(detail, roundtrip.ProtoDetails()[0]))
	}
	// Convenience violations and full messages share one storage path.
	annotated := herodot.From(herodot.Annotate(e, herodot.Violation("email", "required")))
	require.Len(t, e.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations, 1)
	require.Len(t, annotated.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations, 2)
	require.Len(t, annotated.ProtoDetails(), 1)
	dynamic := dynamicpb.NewMessage(detail.ProtoReflect().Descriptor())
	proto.Merge(dynamic, detail)
	require.True(t, proto.Equal(e.ProtoDetails()[0], herodot.Internal.New(herodot.ProtoDetail(dynamic)).ProtoDetails()[0]))
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

func TestProtoDetailDynamicRequestID(t *testing.T) {
	t.Parallel()
	detail := dynamicpb.NewMessage((&errdetails.RequestInfo{}).ProtoReflect().Descriptor())
	detail.Set(detail.Descriptor().Fields().ByName("request_id"), protoreflect.ValueOfString("attached"))
	e := herodot.Internal.New(herodot.ProtoDetail(detail))
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-request-id", "transport"))
	s := grpcerr.ToStatus(ctx, e, false)
	require.Len(t, s.Details(), 2)
	require.Equal(t, "transport", s.Details()[1].(*errdetails.RequestInfo).RequestId)
	body, err := json.Marshal(httperr.NewBody(e, httperr.Options{RequestID: "transport"}))
	require.NoError(t, err)
	decoded, ok := httperr.DecodeBody(body)
	require.True(t, ok)
	require.Len(t, decoded.ProtoDetails(), 1)
	require.Equal(t, "transport", decoded.ProtoDetails()[0].(*errdetails.RequestInfo).RequestId)
}

// Model a registered message implementing only the protobuf v2 API.
// Override reflection's constructors so cloning and unmarshalling retain that API.
type v2OnlyDetail struct{ reflection v2OnlyReflection }

func (m *v2OnlyDetail) ProtoReflect() protoreflect.Message { return m.reflection }

type v2OnlyReflection struct {
	protoreflect.Message
	owner *v2OnlyDetail
	kind  v2OnlyType
}

func (m v2OnlyReflection) Interface() protoreflect.ProtoMessage { return m.owner }
func (m v2OnlyReflection) Type() protoreflect.MessageType       { return m.kind }
func (m v2OnlyReflection) New() protoreflect.Message            { return m.kind.New() }

type v2OnlyType struct {
	descriptor protoreflect.MessageDescriptor
}

func (t v2OnlyType) Descriptor() protoreflect.MessageDescriptor { return t.descriptor }
func (t v2OnlyType) New() protoreflect.Message {
	m := &v2OnlyDetail{}
	m.reflection = v2OnlyReflection{Message: dynamicpb.NewMessage(t.descriptor), owner: m, kind: t}
	return m.reflection
}
func (t v2OnlyType) Zero() protoreflect.Message { return t.New() }

func TestProtoDetailV2OnlyGRPC(t *testing.T) {
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("herodot_test_v2_only.proto"), Package: proto.String("herodot.test"), Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("V2OnlyDetail"), Field: []*descriptorpb.FieldDescriptorProto{{
			Name: proto.String("value"), Number: proto.Int32(1),
			Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		}}}},
	}, nil)
	require.NoError(t, err)
	kind := v2OnlyType{descriptor: file.Messages().Get(0)}
	// Permit repeated test runs in the same process.
	if _, err := protoregistry.GlobalTypes.FindMessageByName(kind.Descriptor().FullName()); err != nil {
		require.NoError(t, protoregistry.GlobalTypes.RegisterMessage(kind))
	}
	detail := kind.New()
	detail.Set(kind.Descriptor().Fields().ByName("value"), protoreflect.ValueOfString("retained"))
	e := herodot.Internal.New(herodot.ProtoDetail(detail.Interface()))
	s := grpcerr.ToStatus(t.Context(), e, false)
	require.Len(t, s.Details(), 2)
	_, implementsV2 := s.Details()[1].(protoadapt.MessageV2)
	require.False(t, implementsV2, "gRPC returns a v1 wrapper for v2-only messages")
	decoded, ok := grpcerr.FromStatus(s)
	require.True(t, ok)
	require.Len(t, decoded.ProtoDetails(), 1)
	require.True(t, proto.Equal(detail.Interface(), decoded.ProtoDetails()[0]))
}

func TestBadRequestMerge(t *testing.T) {
	t.Parallel()
	first := &errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{
		Field: "name", Description: "required", Reason: "MISSING",
		LocalizedMessage: &errdetails.LocalizedMessage{Locale: "de", Message: "Name erforderlich"},
	}}}
	// Unknown varint fields at message and violation level must survive merging.
	first.ProtoReflect().SetUnknown([]byte{0x50, 1})
	first.FieldViolations[0].ProtoReflect().SetUnknown([]byte{0x50, 2})
	second := dynamicpb.NewMessage(first.ProtoReflect().Descriptor())
	proto.Merge(second, first)
	second.SetUnknown([]byte{0x58, 3})
	original := herodot.Internal.New(herodot.ProtoDetail(first))
	merged := herodot.From(herodot.Annotate(original,
		herodot.Violation("email", "required"), herodot.ProtoDetail(second),
		herodot.ProtoDetail(&errdetails.RetryInfo{}),
	))
	require.Len(t, original.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations, 1)
	require.Equal(t, "email", merged.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations[1].Field)
	require.Equal(t, "required", merged.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations[1].Description)
	details := merged.ProtoDetails()
	require.Len(t, details, 2)
	badRequest := details[0].(*errdetails.BadRequest)
	require.IsType(t, &errdetails.RetryInfo{}, details[1])
	require.Len(t, badRequest.FieldViolations, 3)
	require.True(t, proto.Equal(first.FieldViolations[0], badRequest.FieldViolations[0]))
	require.True(t, proto.Equal(first.FieldViolations[0], badRequest.FieldViolations[2]))
	require.Equal(t, []byte{0x50, 1, 0x58, 3}, []byte(badRequest.ProtoReflect().GetUnknown()))
	badRequest.FieldViolations[0].LocalizedMessage.Message = "mutated"
	first.FieldViolations[0].Reason = "mutated"
	require.Equal(t, "Name erforderlich", merged.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations[0].LocalizedMessage.Message)
	require.Equal(t, "MISSING", original.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations[0].Reason)

	// Multiple wire-level BadRequest messages normalize through both decoders.
	wireFirst := original.ProtoDetails()[0].(*errdetails.BadRequest)
	wireSecond := &errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: "email", Description: "required"}}}
	s, err := grpcerr.ToStatus(t.Context(), original, false).WithDetails(wireSecond)
	require.NoError(t, err)
	decoded, ok := grpcerr.FromStatus(s)
	require.True(t, ok)
	require.Len(t, decoded.ProtoDetails(), 1)
	expected := proto.Clone(wireFirst).(*errdetails.BadRequest)
	proto.Merge(expected, wireSecond)
	require.True(t, proto.Equal(expected, decoded.ProtoDetails()[0]))
	require.Len(t, grpcerr.ToStatus(t.Context(), decoded, false).Details(), 2)

	body := httperr.NewBody(original, httperr.Options{})
	other := httperr.NewBody(herodot.Internal.New(herodot.ProtoDetail(wireSecond)), httperr.Options{})
	body.Error.Details = append(body.Error.Details, other.Error.Details[1])
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	decodedHTTP, ok := httperr.DecodeBody(raw)
	require.True(t, ok)
	require.Len(t, decodedHTTP.ProtoDetails(), 1)
	require.Len(t, decodedHTTP.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations, 2)
	require.Equal(t, "MISSING", decodedHTTP.ProtoDetails()[0].(*errdetails.BadRequest).FieldViolations[0].Reason)
	require.Len(t, httperr.NewBody(decodedHTTP, httperr.Options{}).Error.Details, 2)
}
