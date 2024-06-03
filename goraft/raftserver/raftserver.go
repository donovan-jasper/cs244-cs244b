package raftserver

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	"cs244_cs244b/goraft/raftserver/raftlog"
)

type State int32

const (
	Follower State = iota
	Candidate
	Leader
)

const HEARTBEAT_INTERVAL = 2000 * 1000000
const HEARTBEAT_TIMEOUT_MIN = 5000 * 1000000
const HEARTBEAT_TIMEOUT_MAX = 10000 * 1000000
const ELECTION_TIMEOUT_MIN = 5000 * 1000000
const ELECTION_TIMEOUT_MAX = 10000 * 1000000

type RaftServer struct {
	id int
	// Network addresses of cluster peers
	peers []Address

	// Persistent state
	currentTerm int
	votedFor    int
	logEntries  *raftlog.RaftLog

	// TODO: Add persistant storage mechanism

	currentState int32
	lastState    int32

	// Volatile state
	commitIndex   int32
	lastApplied   int
	currentLeader int

	votesReceived map[int]bool
	nextIndex     []int
	ackedIndex    []int

	net *NetworkModule

	heartbeatTimeoutTimer *Timer
	electionTimeoutTimer  *Timer

	// TODO: Apply logs in background
	logApplicationQueue chan raftlog.LogEntry
	dnsModule           DNSModule
}

func setStateToCandidateCB(rs *RaftServer) {
	rs.setCurrentState(Candidate)
}

func setStateToFollowerCB(rs *RaftServer) {
	rs.setCurrentState(Follower)
}

func NewRaftServer(id int, peers []Address, backupFilepath string, restoreFromDisk bool) *RaftServer {
	rs := new(RaftServer)
	rs.id = id
	rs.peers = peers
	rs.currentTerm = 0
	rs.votedFor = -1

	rs.logEntries = raftlog.NewRaftLog(backupFilepath, restoreFromDisk)

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

	rs.heartbeatTimeoutTimer = NewTimer(randomDuration(HEARTBEAT_TIMEOUT_MIN, HEARTBEAT_TIMEOUT_MAX), rs, setStateToCandidateCB)
	rs.electionTimeoutTimer = NewTimer(randomDuration(ELECTION_TIMEOUT_MIN, ELECTION_TIMEOUT_MAX), rs, setStateToFollowerCB)

	return rs
}

func (rs *RaftServer) Run() {
	go rs.net.listen(rs.peers[rs.id].Port)

	for {
		rs.setLastState(rs.loadCurrentState())

		switch rs.loadCurrentState() {
		case Follower:
			go rs.heartbeatTimeoutTimer.Run()
		case Candidate:
			rs.doElection()
		case Leader:
			go rs.sendHeartbeats()
		}

		for rs.loadCurrentState() == rs.loadLastState() {
			select {
			case msg, ok := <-rs.net.msgQueue:
				if ok {
					fmt.Println("Received a message")
					rs.handleMessage(msg)
				} else {
					fmt.Println("Channel closed!")
				}
			default:
				continue
			}
		}

		rs.heartbeatTimeoutTimer.Stop()
		rs.electionTimeoutTimer.Stop()
	}
}

