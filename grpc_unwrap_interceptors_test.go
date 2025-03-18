// Copyright © 2023 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package herodot

import (
	"context"
	"net"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ory/herodot/internal"
)

type testingGreeter struct {
	internal.UnimplementedGreeterServer
	shouldErr bool
}

func (g *testingGreeter) SayHello(context.Context, *internal.HelloRequest) (*internal.HelloReply, error) {
	if g.shouldErr {
		return nil, errors.WithStack(ErrInternalServerError)
	}
	return &internal.HelloReply{Message: "see, no error"}, nil
}

func TestGRPCInterceptors(t *testing.T) {
	server := &testingGreeter{}
	s := grpc.NewServer(grpc.UnaryInterceptor(UnaryErrorUnwrapInterceptor))
	internal.RegisterGreeterServer(s, server)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	serveErr := &errgroup.Group{}
	serveErr.Go(func() error {
		return s.Serve(l)
	})

	conn, err := grpc.Dial(l.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)
	c := internal.NewGreeterClient(conn)

	for _, tc := range []struct {
		name      string
		shouldErr bool
	}{
		{
			name:      "no error",
			shouldErr: false,
		},
		{
			name:      "internal error",
			shouldErr: true,
		},
	} {
		t.Run("case="+tc.name, func(t *testing.T) {
			server.shouldErr = tc.shouldErr

			resp, err := c.SayHello(context.Background(), &internal.HelloRequest{})
			if tc.shouldErr {
				assert.Equal(t, codes.Internal, status.Code(err))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "see, no error", resp.Message)
			}
		})
	}

	s.Stop()
	require.NoError(t, serveErr.Wait())
}
