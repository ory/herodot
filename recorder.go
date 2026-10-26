// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"bytes"
	"context"
	"net/http"

	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/ory/herodot/v2/grpcerr"
	"github.com/ory/herodot/v2/httperr"
)

// ErrorRecorder records error responses together with their AIP shadow
// rendering, i.e. what herodot v2's transports would return for the same
// error. It is meant for test servers: comparing the recordings of a test
// suite between two branches shows whether a migration changed legacy
// responses, and how the AIP responses will look.
//
// Implementations must be safe for concurrent use.
type ErrorRecorder interface {
	RecordHTTPError(HTTPErrorRecording)
	RecordGRPCError(GRPCErrorRecording)
}

// HTTPErrorRecording is one error response written by a JSONWriter.
type HTTPErrorRecording struct {
	Method string `json:"method"`
	// Route is the route pattern if the request was routed by an
	// http.ServeMux, otherwise the URL path.
	Route string `json:"route"`

	// Response is the legacy response that was written.
	Response RecordedResponse `json:"response"`
	// Shadow is the response httperr.Writer would have written.
	Shadow RecordedResponse `json:"shadow"`
}

// RecordedResponse is a recorded HTTP response.
type RecordedResponse struct {
	Status int `json:"status"`
	// Header holds the Ory-Error-Id and Content-Type headers.
	Header http.Header `json:"header"`
	Body   string      `json:"body"`
}

// GRPCErrorRecording is one error returned through a recording interceptor.
type GRPCErrorRecording struct {
	FullMethod string `json:"fullMethod"`

	// Status is the legacy status that was returned.
	Status *spb.Status `json:"status"`
	// Shadow is the status grpcerr.ToStatus would have returned.
	Shadow *spb.Status `json:"shadow"`
}

var recordedHeaders = []string{"Ory-Error-Id", "Content-Type"}

// recordingResponseWriter tees an error response.
type recordingResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *recordingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *recordingResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *recordingResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// bufferedResponseWriter captures the shadow rendering.
type bufferedResponseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *bufferedResponseWriter) Header() http.Header         { return w.header }
func (w *bufferedResponseWriter) WriteHeader(code int)        { w.status = code }
func (w *bufferedResponseWriter) Write(b []byte) (int, error) { return w.body.Write(b) }

func recordedResponse(h http.Header, status int, body []byte) RecordedResponse {
	res := RecordedResponse{Status: status, Header: http.Header{}, Body: string(body)}
	for _, k := range recordedHeaders {
		if v, ok := h[k]; ok {
			res.Header[k] = v
		}
	}
	return res
}

func (h *JSONWriter) record(rw *recordingResponseWriter, r *http.Request, err error) {
	shadow := &bufferedResponseWriter{header: http.Header{}}
	(&httperr.Writer{Debug: h.EnableDebug}).WriteError(shadow, r, err)

	route := r.Pattern
	if route == "" {
		route = r.URL.Path
	}
	h.Recorder.RecordHTTPError(HTTPErrorRecording{
		Method:   r.Method,
		Route:    route,
		Response: recordedResponse(rw.Header(), rw.status, rw.body.Bytes()),
		Shadow:   recordedResponse(shadow.header, shadow.status, shadow.body.Bytes()),
	})
}

// RecordingUnaryErrorUnwrapInterceptor works like UnaryErrorUnwrapInterceptor
// and records every error with rec.
func RecordingUnaryErrorUnwrapInterceptor(rec ErrorRecorder) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, original := handler(ctx, req)
		err := unwrapGRPCStatusError(ctx, original)
		recordGRPC(ctx, rec, info.FullMethod, err, original)
		return resp, err
	}
}

// RecordingStreamErrorUnwrapInterceptor works like
// StreamErrorUnwrapInterceptor and records every error with rec.
func RecordingStreamErrorUnwrapInterceptor(rec ErrorRecorder) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		original := handler(srv, ss)
		err := unwrapGRPCStatusError(ss.Context(), original)
		recordGRPC(ss.Context(), rec, info.FullMethod, err, original)
		return err
	}
}

func recordGRPC(ctx context.Context, rec ErrorRecorder, method string, err, original error) {
	if err == nil {
		return
	}
	rec.RecordGRPCError(GRPCErrorRecording{
		FullMethod: method,
		Status:     status.Convert(err).Proto(),
		Shadow:     grpcerr.ToStatus(ctx, original, false).Proto(),
	})
}
