package main

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/enigma/raft/pb"

	"google.golang.org/protobuf/proto"
)

type State int32

const (
	Follower State = iota
	Candidate
	Leader
)

type RaftServer struct {
	id int
	// Network addresses of cluster peers
	peers []string

	// Queue things
	mu       sync.Mutex
	queue    []proto.Message

	// Persistent state
	currentTerm int
	votedFor    int
	// TODO: Add log entries

	// TODO: Add persistant storage mechanism

	currentState int32
	lastState    int32

	// Volatile state
	commitIndex   int
	lastApplied   int
	currentLeader int

	votesReceived map[int]bool
	nextIndex     []int
	ackedIndex    []int

	net 		  *NetworkModule

	heartbeatTimeoutTimer *Timer
	electionTimeoutTimer  *Timer

	// TODO: Apply logs in background
}

func setStateToCandidateCB(rs *RaftServer) {
	rs.setCurrentState(Candidate)
}

func setStateToFollowerCB(rs *RaftServer) {
	rs.setCurrentState(Follower)
}

func NewRaftServer(id int, peers []Address, restoreFromDisk bool) *RaftServer {
	rs := new(RaftServer)
	rs.id = id
	rs.peers = peers
	rs.currentTerm = 0
	rs.votedFor = -1
	// TODO: Restore from log based on bool

	rs.setCurrentState(Follower)
	rs.setLastState(Follower)
	rs.commitIndex = -1
	rs.lastApplied = -1
	rs.currentLeader = -1
	rs.votesReceived = make(map[int]bool)

	rs.nextIndex = make([]int, len(peers))
	rs.ackedIndex = make([]int, len(peers))
	for i := range rs.ackedIndex {
		rs.ackedIndex[i] = -1
	}

	rs.net := NewNetworkModule()

	rs.heartbeatTimeoutTimer = NewTimer(randomDuration(1000000000, 2000000000), rs, setStateToCandidateCB)
	rs.electionTimeoutTimer = NewTimer(randomDuration(1000000000, 2000000000), rs, setStateToFollowerCB)

	return rs
}

func (rs *RaftServer) run() {
	rs.net.listen(rs.peerAddresses[rs.id].port)

	for {
		rs.setLastState(rs.loadCurrentState())

		switch rs.loadCurrentState() {
		case Follower:
			go rs.heartbeatTimeoutTimer.Run()
		case Candidate:
			rs.doElection()
		case Leader:
			// TODO: Start heartbeat thread
		}

		for rs.loadCurrentState() == rs.loadLastState() {
			var msg string
			msg <- n.msgQueue
			rs.handleMessage(msg)
		}

		rs.heartbeatTimeoutTimer.Stop()

		// TODO: Stop leader heartbeat thread
	}
}

func (rs *RaftServer) doElection() {
	fmt.Println("Election starting")
	rs.currentTerm++
	rs.votedFor = rs.id
	rs.votesReceived = make(map[int]bool)

	rs.votesReceived[rs.id] = true

	go rs.electionTimeoutTimer.Run()

	// TODO: Update and uncomment
	/*
		var lastLogIndex int
		var lastLogTerm int
		if len(rs.log == 0) {

		} else {

		}
	*/

	for i, addr := range rs.peers {
		if i != rs.id {
			// TODO: Send RequestVote RPC
			fmt.Println(addr)
		}
	}

	rs.evaluateElection()
}

func (rs *RaftServer) handleMessage(msg string) {
	var raftMsg pb.RaftMessage
	if err := proto.Unmarshal(data, &raftMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	switch msg := raftMsg.Message.(type) {
	case *pb.RaftMessage_AppendEntriesRequest:
		rs.handleAppendEntriesRequest(msg.AppendEntriesRequest)
	case *pb.RaftMessage_AppendEntriesResponse:
		rs.handleAppendEntriesResponse(msg.AppendEntriesResponse)
	case *pb.RaftMessage_RequestVoteRequest:
		rs.handleRequestVoteRequest(msg.RequestVoteRequest)
	case *pb.RaftMessage_RequestVoteResponse:
		rs.handleRequestVoteResponse(msg.RequestVoteResponse)
	default:
		return fmt.Errorf("unknown message type")
	}
} 

func (rs *RaftMessage) handleAppendEntriesRequest() {
	
}

func (rs *RaftServer) evaluateElection() {
	numVotes := 0
	for range rs.votesReceived {
		numVotes++
	}

	if numVotes >= (len(rs.peers)+1.0)/2 {
		rs.setCurrentState(Leader)
		rs.currentLeader = rs.id
	}
}

func (rs *RaftServer) loadCurrentState() State {
	return State(atomic.LoadInt32(&rs.currentState))
}

func (rs *RaftServer) setCurrentState(s State) {
	atomic.StoreInt32(&rs.currentState, int32(s))
}

func (rs *RaftServer) loadLastState() State {
	return State(atomic.LoadInt32(&rs.lastState))
}

func (rs *RaftServer) setLastState(s State) {
	atomic.StoreInt32(&rs.lastState, int32(s))
}

func randomDuration(min, max int) time.Duration {
	return time.Duration(rand.Intn(max-min+1) + min)
}