// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	herodotv2 "github.com/ory/herodot/v2"
	"github.com/ory/herodot/v2/grpcerr"
)

var (
	identityNotFound = herodotv2.Define("kratos.ory.com", "IDENTITY_NOT_FOUND", herodotv2.CodeNotFound, "The identity could not be found.")
	flowExpired      = herodotv2.Define("kratos.ory.com", "FLOW_EXPIRED", herodotv2.CodeExpired, "The self-service flow has expired.")
)

func TestLegacy(t *testing.T) {
	cause := errors.New("sql: no rows")
	newLegacy := func() error {
		return Legacy(
			ErrNotFound().WithReasonf("Unable to locate identity %s", "1234"),
			identityNotFound.New(herodotv2.Cause(cause), herodotv2.Metadata("identityId", "1234")),
		)
	}
	newBefore := func() error {
		return ErrNotFound().WithReasonf("Unable to locate identity %s", "1234").WithTrace(cause)
	}

	t.Run("case=old checks see the outer error", func(t *testing.T) {
		err := errors.WithStack(newLegacy())
		var de *DefaultError
		require.ErrorAs(t, err, &de)
		assert.Equal(t, "Unable to locate identity 1234", de.Reason())
		assert.ErrorIs(t, err, ErrNotFound())
		assert.ErrorIs(t, err, cause)
		assert.Equal(t, http.StatusNotFound, ToDefaultError(err, "").StatusCode())
		assert.NotEmpty(t, de.StackTrace(), "the stack trace is found through the inner error")
	})

	t.Run("case=new checks see the inner error", func(t *testing.T) {
		err := errors.WithStack(newLegacy())
		assert.True(t, identityNotFound.Matches(err))
		e := herodotv2.From(err)
		assert.Equal(t, identityNotFound, e.Definition())
		assert.Equal(t, map[string]string{"identityId": "1234"}, e.Metadata())
	})

	t.Run("case=responses are byte-equal", func(t *testing.T) {
		render := func(err error) (int, http.Header, string) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("X-Request-ID", "rid")
			NewJSONWriter(nil).WriteError(w, r, err)
			return w.Code, w.Header(), w.Body.String()
		}
		bc, bh, bb := render(newBefore())
		lc, lh, lb := render(newLegacy())
		assert.Equal(t, bc, lc)
		assert.Equal(t, bh, lh)
		assert.Equal(t, bb, lb)

		bs, _ := status.FromError(unwrapGRPCStatusError(context.Background(), newBefore()))
		ls, _ := status.FromError(unwrapGRPCStatusError(context.Background(), newLegacy()))
		assert.Equal(t, bs.Code(), ls.Code())
		assert.Equal(t, bs.Message(), ls.Message())
	})
}

func TestHerodotError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      *DefaultError
		reason   string
		code     herodotv2.Code
		metadata map[string]string
	}{
		{"grpc code", ErrNotFound(), "NOT_FOUND", herodotv2.CodeNotFound, nil},
		{"id", ErrMisconfiguration(), "INVALID_CONFIGURATION", herodotv2.CodeInternal, nil},
		{"http status only", ErrUpstreamError(), "UPSTREAM_ERROR", herodotv2.CodeBadGateway, nil},
		{"camel case id", ErrBadRequest().WithID("sessionInactive"), "SESSION_INACTIVE", herodotv2.CodeInvalidArgument, nil},
		{"id with separators", ErrBadRequest().WithID("foo-bar.baz  qux_"), "FOO_BAR_BAZ_QUX", herodotv2.CodeInvalidArgument, nil},
		{"invalid id", ErrBadRequest().WithID("1abc"), "INVALID_ARGUMENT", herodotv2.CodeInvalidArgument, nil},
		{"unknown 4xx", &DefaultError{CodeField: 418}, "FAILED_PRECONDITION", herodotv2.CodeFailedPrecondition, nil},
		{"no code", &DefaultError{}, "INTERNAL", herodotv2.CodeInternal, nil},
		{
			"details", ErrBadRequest().WithDetail("str", "v").WithDetail("num", 1).WithDetail("obj", map[string]int{"a": 1}),
			"INVALID_ARGUMENT", herodotv2.CodeInvalidArgument, map[string]string{"str": "v", "num": "1", "obj": `{"a":1}`},
		},
	} {
		t.Run("case="+tc.name, func(t *testing.T) {
			e := herodotv2.From(fmt.Errorf("wrapped: %w", tc.err))
			assert.Equal(t, LegacyDomain, e.Definition().Domain())
			assert.Equal(t, tc.reason, e.Definition().Reason())
			assert.Equal(t, tc.code, e.Definition().Code())
			assert.Equal(t, tc.err.ErrorField, e.Error())
			assert.Equal(t, tc.metadata, e.Metadata())
			assert.Same(t, tc.err, e.Unwrap())
		})
	}

	_, ok := (*DefaultError)(nil).HerodotError()
	assert.False(t, ok)
}

