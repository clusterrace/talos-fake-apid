package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/clusterrace/talos-fake-apid/proto/v1alpha1"
)

type server struct {
	pb.UnimplementedStateServer
}

// GetState implementation
func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetResponse, error) {
	return &pb.GetResponse{}, nil
}

func (s *server) Watch(ctx context.Context, in *pb.WatchRequest) (grpc.ServerStreamingClient[pb.WatchResponse], error) {
	
}

func main() {
	lis, err := net.Listen("tcp", "0.0.0.0:50000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	creds, err := credentials.NewServerTLSFromFile("server.crt", "server.key")
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterStateServer(s, &server{})
	fmt.Println("Starting server at port 50000...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
