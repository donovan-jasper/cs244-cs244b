package raftserver

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	"goraft/raftserver/raftlog"
	"raftnetwork"
	"raftprotos"
	pb "raftprotos"
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
	peers []raftnetwork.Address

	// Persistent state
	currentTerm         int
	votedFor            int
	logEntries          *raftlog.RaftLog
	persistantVariables *raftlog.WAL

	currentState int32
	lastState    int32

	// Volatile state
	commitIndex   int32
	lastApplied   int
	currentLeader int

	votesReceived map[int]bool
	nextIndex     []int
	ackedIndex    []int

	net *raftnetwork.NetworkModule

	heartbeatTimeoutTimer *Timer
	electionTimeoutTimer  *Timer

	// TODO: Apply logs in background
	logApplicationQueue chan pb.LogEntry
	dnsModule           DNSModule
}

func setStateToCandidateCB(rs *RaftServer) {
	rs.setCurrentState(Candidate)
}

func setStateToFollowerCB(rs *RaftServer) {
	rs.setCurrentState(Follower)
}

func NewRaftServer(id int, peers []raftnetwork.Address, backupFilepath string, restoreFromDisk bool) *RaftServer {
	rs := new(RaftServer)
	rs.id = id
	rs.peers = peers
	rs.currentTerm = 0
	rs.votedFor = -1

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

	rs.persistantVariables = raftlog.NewWAL(filepath.Join(backupFilepath, strconv.Itoa(id)+"-state"), restoreFromDisk)
	rs.logEntries = raftlog.NewRaftLog(filepath.Join(backupFilepath, strconv.Itoa(id)), restoreFromDisk)

	if restoreFromDisk {
		rs.loadPersistentVariables()
	}

	rs.net = raftnetwork.NewNetworkModule()

	rs.logApplicationQueue = make(chan pb.LogEntry)
	rs.dnsModule = *NewDNSModule()

	rs.heartbeatTimeoutTimer = NewTimer(randomDuration(HEARTBEAT_TIMEOUT_MIN, HEARTBEAT_TIMEOUT_MAX), rs, setStateToCandidateCB)
	rs.electionTimeoutTimer = NewTimer(randomDuration(ELECTION_TIMEOUT_MIN, ELECTION_TIMEOUT_MAX), rs, setStateToFollowerCB)

	return rs
}

func (rs *RaftServer) Run() {
	go rs.net.Listen(rs.peers[rs.id].Port)
	go rs.applyQueuedLogs()

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
			case msg, ok := <-rs.net.MsgQueue:
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
			reqVoteReq := &raftprotos.RequestVoteRequest{
				Term:         int32(rs.currentTerm),
				CandidateId:  int32(rs.id),
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}
			raftMsg := &pb.RaftMessage{
				Message: &pb.RaftMessage_RequestVoteRequest{reqVoteReq},
			}
			rs.savePersistentVariables()
			rs.sendRaftMsg(i, raftMsg)
		}
	}

	rs.evaluateElection()
}

func (rs *RaftServer) handleMessage(msg string) {
	var raftMsg pb.RaftMessage
	if err := proto.Unmarshal([]byte(msg), &raftMsg); err != nil {
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
	case *pb.RaftMessage_ClientRequest:
		rs.handleClientRequest(raftMsg.GetClientRequest())
	default:
		fmt.Errorf("unknown message type")
	}
}

func (rs *RaftServer) handleAppendEntriesRequest(aeMsg *pb.AppendEntriesRequest) {
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
				fmt.Println("Appending new entry from AE RPC")
				rs.logEntries.AppendEntry(aeMsg.Entries[i])
			}

			if aeMsg.LeaderCommit > rs.commitIndex {
				rs.commitIndex = min(aeMsg.LeaderCommit, rs.logEntries.GetSize()-1)
				for i := rs.lastApplied + 1; i < int(rs.commitIndex); i++ {
					rs.logApplicationQueue <- *rs.logEntries.GetEntry(int32(i))
				}
			}
		}
	}

	// Send AppendEntriesResponse to leader
	appEntriesResp := &pb.AppendEntriesResponse{
		Term:       int32(rs.currentTerm),
		FollowerId: int32(rs.id),
		AckedIdx:   aeMsg.PrevLogIndex + int32(len(aeMsg.Entries)),
		Success:    success,
	}
	raftMsg := &pb.RaftMessage{
		Message: &pb.RaftMessage_AppendEntriesResponse{appEntriesResp},
	}
	rs.savePersistentVariables()
	rs.sendRaftMsg(int(aeMsg.GetLeaderId()), raftMsg)
}

