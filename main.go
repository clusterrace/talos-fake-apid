package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
)

type server struct {
	v1alpha1.UnimplementedStateServer
}

// GetState implementation
func (s *server) Get(ctx context.Context, in *v1alpha1.GetRequest) (*v1alpha1.GetResponse, error) {
	log.Println(in)
	return &v1alpha1.GetResponse{}, nil
}

func (s *server) Watch(in *v1alpha1.WatchRequest, stream grpc.ServerStreamingServer[v1alpha1.WatchResponse]) error {
	log.Println(in)
	for {
		select {
		// Exit on stream context done
		case <-stream.Context().Done():
			return nil
		default:
			err := stream.Send(&v1alpha1.WatchResponse{
				Event: []*v1alpha1.Event{
					{
						Resource: &v1alpha1.Resource{
							Metadata: &v1alpha1.Metadata{
								Version: "1",
								Phase:   "running",
							},
							Spec: &v1alpha1.Spec{},
						},
						EventType: v1alpha1.EventType_CREATED,
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
	lis, err := net.Listen("tcp", "0.0.0.0:50000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	creds, err := credentials.NewServerTLSFromFile("server.crt", "server.key")
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	protobuf.RegisterDynamic[ServiceSpec](ServiceType, &Service{})
	s.RegisterService(&v1alpha1.State_ServiceDesc, &server{})
	log.Println("Starting server at port 50000...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