func TestBareError(t *testing.T) {
	err := herodotv2.Annotate(
		flowExpired.New(herodotv2.Metadata("flowId", "1234"), herodotv2.Cause(errors.New("expired at noon"))),
		herodotv2.Metadata("returnTo", "/home"),
	)

	t.Run("case=JSON writer renders the legacy shape", func(t *testing.T) {
		for _, debug := range []bool{false, true} {
			t.Run(fmt.Sprintf("debug=%v", debug), func(t *testing.T) {
				w := httptest.NewRecorder()
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Set("X-Request-ID", "rid")
				h := NewJSONWriter(nil)
				h.EnableDebug = debug
				h.WriteError(w, r, errors.WithStack(err))

				assert.Equal(t, http.StatusGone, w.Code)
				assert.Equal(t, "flow_expired", w.Header().Get("Ory-Error-Id"))
				expected := `{"error":{"id":"flow_expired","code":410,"status":"Gone","request":"rid","details":{"flowId":"1234","returnTo":"/home"},"message":"The self-service flow has expired."}}`
				if debug {
					expected = `{"error":{"id":"flow_expired","code":410,"status":"Gone","request":"rid","debug":"expired at noon","details":{"flowId":"1234","returnTo":"/home"},"message":"The self-service flow has expired."}}`
				}
				assert.JSONEq(t, expected, w.Body.String())
			})
		}
	})

	t.Run("case=ToDefaultError", func(t *testing.T) {
		de := ToDefaultError(err, "rid")
		assert.Equal(t, codes.FailedPrecondition, de.GRPCCodeField)
		assert.Empty(t, de.ReasonField)
		assert.True(t, flowExpired.Matches(de))
	})

	t.Run("case=499 status text", func(t *testing.T) {
		cancelled := herodotv2.Define("ory.com", "CANCELLED", herodotv2.CodeCancelled, "The request was cancelled.")
		assert.Equal(t, "Client Closed Request", ToDefaultError(cancelled.New(), "").StatusField)
	})

	t.Run("case=a DefaultError in the tree takes precedence", func(t *testing.T) {
		withCause := flowExpired.New(herodotv2.Cause(ErrForbidden()))
		assert.Equal(t, http.StatusForbidden, ToDefaultError(withCause, "").StatusCode())
	})

	t.Run("case=gRPC interceptor renders the AIP status", func(t *testing.T) {
		inner := status.Error(codes.Unavailable, "upstream")
		s, ok := status.FromError(unwrapGRPCStatusError(context.Background(), flowExpired.New(herodotv2.Cause(inner))))
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, s.Code())
		info, ok := s.Details()[0].(*errdetails.ErrorInfo)
		require.True(t, ok)
		assert.Equal(t, "FLOW_EXPIRED", info.Reason)

		received, ok := grpcerr.FromStatus(s)
		require.True(t, ok)
		assert.Equal(t, herodotv2.CodeExpired, received.Definition().Code())
	})
}

type memoryRecorder struct {
	sync.Mutex
	http []HTTPErrorRecording
	grpc []GRPCErrorRecording
}

func (m *memoryRecorder) RecordHTTPError(r HTTPErrorRecording) {
	m.Lock()
	defer m.Unlock()
	m.http = append(m.http, r)
}

func (m *memoryRecorder) RecordGRPCError(r GRPCErrorRecording) {
	m.Lock()
	defer m.Unlock()
	m.grpc = append(m.grpc, r)
}

