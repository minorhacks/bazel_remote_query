package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/minorhacks/bazel_remote_query/db"
	"github.com/minorhacks/bazel_remote_query/db/datastore"
	"github.com/minorhacks/bazel_remote_query/db/sqlite"
	"github.com/minorhacks/bazel_remote_query/dispatch"
	pb "github.com/minorhacks/bazel_remote_query/proto"
	"github.com/minorhacks/bazel_remote_query/queue"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/prototext"
)

var configPath = flag.String("config", "", "Path to textproto DispatcherConfig")

func main() {
	flag.Parse()

	config, err := loadConfig(*configPath)
	exitIf(err)

	conn, err := net.Listen("tcp", net.JoinHostPort("", config.GetGrpcPort()))
	exitIf(err)

	ctx := context.Background()

	var database db.DB
	switch dbConfig := config.Database.(type) {
	case *pb.DispatcherConfig_Sqlite:
		database, err = sqlite.New(ctx, dbConfig.Sqlite.GetDbPath())
	case *pb.DispatcherConfig_Datastore:
		database, err = datastore.New(ctx, dbConfig.Datastore.GetGcpProject())
	}
	exitIf(err)
	defer database.Close()

	dispatchService := &dispatch.DatabaseDispatch{
		DB: database,
	}

	queueService := &queue.DatabaseQueue{
		DB: database,
	}

	srv := grpc.NewServer()
	pb.RegisterQueryDispatchServer(srv, dispatchService)
	pb.RegisterQueryQueueServer(srv, queueService)
	reflection.Register(srv)

	glog.Infof("Listening on port %s", config.GetGrpcPort())
	srv.Serve(conn)
}

func loadConfig(path string) (*pb.DispatcherConfig, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %q: %w", path, err)
	}
	var config pb.DispatcherConfig
	if err := prototext.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config %q: %w", path, err)
	}
	return &config, nil
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}
