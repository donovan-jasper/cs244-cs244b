package tests

import (
	"context"
	"log"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	pb "github.com/enigma/raft/pb/github.com/enigma/raft/pb"
)

// Define the buffer size for the bufconn listener
const bufSize = 1024 * 1024

var lis *bufconn.Listener

// Initialize the in-memory gRPC server
func init() {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	pb.RegisterRaftServer(s, &server{})
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
}

// server is used to implement the Raft service
type server struct {
	pb.UnimplementedRaftServer
}

// AppendEntries handles the AppendEntries RPC
func (s *server) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	if req.PrevLogIndex != 0 || req.PrevLogTerm != 0 || len(req.Entries) == 0 {
		return &pb.AppendEntriesResponse{
			Term:    req.Term,
			Success: false,
		}, nil
	}

	return &pb.AppendEntriesResponse{
		Term:    req.Term,
		Success: true,
	}, nil
}

// RequestVote handles the RequestVote RPC
func (s *server) RequestVote(ctx context.Context, req *pb.RequestVoteRequest) (*pb.RequestVoteResponse, error) {
	if req.Term > 1 || req.LastLogIndex > 0 || req.LastLogTerm > 0 {
		return &pb.RequestVoteResponse{
			Term:        req.Term,
			VoteGranted: false,
		}, nil
	}

	return &pb.RequestVoteResponse{
		Term:        req.Term,
		VoteGranted: true,
	}, nil
}

// InstallSnapshot handles the InstallSnapshot RPC
func (s *server) InstallSnapshot(ctx context.Context, req *pb.InstallSnapshotRequest) (*pb.InstallSnapshotResponse, error) {
	return &pb.InstallSnapshotResponse{
		Term:    req.Term,
		Success: true,
	}, nil
}

// bufDialer creates an in-memory connection for the gRPC client to use
func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

// TestAppendEntriesSuccess tests the AppendEntries RPC for a successful case
func TestAppendEntriesSuccess(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()
	client := pb.NewRaftClient(conn)

	req := &pb.AppendEntriesRequest{
		Term:         1,
		LeaderId:     "leader-1",
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries: []*pb.Entry{
			{Index: 1, Term: 1, Command: "set x = 1"},
		},
		LeaderCommit: 0,
	}

	resp, err := client.AppendEntries(ctx, req)
	if err != nil {
		t.Fatalf("AppendEntries failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got %v", resp.Success)
	}
}

// TestAppendEntriesFailure tests the AppendEntries RPC for a failure case
func TestAppendEntriesFailure(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()
	client := pb.NewRaftClient(conn)

	req := &pb.AppendEntriesRequest{
		Term:         2,
		LeaderId:     "leader-2",
		PrevLogIndex: 1,
		PrevLogTerm:  1,
		Entries:      []*pb.Entry{},
		LeaderCommit: 0,
	}

	resp, err := client.AppendEntries(ctx, req)
	if err != nil {
		t.Fatalf("AppendEntries failed: %v", err)
	}

	if resp.Success {
		t.Errorf("Expected failure, got %v", resp.Success)
	}
}

// TestRequestVoteGranted tests the RequestVote RPC for a granted vote case
func TestRequestVoteGranted(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()
	client := pb.NewRaftClient(conn)

	req := &pb.RequestVoteRequest{
		Term:         1,
		CandidateId:  "candidate-1",
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := client.RequestVote(ctx, req)
	if err != nil {
		t.Fatalf("RequestVote failed: %v", err)
	}

	if !resp.VoteGranted {
		t.Errorf("Expected vote granted, got %v", resp.VoteGranted)
	}
}

// TestRequestVoteDenied tests the RequestVote RPC for a denied vote case
func TestRequestVoteDenied(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()
	client := pb.NewRaftClient(conn)

	req := &pb.RequestVoteRequest{
		Term:         2,
		CandidateId:  "candidate-2",
		LastLogIndex: 1,
		LastLogTerm:  1,
	}

	resp, err := client.RequestVote(ctx, req)
	if err != nil {
		t.Fatalf("RequestVote failed: %v", err)
	}

	if resp.VoteGranted {
		t.Errorf("Expected vote denied, got %v", resp.VoteGranted)
	}
}

// TestInstallSnapshotSuccess tests the InstallSnapshot RPC for a successful case
func TestInstallSnapshotSuccess(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()
	client := pb.NewRaftClient(conn)

	req := &pb.InstallSnapshotRequest{
		Term:              1,
		LeaderId:          "leader-1",
		LastIncludedIndex: 0,
		LastIncludedTerm:  0,
		Data:              []byte("snapshot data"),
	}

	resp, err := client.InstallSnapshot(ctx, req)
	if err != nil {
		t.Fatalf("InstallSnapshot failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got %v", resp.Success)
	}
}
