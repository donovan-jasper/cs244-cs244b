package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"goraft/github.com/enigma/raft/pb"

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
	peers []Address

	// Queue things
	mu    sync.Mutex
	queue []proto.Message

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

	net *NetworkModule

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

	rs.net = NewNetworkModule()

	rs.heartbeatTimeoutTimer = NewTimer(randomDuration(1000000000, 2000000000), rs, setStateToCandidateCB)
	rs.electionTimeoutTimer = NewTimer(randomDuration(1000000000, 2000000000), rs, setStateToFollowerCB)

	return rs
}

func (rs *RaftServer) run() {
	rs.net.listen(rs.peers[rs.id].port)

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
			select {
			case msg, ok := <-rs.net.msgQueue:
				if ok {
					rs.handleMessage(msg)
				} else {
					fmt.Println("Channel closed!")
				}
			default:
				continue
			}
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
	if err := proto.Unmarshal(msg, &raftMsg); err != nil {
		fmt.Errorf("failed to unmarshal message: %w", err)
	}

	switch raftMsg.Message.(type) {
	case *pb.RaftMessage_AppendEntriesRequest:
		rs.handleAppendEntriesRequest(raftMsg.GetAppendEntriesRequest())
	case *pb.RaftMessage_AppendEntriesResponse:
		rs.handleAppendEntriesResponse(raftMsg.GetAppendEntriesResponse())
	case *pb.RaftMessage_RequestVoteRequest:
		rs.handleRequestVoteRequest(raftMsg.GetRequestVoteRequest())
	case *pb.RaftMessage_RequestVoteResponse:
		rs.handleRequestVoteResponse(raftMsg.GetRequestVoteResponse())
	default:
		fmt.Errorf("unknown message type")
	}
}

// TODO: Add logentries
func (rs *RaftServer) handleAppendEntriesRequest(aeMsg *pb.AppendEntriesRequest) {
	if int(aeMsg.GetTerm()) > rs.currentTerm {
		rs.currentTerm = int(aeMsg.GetTerm())
		rs.votedFor = -1
	}

	success := false
	if int(aeMsg.GetTerm()) == rs.currentTerm {
		rs.setCurrentState(Follower)
		rs.currentLeader = int(aeMsg.GetLeaderId())

		// TODO: Check log lengths
		logChecksOut := true

		if logChecksOut {
			success = true

			go rs.electionTimeoutTimer.Run()

			// TODO: Remove all inconsistent entries and append new ones

			// TODO: Commit entries that have been committed by the leader
		}
	}

	// Send AppendEntriesResponse to leader

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
