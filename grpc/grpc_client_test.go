package grpc

import (
	"context"
	"github.com/jukylin/esim/config"
	"github.com/jukylin/esim/log"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/reflection"
	"net"
	"testing"
	"time"
)

var logger log.Logger

var svr *GrpcServer

// server is used to implement helloworld.GreeterServer.
type server struct{}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func TestMain(m *testing.M) {

	logger = log.NewLogger()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	serverOptions := ServerOptions{}
	memConfig := config.NewMemConfig()
	memConfig.Set("debug", true)
	memConfig.Set("grpc_server_debug", true)

	svr = NewGrpcServer("test",
		serverOptions.WithServerLogger(logger),
		serverOptions.WithServerConf(memConfig),
		serverOptions.WithUnarySrvItcp(
			ServerStubs(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
				if req.(*pb.HelloRequest).Name == "call_panic" {
					panic("is a test")
				} else if req.(*pb.HelloRequest).Name == "call_panic_arr" {
					var arr [1]string
					arr[0] = "is a test"
					panic(arr)
				} else if req.(*pb.HelloRequest).Name == "call_nil" {
					return nil, err
				}
				resp, err = handler(ctx, req)

				return resp, err
			}),
		))

	pb.RegisterGreeterServer(svr.Server, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(svr.Server)
	go func() {
		if err := svr.Server.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
	}()

	m.Run()
}

func TestNewGrpcClient(t *testing.T) {

	memConfig := config.NewMemConfig()
	memConfig.Set("debug", true)
	memConfig.Set("grpc_client_debug", true)

	clientOptional := ClientOptionals{}
	clientOptions := NewClientOptions(
		clientOptional.WithLogger(logger),
		clientOptional.WithConf(memConfig),
	)

	ctx := context.Background()
	client := NewClient(clientOptions)
	conn := client.DialContext(ctx, ":50051")

	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "esim"})
	if err != nil {
		logger.Errorf(err.Error())
	} else {
		assert.NotEmpty(t, r.Message)
	}
}

func TestSlowClient(t *testing.T) {

	memConfig := config.NewMemConfig()
	memConfig.Set("debug", true)
	memConfig.Set("grpc_client_debug", true)
	memConfig.Set("grpc_client_check_slow", true)
	memConfig.Set("grpc_client_slow_time", 10)

	clientOptional := ClientOptionals{}
	clientOptions := NewClientOptions(
		clientOptional.WithLogger(logger),
		clientOptional.WithConf(memConfig),
		clientOptional.WithDialOptions(
			grpc.WithBlock(),
			grpc.WithChainUnaryInterceptor(slowRequest),
		),
	)

	ctx := context.Background()
	client := NewClient(clientOptions)
	conn := client.DialContext(ctx, ":50051")

	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "esim"})
	if err != nil {
		logger.Errorf(err.Error())
	} else {
		assert.NotEmpty(t, r.Message)
	}
}

func TestServerPanic(t *testing.T) {

	svr.unaryServerInterceptors = append(svr.unaryServerInterceptors, panicResp())

	memConfig := config.NewMemConfig()
	memConfig.Set("debug", true)
	memConfig.Set("grpc_client_debug", true)

	clientOptional := ClientOptionals{}
	clientOptions := NewClientOptions(
		clientOptional.WithLogger(logger),
		clientOptional.WithConf(memConfig),
	)

	ctx := context.Background()
	client := NewClient(clientOptions)
	conn := client.DialContext(ctx, ":50051")

	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "call_panic"})
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestServerPanicArr(t *testing.T) {

	memConfig := config.NewMemConfig()
	memConfig.Set("debug", true)
	memConfig.Set("grpc_client_debug", true)

	clientOptional := ClientOptionals{}
	clientOptions := NewClientOptions(
		clientOptional.WithLogger(logger),
		clientOptional.WithConf(memConfig),
	)

	ctx := context.Background()
	client := NewClient(clientOptions)
	conn := client.DialContext(ctx, ":50051")

	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "call_panic_arr"})
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestSubsReply(t *testing.T) {

	memConfig := config.NewMemConfig()
	memConfig.Set("debug", true)
	memConfig.Set("grpc_client_debug", true)

	clientOptional := ClientOptionals{}
	clientOptions := NewClientOptions(
		clientOptional.WithLogger(logger),
		clientOptional.WithConf(memConfig),
		clientOptional.WithDialOptions(
			grpc.WithChainUnaryInterceptor(ClientStubs(func(ctx context.Context,
				method string, req, reply interface{}, cc *grpc.ClientConn,
				invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				if method == "/helloworld.Greeter/SayHello" {
					reply.(*pb.HelloReply).Message = "esim"
				}
				return nil
			})),
		),
	)

	ctx := context.Background()
	client := NewClient(clientOptions)
	conn := client.DialContext(ctx, ":50051")

	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "esim"})
	if err != nil {
		logger.Errorf(err.Error())
	} else {
		assert.Equal(t, "esim", r.Message)
	}
}
