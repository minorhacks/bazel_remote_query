package main

import (
	"context"
	"flag"
	"net"

	pb "github.com/minorhacks/bazel_remote_query/proto"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	grpcPort = flag.String("grpc_port", "", "Port on which to start gRPC service")
)

type StaticDispatch struct {
}

func (d *StaticDispatch) GetQueryJob(ctx context.Context, req *pb.GetQueryJobRequest) (*pb.GetQueryJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "GetQueryJob not implemented")
}

func (d *StaticDispatch) FinishQueryJob(ctx context.Context, req *pb.FinishQueryJobRequest) (*pb.FinishQueryJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "FinishQueryJob not implemented")
}

func main() {
	flag.Parse()
	conn, err := net.Listen("tcp", net.JoinHostPort("", *grpcPort))
	exitIf(err)

	srv := grpc.NewServer()
	pb.RegisterQueryDispatcherServer(srv, &StaticDispatch{})

	glog.Infof("Listening on port %s", *grpcPort)
	srv.Serve(conn)
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}
