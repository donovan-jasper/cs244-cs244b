package raftclient

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"raftnetwork"
	pb "raftprotos"

	"google.golang.org/protobuf/proto"
)

type RaftClient struct {
	peerAddresses []string

	myIP   string
	myPort int

	currentLeaderID int

	net *raftnetwork.NetworkModule

	commandID int32

	responses map[int32]*pb.DNSResponse
}

func NewRaftClient(peers []string, ip string, port int) *RaftClient {
	rc := new(RaftClient)
	rc.peerAddresses = peers
	rc.myIP = ip
	rc.myPort = port

	rc.commandID = 0

	rc.net = raftnetwork.NewNetworkModule()
	go rc.net.Listen(strconv.Itoa(rc.myPort))

	rc.responses = make(map[int32]*pb.DNSResponse)

	return rc
}

func (rc *RaftClient) SendDNSCommand(dnsCommand pb.DNSCommand) pb.DNSResponse {

	serializedCommand, err := proto.Marshal(&dnsCommand)
	if err != nil {
		slog.Error("failed to marshal command", "error", err)
	}

	currentCommandID := rc.commandID
	clientRequest := &pb.ClientRequest{
		CommandID:    rc.commandID,
		Command:      serializedCommand,
		ReplyAddress: rc.myIP,
		ReplyPort:    int32(rc.myPort),
	}
	rc.commandID++
	raftMsg := &pb.RaftMessage{
		Message: &pb.RaftMessage_ClientRequest{clientRequest},
	}
	serializedRequest, err := proto.Marshal(raftMsg)
	if err != nil {
		slog.Error("error trying to marshal request", "error", err)
	}
	slog.Info("First send to raft cluster for ", "id", currentCommandID)
	rc.sendToRaftLeader(string(serializedRequest))
	for {
		reply, ok := rc.responses[currentCommandID]
		// If the key exists
		if ok {
			fmt.Println("Received command response")
			return *reply
		}
		slog.Info("Waiting for response from raft cluster with command ID", "id", currentCommandID)
		msg, ok := <-rc.net.MsgQueue
		slog.Info("Received response from raft cluster")
		if ok {
			var clientReply pb.ClientReply
			if err := proto.Unmarshal([]byte(msg), &clientReply); err != nil {
				slog.Error("failed to unmarshal message: %w", "error", err)
			}
			slog.Info("Response Command ID:", "id", clientReply.CommandID)
			rc.currentLeaderID = int(clientReply.LeaderId)
			if !clientReply.AmLeader {
				fmt.Println("Received response from non leader")
				if clientReply.CommandID == currentCommandID {
					fmt.Println("Resending to leader")
					rc.sendToRaftLeader(string(serializedRequest))
					time.Sleep(1 * time.Second)
				}
				continue
			}
			var dnsResponse pb.DNSResponse
			if err := proto.Unmarshal([]byte(clientReply.Output), &dnsResponse); err != nil {
				slog.Error("failed to unmarshal message: %w", "error", err)
			}
			slog.Info("DNS Response: ", "success", dnsResponse.Success)
			rc.responses[clientReply.CommandID] = &dnsResponse
		} else {
			slog.Info("Channel closed!")
		}
	}

}

func (rc *RaftClient) sendToRaftLeader(msg string) {
	success := rc.net.Send(rc.peerAddresses[rc.currentLeaderID], string(msg))

	for !success {
		for _, addr := range rc.peerAddresses {
			success = rc.net.Send(addr, string(msg))
			if success {
				break
			}
		}
	}
}
