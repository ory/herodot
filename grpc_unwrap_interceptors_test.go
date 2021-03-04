package herodot

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/status"
)

type internalErrServer struct {
	helloworld.UnimplementedGreeterServer
}

func (*internalErrServer) SayHello(context.Context, *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	return nil, errors.WithStack(ErrInternalServerError)
}

func TestGRPCInterceptors(t *testing.T) {
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	s := grpc.NewServer(grpc.UnaryInterceptor(UnaryErrorUnwrapInterceptor))
	helloworld.RegisterGreeterServer(s, &internalErrServer{})
	l, err := net.Listen("tcp", addr)

	serveErr := &errgroup.Group{}
	serveErr.Go(func() error {
		return s.Serve(l)
	})

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	require.NoError(t, err)
	c := helloworld.NewGreeterClient(conn)

	_, err = c.SayHello(context.Background(), &helloworld.HelloRequest{})
	require.NotNil(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	s.Stop()
	require.NoError(t, serveErr.Wait())
}