func (rs *RaftServer) handleAppendEntriesResponse(aerMsg *pb.AppendEntriesResponse) {
	fmt.Println("Handling append entries response with term", aerMsg.GetTerm())
	if int(aerMsg.GetTerm()) == rs.currentTerm && rs.loadCurrentState() == Leader {
		if bool(aerMsg.GetSuccess()) && int(aerMsg.GetAckedIdx()) > rs.ackedIndex[int(aerMsg.GetFollowerId())] {
			rs.nextIndex[int(aerMsg.GetFollowerId())] = int(aerMsg.GetAckedIdx()) + 1
			rs.ackedIndex[int(aerMsg.GetFollowerId())] = int(aerMsg.GetAckedIdx())
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

func (rs *RaftServer) handleRequestVoteRequest(rvMsg *pb.RequestVoteRequest) {
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

	requestVoteReplyMsg := &pb.RequestVoteResponse{
		Term:        int32(rs.currentTerm),
		VoteGranted: vote,
		VoterId:     int32(rs.id),
	}

	raftMsg := &pb.RaftMessage{
		Message: &pb.RaftMessage_RequestVoteResponse{requestVoteReplyMsg},
	}
	rs.savePersistentVariables()
	rs.sendRaftMsg(int(rvMsg.GetCandidateId()), raftMsg)

}

func (rs *RaftServer) handleRequestVoteResponse(rvMsg *pb.RequestVoteResponse) {
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
				appEntriesReq := &pb.AppendEntriesRequest{
					Term:         int32(rs.currentTerm),
					LeaderId:     int32(rs.id),
					PrevLogIndex: prevLogIndex,
					PrevLogTerm:  prevLogTerm,
					LeaderCommit: atomic.LoadInt32(&rs.commitIndex),
				}
				raftMsg := &pb.RaftMessage{
					Message: &pb.RaftMessage_AppendEntriesRequest{appEntriesReq},
				}
				rs.savePersistentVariables()
				rs.sendRaftMsg(i, raftMsg)
			}
		}

		time.Sleep(HEARTBEAT_INTERVAL)
	}
}

func (rs *RaftServer) handleClientRequest(crMsg *pb.ClientRequest) {
	fmt.Println("Handling client request")
	if rs.loadCurrentState() == Leader {
		fmt.Println("We are leader, so add client command to log")
		newLogEntry := &pb.LogEntry{}
		newLogEntry.Term = int32(rs.currentTerm)
		newLogEntry.Command = crMsg.Command

		logIndex := 0
		if rs.logEntries.GetSize() != 0 {
			logIndex = int(rs.logEntries.GetLastEntry().Index) + 1
		}
		newLogEntry.Index = int32(logIndex)

		newLogEntry.ClientAddr = crMsg.ReplyAddress
		newLogEntry.ClientPort = crMsg.ReplyPort

		rs.logEntries.AppendEntry(newLogEntry)

		if len(rs.peers) == 1 {
			rs.logApplicationQueue <- *newLogEntry
			fmt.Println("We are the only server, so queued entry to apply")
		}

		for followerId := 0; followerId < len(rs.peers); followerId++ {
			if followerId != rs.id {
				rs.replicateLogs(rs.id, followerId)
			}
		}
	} else {
		fmt.Println("We are not leader, so redirect client command")
		rs.replyToClient([]byte("Not the leader"), false, crMsg.ReplyAddress+":"+strconv.Itoa(int(crMsg.ReplyPort)))
	}

}