func TestRecorder(t *testing.T) {
	t.Run("case=http", func(t *testing.T) {
		rec := &memoryRecorder{}
		h := NewJSONWriter(nil)
		h.Recorder = rec

		mux := http.NewServeMux()
		mux.HandleFunc("GET /identities/{id}", func(w http.ResponseWriter, r *http.Request) {
			h.WriteError(w, r, Legacy(ErrBadRequest().WithReason("expired"), flowExpired.New()))
		})
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/identities/1234", nil))

		require.Len(t, rec.http, 1)
		got := rec.http[0]
		assert.Equal(t, "GET", got.Method)
		assert.Equal(t, "GET /identities/{id}", got.Route)

		assert.Equal(t, http.StatusBadRequest, got.Response.Status)
		assert.Equal(t, w.Body.String(), got.Response.Body)
		assert.Equal(t, http.Header{"Ory-Error-Id": {""}, "Content-Type": {"application/json"}}, got.Response.Header)

		assert.Equal(t, http.StatusGone, got.Shadow.Status, "the shadow shows the intended change")
		assert.Equal(t, http.Header{"Content-Type": {"application/json; charset=utf-8"}}, got.Shadow.Header)
		var body map[string]map[string]any
		require.NoError(t, json.Unmarshal([]byte(got.Shadow.Body), &body))
		assert.Equal(t, "FAILED_PRECONDITION", body["error"]["status"])
	})

	t.Run("case=grpc", func(t *testing.T) {
		rec := &memoryRecorder{}
		info := &grpc.UnaryServerInfo{FullMethod: "/helloworld.Greeter/SayHello"}
		_, err := RecordingUnaryErrorUnwrapInterceptor(rec)(context.Background(), nil, info, func(context.Context, interface{}) (interface{}, error) {
			return nil, Legacy(ErrNotFound(), identityNotFound.New())
		})
		assert.Equal(t, codes.NotFound, status.Code(err))

		_, err = RecordingUnaryErrorUnwrapInterceptor(rec)(context.Background(), nil, info, func(context.Context, interface{}) (interface{}, error) {
			return "ok", nil
		})
		require.NoError(t, err)

		require.Len(t, rec.grpc, 1)
		got := rec.grpc[0]
		assert.Equal(t, "/helloworld.Greeter/SayHello", got.FullMethod)
		assert.EqualValues(t, codes.NotFound, got.Status.Code)
		assert.EqualValues(t, codes.NotFound, got.Shadow.Code)
		assert.Len(t, got.Shadow.Details, 1)
	})
}

type recordingTestStream struct{ grpc.ServerStream }

func (recordingTestStream) Context() context.Context { return context.Background() }

func TestGRPCRecorderPreservesOriginalError(t *testing.T) {
	for _, original := range []error{
		flowExpired.New(),
		Legacy(ErrNotFound(), identityNotFound.New()),
		herodotv2.Annotate(Legacy(ErrNotFound(), identityNotFound.New()), herodotv2.Metadata("outer", "kept")),
		nil,
	} {
		for _, transport := range []string{"unary", "stream"} {
			t.Run(fmt.Sprintf("%s/%T", transport, original), func(t *testing.T) {
				rec := &memoryRecorder{}
				calls := 0
				var err error
				if transport == "unary" {
					var resp any
					resp, err = RecordingUnaryErrorUnwrapInterceptor(rec)(context.Background(), nil,
						&grpc.UnaryServerInfo{FullMethod: "/test"}, func(context.Context, any) (any, error) {
							calls++
							return "response", original
						})
					assert.Equal(t, "response", resp)
				} else {
					err = RecordingStreamErrorUnwrapInterceptor(rec)(nil, recordingTestStream{},
						&grpc.StreamServerInfo{FullMethod: "/test"}, func(any, grpc.ServerStream) error {
							calls++
							return original
						})
				}
				assert.Equal(t, 1, calls)
				want := unwrapGRPCStatusError(context.Background(), original)
				assert.True(t, proto.Equal(status.Convert(want).Proto(), status.Convert(err).Proto()))
				if original == nil {
					require.Empty(t, rec.grpc)
					return
				}
				require.Len(t, rec.grpc, 1)
				got := rec.grpc[0]
				assert.Equal(t, "/test", got.FullMethod)
				assert.True(t, proto.Equal(status.Convert(err).Proto(), got.Status))
				assert.True(t, proto.Equal(grpcerr.ToStatus(context.Background(), original, false).Proto(), got.Shadow))
			})
		}
	}
}
