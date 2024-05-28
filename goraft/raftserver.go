package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

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
	var raftMsg RaftMessage
	if err := proto.Unmarshal([]byte(msg), &raftMsg); err != nil {
		fmt.Errorf("failed to unmarshal message: %w", err)
	}

	switch raftMsg.Message.(type) {
	case *RaftMessage_AppendEntriesRequest:
		rs.handleAppendEntriesRequest(raftMsg.GetAppendEntriesRequest())
	case *RaftMessage_AppendEntriesResponse:
		rs.handleAppendEntriesResponse(raftMsg.GetAppendEntriesResponse())
	case *RaftMessage_RequestVoteRequest:
		rs.handleRequestVoteRequest(raftMsg.GetRequestVoteRequest())
	case *RaftMessage_RequestVoteResponse:
		rs.handleRequestVoteResponse(raftMsg.GetRequestVoteResponse())
	default:
		fmt.Errorf("unknown message type")
	}
}

// TODO: Add logentries
func (rs *RaftServer) handleAppendEntriesRequest(aeMsg *AppendEntriesRequest) {
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
	appEntriesResp := &AppendEntriesResponse{
		Term:       int32(rs.currentTerm),
		FollowerId: int32(rs.id),
		// TODO: Set real acked index
		AckedIdx: int32(-1),
		Success:  success,
	}
	raftMsg := &RaftMessage{
		Message: &RaftMessage_AppendEntriesResponse{appEntriesResp},
	}

	rs.sendRaftMsg(int(aeMsg.GetLeaderId()), raftMsg)
}

func (rs *RaftServer) handleAppendEntriesResponse(aerMsg *AppendEntriesResponse) {
	if int(aerMsg.GetTerm()) == rs.currentTerm && rs.loadCurrentState() == Leader {
		if bool(aerMsg.GetSuccess()) && int(aerMsg.GetAckedIdx()) > rs.ackedIndex[int(aerMsg.GetFollowerId())] {
			rs.nextIndex[int(aerMsg.GetFollowerId())] = int(aerMsg.GetAckedIdx())
			rs.ackedIndex[int(aerMsg.GetFollowerId())] = int(aerMsg.GetAckedIdx()) - 1
			// TODO: Commit log entry
		} else if rs.nextIndex[int(aerMsg.GetFollowerId())] > 0 {
			rs.nextIndex[int(aerMsg.GetFollowerId())]--
			// TODO: Replicate log to other servers
		}
	} else if int(aerMsg.GetTerm()) > rs.currentTerm {
		rs.currentTerm = int(aerMsg.GetTerm())
		rs.setCurrentState(Follower)
		rs.votedFor = -1
	}
}

func (rs *RaftServer) handleRequestVoteRequest(rvMsg *RequestVoteRequest) {
	if int(rvMsg.GetTerm()) > rs.currentTerm {
		rs.currentTerm = int(rvMsg.GetTerm())
		rs.setCurrentState(Follower)
		rs.votedFor = -1
	}

	// TODO: Get last term from log
	//lastTerm := rs.currentTerm

	// TODO: Determine whether their log is up to dated
	logUpdated := true
	vote := false

	// If these conditions are met, vote for candidate
	if int(rvMsg.GetTerm()) >= rs.currentTerm && logUpdated && (rs.votedFor == -1 || rs.votedFor == int(rvMsg.GetCandidateId())) {
		rs.votedFor = int(rvMsg.GetCandidateId())
		vote = true

		go rs.electionTimeoutTimer.Run()
	}

	requestVoteReplyMsg := &RequestVoteResponse{
		Term:        int32(rs.currentTerm),
		VoteGranted: vote,
		VoterId:     int32(rs.id),
	}

	raftMsg := &RaftMessage{
		Message: &RaftMessage_RequestVoteResponse{requestVoteReplyMsg},
	}

	rs.sendRaftMsg(int(rvMsg.GetCandidateId()), raftMsg)

}

func (rs *RaftServer) handleRequestVoteResponse(rvMsg *RequestVoteResponse) {
	if rs.loadCurrentState() == Candidate && int(rvMsg.GetTerm()) == rs.currentTerm && bool(rvMsg.GetVoteGranted()) {
		rs.votesReceived[int(rvMsg.GetVoterId())] = true
		rs.evaluateElection()
	} else if int(rvMsg.GetTerm()) > rs.currentTerm {
		rs.currentTerm = int(rvMsg.GetTerm())
		rs.setCurrentState(Follower)
		rs.votedFor = -1
	}
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

func (rs *RaftServer) sendRaftMsg(targetId int, raftMsg *RaftMessage) {
	serializedMsg, err := proto.Marshal(raftMsg)
	if err != nil {
		fmt.Println(err)
		return
	}

	addr := rs.peers[targetId]
	rs.net.send(addr.ip+":"+addr.port, string(serializedMsg))
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
