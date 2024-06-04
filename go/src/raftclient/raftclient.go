package raftclient

import (
	"fmt"
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
		fmt.Println(err)
	}

	currentCommandID := rc.commandID
	clientRequest := &pb.ClientRequest{
		CommandID:    rc.commandID,
		Command:      string(serializedCommand),
		ReplyAddress: rc.myIP,
		ReplyPort:    int32(rc.myPort),
	}
	rc.commandID++
	raftMsg := &pb.RaftMessage{
		Message: &pb.RaftMessage_ClientRequest{clientRequest},
	}
	serializedRequest, err := proto.Marshal(raftMsg)
	if err != nil {
		fmt.Println(err)
	}
	rc.sendToRaftLeader(string(serializedRequest))
	for {
		reply, ok := rc.responses[currentCommandID]
		// If the key exists
		if ok {
			return *reply
		}
		msg, ok := <-rc.net.MsgQueue
		if ok {
			var clientReply pb.ClientReply
			if err := proto.Unmarshal([]byte(msg), &clientReply); err != nil {
				fmt.Errorf("failed to unmarshal message: %w", err)
			}
			rc.currentLeaderID = int(clientReply.LeaderId)
			if !clientReply.AmLeader {

				if clientReply.CommandID == currentCommandID {
					rc.sendToRaftLeader(string(serializedRequest))
					time.Sleep(1 * time.Second)
				}
				continue
			}
			var dnsResponse pb.DNSResponse
			if err := proto.Unmarshal([]byte(clientReply.Output), &dnsResponse); err != nil {
				fmt.Errorf("failed to unmarshal message: %w", err)
			}
			rc.responses[clientReply.CommandID] = &dnsResponse
		} else {
			fmt.Println("Channel closed!")
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
