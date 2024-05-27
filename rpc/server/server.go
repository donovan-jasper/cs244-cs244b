package main

import (
	"context"
	"log"
	"net"

	pb "github.com/enigma/raft/pb/github.com/enigma/raft/pb"
	"google.golang.org/grpc"
)

// server is used to implement the Raft service
type server struct {
	pb.UnimplementedRaftServer
}

// AppendEntries handles the AppendEntries RPC
func (s *server) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	// Check if the request contains valid entries
	if req.PrevLogIndex != 0 || req.PrevLogTerm != 0 || len(req.Entries) == 0 {
		return &pb.AppendEntriesResponse{
			Term:    req.Term,
			Success: false,
		}, nil
	}

	// Simulate a successful append entries operation
	return &pb.AppendEntriesResponse{
		Term:    req.Term,
		Success: true,
	}, nil
}

// RequestVote handles the RequestVote RPC
func (s *server) RequestVote(ctx context.Context, req *pb.RequestVoteRequest) (*pb.RequestVoteResponse, error) {
	// Check if the candidate's term and log index are valid
	if req.Term > 1 || req.LastLogIndex > 0 || req.LastLogTerm > 0 {
		return &pb.RequestVoteResponse{
			Term:        req.Term,
			VoteGranted: false,
		}, nil
	}

	// Simulate a successful vote grant
	return &pb.RequestVoteResponse{
		Term:        req.Term,
		VoteGranted: true,
	}, nil
}

// InstallSnapshot handles the InstallSnapshot RPC
func (s *server) InstallSnapshot(ctx context.Context, req *pb.InstallSnapshotRequest) (*pb.InstallSnapshotResponse, error) {
	// Simulate a successful snapshot installation
	return &pb.InstallSnapshotResponse{
		Term:    req.Term,
		Success: true,
	}, nil
}

func main() {
	// Listen on TCP port 50051
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create a new gRPC server
	s := grpc.NewServer()

	// Register the Raft service with the server
	pb.RegisterRaftServer(s, &server{})

	log.Printf("Server is listening on %v", lis.Addr())

	// Start serving incoming connections
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