func (rs *RaftServer) doElection() {
	fmt.Println("Election starting")
	rs.currentTerm++
	rs.votedFor = rs.id
	rs.votesReceived = make(map[int]bool)

	rs.votesReceived[rs.id] = true

	go rs.electionTimeoutTimer.Run()

	logSize := rs.logEntries.GetSize()
	var lastLogIndex int32
	var lastLogTerm int32

	if logSize == 0 {
		lastLogIndex = -1
		lastLogTerm = 0
	} else {
		lastLog := rs.logEntries.GetLastEntry()
		lastLogIndex = lastLog.Index
		lastLogTerm = lastLog.Term
	}

	for i := range len(rs.peers) {
		if i != rs.id {
			fmt.Println("Sending request vote request to", i)
			// Send request vote to peer
			reqVoteReq := &RequestVoteRequest{
				Term:         int32(rs.currentTerm),
				CandidateId:  int32(rs.id),
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}
			raftMsg := &RaftMessage{
				Message: &RaftMessage_RequestVoteRequest{reqVoteReq},
			}

			rs.sendRaftMsg(i, raftMsg)
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

func (rs *RaftServer) handleAppendEntriesRequest(aeMsg *AppendEntriesRequest) {
	fmt.Println("Handling append entries request")
	if int(aeMsg.GetTerm()) > rs.currentTerm {
		rs.currentTerm = int(aeMsg.GetTerm())
		rs.votedFor = -1
	}

	success := false
	if int(aeMsg.GetTerm()) == rs.currentTerm {
		rs.setCurrentState(Follower)
		rs.currentLeader = int(aeMsg.GetLeaderId())

		logChecksOut := false
		if rs.logEntries.GetSize() > aeMsg.PrevLogIndex && (aeMsg.PrevLogIndex == -1 || rs.logEntries.GetEntry(aeMsg.PrevLogIndex).Term == aeMsg.Term) {
			logChecksOut = true
		}

		if logChecksOut {
			success = true
			rs.heartbeatTimeoutTimer.Stop()
			go rs.heartbeatTimeoutTimer.Run()

			// Replace inconsistent logs
			if rs.logEntries.GetLastIndex() > aeMsg.PrevLogIndex {
				rs.logEntries.DeleteEntries(aeMsg.PrevLogIndex + 1)
			}
			for i := 0; i < len(aeMsg.Entries); i++ {
				rs.logEntries.AppendEntry(aeMsg.Entries[i])
			}

			if aeMsg.LeaderCommit > rs.commitIndex {
				rs.commitIndex = min(aeMsg.LeaderCommit, rs.logEntries.GetSize()-1)
				// TODO: Queue committed log entries for application
			}
		}
	}

	// Send AppendEntriesResponse to leader
	appEntriesResp := &AppendEntriesResponse{
		Term:       int32(rs.currentTerm),
		FollowerId: int32(rs.id),
		AckedIdx:   aeMsg.PrevLogIndex + int32(len(aeMsg.Entries)),
		Success:    success,
	}
	raftMsg := &RaftMessage{
		Message: &RaftMessage_AppendEntriesResponse{appEntriesResp},
	}

	rs.sendRaftMsg(int(aeMsg.GetLeaderId()), raftMsg)
}

func (rs *RaftServer) handleAppendEntriesResponse(aerMsg *AppendEntriesResponse) {
	fmt.Println("Handling append entries response with term", aerMsg.GetTerm())
	if int(aerMsg.GetTerm()) == rs.currentTerm && rs.loadCurrentState() == Leader {
		if bool(aerMsg.GetSuccess()) && int(aerMsg.GetAckedIdx()) > rs.ackedIndex[int(aerMsg.GetFollowerId())] {
			rs.nextIndex[int(aerMsg.GetFollowerId())] = int(aerMsg.GetAckedIdx())
			rs.ackedIndex[int(aerMsg.GetFollowerId())] = int(aerMsg.GetAckedIdx()) - 1
			rs.commitLogs()
		} else if rs.nextIndex[int(aerMsg.GetFollowerId())] > 0 {
			rs.nextIndex[int(aerMsg.GetFollowerId())]--
			rs.replicateLogs(rs.id, int(aerMsg.GetFollowerId()))
		}
	} else if int(aerMsg.GetTerm()) > rs.currentTerm {
		fmt.Println("Received append entries response with higher term")
		rs.currentTerm = int(aerMsg.GetTerm())
		rs.setCurrentState(Follower)
		rs.votedFor = -1
	}
}

func (rs *RaftServer) handleRequestVoteRequest(rvMsg *RequestVoteRequest) {
	fmt.Println("Handling request vote request")
	if int(rvMsg.GetTerm()) > rs.currentTerm {
		rs.currentTerm = int(rvMsg.GetTerm())
		rs.setCurrentState(Follower)
		rs.votedFor = -1
	}

	var lastTerm int32 = 0
	logSize := rs.logEntries.GetSize()
	if logSize != 0 {
		lastTerm = rs.logEntries.GetLastEntry().Term
	}

	// Determine whether the candidate log is up to date
	logUpdated := (rvMsg.LastLogTerm > lastTerm) || (rvMsg.LastLogTerm == lastTerm && rvMsg.LastLogIndex >= rs.logEntries.GetLastIndex())
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
	fmt.Println("Handling request vote response")
	if rs.loadCurrentState() == Candidate && int(rvMsg.GetTerm()) == rs.currentTerm && bool(rvMsg.GetVoteGranted()) {
		rs.votesReceived[int(rvMsg.GetVoterId())] = true
		rs.evaluateElection()
	} else if int(rvMsg.GetTerm()) > rs.currentTerm {
		rs.currentTerm = int(rvMsg.GetTerm())
		rs.setCurrentState(Follower)
		rs.votedFor = -1
	}
}

func (rs *RaftServer) sendHeartbeats() {
	for rs.loadCurrentState() == Leader {
		for i := range len(rs.peers) {
			if i != rs.id {
				fmt.Println("Sending heartbeat to", i)
				var prevLogIndex int32 = -1
				var prevLogTerm int32 = 0
				if rs.logEntries.GetSize() != 0 {
					prevLogIndex = rs.logEntries.GetLastEntry().GetIndex()
					prevLogTerm = rs.logEntries.GetLastEntry().Term
				}

				// Create AppendEntries message and send
				appEntriesReq := &AppendEntriesRequest{
					Term:         int32(rs.currentTerm),
					LeaderId:     int32(rs.id),
					PrevLogIndex: prevLogIndex,
					PrevLogTerm:  prevLogTerm,
					LeaderCommit: atomic.LoadInt32(&rs.commitIndex),
				}
				raftMsg := &RaftMessage{
					Message: &RaftMessage_AppendEntriesRequest{appEntriesReq},
				}

				rs.sendRaftMsg(i, raftMsg)
				fmt.Println("Send to", i, "completed")
			}
		}

		time.Sleep(HEARTBEAT_INTERVAL)
	}
}

func (rs *RaftServer) commitLogs() {
	for i := rs.commitIndex + 1; i < rs.logEntries.GetSize(); i++ {
		numAcks := 1
		for _, idx := range rs.ackedIndex {
			if int32(idx) >= i {
				numAcks++
			}

			if float32(numAcks) >= (float32(len(rs.peers))+1.0)/2.0 {
				rs.commitIndex = i
				// TODO: Queue log to be applied
			} else {
				break
			}
		}
	}
	// TODO: Save to permanent storage
}

func (rs *RaftServer) replicateLogs(leaderId, followerId int) {
	var entriesAlreadySent int32 = int32(rs.nextIndex[followerId])
	var toReplicate []*raftlog.LogEntry

	if entriesAlreadySent < rs.logEntries.GetSize() {
		for i := int32(max(0, entriesAlreadySent)); i < rs.logEntries.GetSize(); i++ {
			toReplicate = append(toReplicate, rs.logEntries.GetEntry(i))
		}
	}

	var lastValidTerm int32 = 0
	if entriesAlreadySent > 0 {
		lastValidTerm = rs.logEntries.GetEntry(entriesAlreadySent - 1).Term
	}

	appEntriesReq := &AppendEntriesRequest{
		Term:         int32(rs.currentTerm),
		LeaderId:     int32(leaderId),
		PrevLogIndex: entriesAlreadySent - 1,
		PrevLogTerm:  lastValidTerm,
		LeaderCommit: atomic.LoadInt32(&rs.commitIndex),
		Entries:      toReplicate,
	}
	raftMsg := &RaftMessage{
		Message: &RaftMessage_AppendEntriesRequest{appEntriesReq},
	}

	rs.sendRaftMsg(followerId, raftMsg)
}

func (rs *RaftServer) applyQueuedLogs() {
	for {
		log := <-rs.logApplicationQueue
		fmt.Println("log to apply", log.Command)
		rs.dnsModule.Apply(log.Command)

		if rs.loadCurrentState() == Leader {
			rs.replyToClient("TODO: real output", log.ClientAddr+":"+strconv.Itoa(int(log.ClientPort)))
		}
	}
}

func (rs *RaftServer) evaluateElection() {
	numVotes := 0
	for range rs.votesReceived {
		numVotes++
	}
	fmt.Println(rs.id, "received", numVotes, "votes")
	if float32(numVotes) >= (float32(len(rs.peers))+1.0)/2.0 {
		fmt.Println("Election won")
		rs.setCurrentState(Leader)
		rs.currentLeader = rs.id
	}
}

func (rs *RaftServer) replyToClient(output string, addr string) {
	clientReply := &ClientReply{
		Output:     output,
		LeaderAddr: rs.peers[rs.id].Ip + ":" + rs.peers[rs.id].Port,
		LeaderId:   int32(rs.id),
	}
	raftMsg := &RaftMessage{
		Message: &RaftMessage_ClientReply{clientReply},
	}

	serializedMsg, err := proto.Marshal(raftMsg)
	if err != nil {
		fmt.Println(err)
		return
	}
	rs.net.send(addr, string(serializedMsg))
}

func (rs *RaftServer) sendRaftMsg(targetId int, raftMsg *RaftMessage) {
	serializedMsg, err := proto.Marshal(raftMsg)
	if err != nil {
		fmt.Println(err)
		return
	}

	addr := rs.peers[targetId]
	rs.net.send(addr.Ip+":"+addr.Port, string(serializedMsg))
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
	return time.Duration((rand.Intn(max-min+1) + min))
}
