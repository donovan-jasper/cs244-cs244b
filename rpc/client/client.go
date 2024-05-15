package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	pb "github.com/enigma/raft/pb/github.com/enigma/raft/pb"
)

func main() {
	// Set up a connection to the gRPC server
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create a new Raft client
	client := pb.NewRaftClient(conn)

	// Set a timeout context for the RPC calls
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	// Test the AppendEntries RPC
	testAppendEntries(ctx, client)

	// Test the RequestVote RPC
	testRequestVote(ctx, client)

	// Test the InstallSnapshot RPC
	testInstallSnapshot(ctx, client)
}

// testAppendEntries tests the AppendEntries RPC
func testAppendEntries(ctx context.Context, client pb.RaftClient) {
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
		log.Fatalf("AppendEntries RPC failed: %v", err)
	}
	log.Printf("AppendEntries response: %v", resp)
}

// testRequestVote tests the RequestVote RPC
func testRequestVote(ctx context.Context, client pb.RaftClient) {
	req := &pb.RequestVoteRequest{
		Term:         1,
		CandidateId:  "candidate-1",
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	resp, err := client.RequestVote(ctx, req)
	if err != nil {
		log.Fatalf("RequestVote RPC failed: %v", err)
	}
	log.Printf("RequestVote response: %v", resp)
}

// testInstallSnapshot tests the InstallSnapshot RPC
func testInstallSnapshot(ctx context.Context, client pb.RaftClient) {
	req := &pb.InstallSnapshotRequest{
		Term:              1,
		LeaderId:          "leader-1",
		LastIncludedIndex: 0,
		LastIncludedTerm:  0,
		Data:              []byte("snapshot data"),
	}

	resp, err := client.InstallSnapshot(ctx, req)
	if err != nil {
		log.Fatalf("InstallSnapshot RPC failed: %v", err)
	}
	log.Printf("InstallSnapshot response: %v", resp)
}
