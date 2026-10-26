// Copyright © 2026 Ory Corp
// SPDX-License-Identifier: Apache-2.0

// Package grpcerr renders herodot errors as gRPC statuses.
package grpcerr

import (
	"context"
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"

	"github.com/ory/herodot/v2"
	"github.com/ory/herodot/v2/internal/herodotpb"
)

// RequestIDKey is the incoming metadata key that ToStatus reads the request
// ID from.
const RequestIDKey = "x-request-id"

// ToStatus returns the status for herodot.From(err): the canonical code,
// ErrorInfo, CodeInfo for a refined code, RequestInfo and BadRequest, and
// DebugInfo only if debug is true.
// It returns nil if err is nil.
func ToStatus(ctx context.Context, err error, debug bool) *status.Status {
	if err == nil {
		return nil
	}
	e := herodot.From(err)
	def := e.Definition()

	details := []protoadapt.MessageV1{&errdetails.ErrorInfo{
		Reason:   def.Reason(),
		Domain:   def.Domain(),
		Metadata: e.Metadata(),
	}}
	if c := def.Code(); !c.IsCanonical() {
		// The status code is canonical. Keep the refinement for receivers.
		details = append(details, &herodotpb.CodeInfo{Code: herodotpb.Code(c)})
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ids := md.Get(RequestIDKey); len(ids) > 0 && ids[0] != "" {
			details = append(details, &errdetails.RequestInfo{RequestId: ids[0]})
		}
	}
	if vs := e.Violations(); len(vs) > 0 {
		br := &errdetails.BadRequest{FieldViolations: make([]*errdetails.BadRequest_FieldViolation, len(vs))}
		for i, v := range vs {
			br.FieldViolations[i] = &errdetails.BadRequest_FieldViolation{Field: v.Field, Description: v.Description}
		}
		details = append(details, br)
	}
	if debug {
		if info, ok := e.DebugInfo(); ok {
			details = append(details, &errdetails.DebugInfo{StackEntries: info.StackEntries, Detail: info.Detail})
		}
	}

	s, wErr := status.New(codes.Code(def.Code().Canonical()), e.Error()).WithDetails(details...)
	if wErr != nil {
		// Only fails for an OK code or unmarshalable details, neither of
		// which can happen here.
		panic(wErr)
	}
	return s
}

// UnaryServerInterceptor converts errors returned by unary handlers with
// ToStatus. It replaces herodot v1's UnaryErrorUnwrapInterceptor.
func UnaryServerInterceptor(debug bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			return resp, ToStatus(ctx, err, debug).Err()
		}
		return resp, nil
	}
}

// StreamServerInterceptor converts errors returned by stream handlers with
// ToStatus. It replaces herodot v1's StreamErrorUnwrapInterceptor.
func StreamServerInterceptor(debug bool) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := handler(srv, ss); err != nil {
			return ToStatus(ss.Context(), err, debug).Err()
		}
		return nil
	}
}

// FromStatus decodes a status received from another service, e.g. one that
// rendered it with ToStatus. It returns false if s isn't a herodot error: an
// OK status, no ErrorInfo, or an invalid domain, reason or code.
//
// The Error has the received identity, so catalog definitions match it, and
// returning it forwards the error unchanged. Its code comes from CodeInfo if
// present, otherwise it is the status code. Its message, metadata and
// violations are the received ones, and its cause is s.Err().
func FromStatus(s *status.Status) (herodot.Error, bool) {
	if s == nil || s.Code() == codes.OK {
		return herodot.Error{}, false
	}
	canonical := herodot.Code(s.Code())
	if !canonical.IsCanonical() {
		return herodot.Error{}, false
	}

	var (
		info       *errdetails.ErrorInfo
		code       = canonical
		violations []herodot.Option
	)
	for _, d := range s.Details() {
		switch d := d.(type) {
		case *errdetails.ErrorInfo:
			if info == nil {
				info = d
			}
		case *herodotpb.CodeInfo:
			// Ignore a refinement that contradicts the status code.
			if c := herodot.Code(d.GetCode()); c.Canonical() == canonical {
				code = c
			}
		case *errdetails.BadRequest:
			for _, v := range d.GetFieldViolations() {
				violations = append(violations, herodot.Violation(v.GetField(), v.GetDescription()))
			}
		}
	}
	if info == nil {
		return herodot.Error{}, false
	}

	def, err := herodot.NewDefinition(info.GetDomain(), info.GetReason(), code, s.Message())
	if err != nil {
		return herodot.Error{}, false
	}
	opts := []herodot.Option{herodot.Cause(s.Err())}
	for k, v := range info.GetMetadata() {
		opts = append(opts, herodot.Metadata(k, v))
	}
	return def.New(append(opts, violations...)...), true
}

// FromError decodes the status of an error returned by a gRPC call, also if
// err wraps it, see FromStatus.
func FromError(err error) (herodot.Error, bool) {
	// Unlike status.FromError, this keeps the received message when err wraps
	// the status error.
	var se interface{ GRPCStatus() *status.Status }
	if !errors.As(err, &se) {
		return herodot.Error{}, false
	}
	return FromStatus(se.GRPCStatus())
}
