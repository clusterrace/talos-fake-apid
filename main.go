package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/clusterrace/talos-fake-apid/proto/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

type server struct {
	pb.UnimplementedStateServer
}

// GetState implementation
func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetResponse, error) {
	log.Println(in)
	return &pb.GetResponse{}, nil
}

func (s *server) Watch(in *pb.WatchRequest, stream grpc.ServerStreamingServer[pb.WatchResponse]) error {
	log.Println(in)
	for {
		select {
		// Exit on stream context done
		case <-stream.Context().Done():
			return nil
		default:
			err := stream.Send(&pb.WatchResponse{
				Event: []*pb.Event{
					{
						Resource:  &pb.Resource{},
						EventType: 0,
					},
				},
			})
			if err != nil {
				log.Println(err.Error())
			}
		}
	}
}

func main() {
	protobuf.RegisterDynamic[ServiceSpec](ServiceType, &Service{})
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
	log.Println("Starting server at port 50000...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