func (rs *RaftServer) commitLogs() {
	fmt.Println("Committing new logs")
	for i := rs.commitIndex + 1; i < rs.logEntries.GetSize(); i++ {
		numAcks := 1
		for _, idx := range rs.ackedIndex {
			if int32(idx) >= i {
				numAcks++
			}
		}
		if float32(numAcks) >= (float32(len(rs.peers))+1.0)/2.0 {
			rs.commitIndex = i
			rs.logApplicationQueue <- *rs.logEntries.GetEntry(int32(i))
		} else {
			break
		}
	}
	rs.savePersistentVariables()
}

func (rs *RaftServer) replicateLogs(leaderId, followerId int) {
	fmt.Println("Replicating log to followers")
	var entriesAlreadySent int32 = int32(rs.nextIndex[followerId])
	var toReplicate []*pb.LogEntry

	if entriesAlreadySent < rs.logEntries.GetSize() {
		for i := int32(max(0, entriesAlreadySent)); i < rs.logEntries.GetSize(); i++ {
			toReplicate = append(toReplicate, rs.logEntries.GetEntry(i))
		}
	}

	var lastValidTerm int32 = 0
	if entriesAlreadySent > 0 {
		lastValidTerm = rs.logEntries.GetEntry(entriesAlreadySent - 1).Term
	}

	appEntriesReq := &pb.AppendEntriesRequest{
		Term:         int32(rs.currentTerm),
		LeaderId:     int32(leaderId),
		PrevLogIndex: entriesAlreadySent - 1,
		PrevLogTerm:  lastValidTerm,
		LeaderCommit: atomic.LoadInt32(&rs.commitIndex),
		Entries:      toReplicate,
	}
	raftMsg := &pb.RaftMessage{
		Message: &pb.RaftMessage_AppendEntriesRequest{appEntriesReq},
	}

	rs.sendRaftMsg(followerId, raftMsg)
}

func (rs *RaftServer) applyQueuedLogs() {
	for {
		fmt.Println("Waiting for log to apply")
		log := <-rs.logApplicationQueue
		fmt.Println("log to apply", log.Command)
		clientResponse := rs.dnsModule.Apply(log.Command)

		if rs.loadCurrentState() == Leader {
			rs.replyToClient(clientResponse, true, log.ClientAddr+":"+strconv.Itoa(int(log.ClientPort)))
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

func (rs *RaftServer) replyToClient(output []byte, isLeader bool, addr string) {
	fmt.Println("Replying to client")
	clientReply := &pb.ClientReply{
		Output:   output,
		AmLeader: isLeader,
		LeaderId: int32(rs.currentLeader),
	}
	serializedMsg, err := proto.Marshal(clientReply)
	if err != nil {
		fmt.Println(err)
		return
	}
	rs.savePersistentVariables()
	rs.net.Send(addr, string(serializedMsg))
}

func (rs *RaftServer) sendRaftMsg(targetId int, raftMsg *pb.RaftMessage) {
	serializedMsg, err := proto.Marshal(raftMsg)
	if err != nil {
		fmt.Println(err)
		return
	}

	addr := rs.peers[targetId]
	rs.net.Send(addr.Ip+":"+addr.Port, string(serializedMsg))
}

func (rs *RaftServer) loadPersistentVariables() {
	variables, _ := rs.persistantVariables.ReadState()
	splitVars := strings.Split(variables, "\n")
	if len(splitVars) != 2 {
		fmt.Println("Error: Failed to load variables from file")
	}

	currTerm, err := strconv.Atoi(splitVars[0])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	rs.currentTerm = currTerm

	votedFor, err := strconv.Atoi(splitVars[1])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	rs.votedFor = votedFor
}

func (rs *RaftServer) savePersistentVariables() {
	variables := strconv.Itoa(rs.currentTerm) + "\n" + strconv.Itoa(rs.votedFor)
	rs.persistantVariables.ClearState()
	rs.persistantVariables.WriteData([]byte(variables))
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
