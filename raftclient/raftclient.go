package raftclient

import (
	"cs244_cs244b/raftnetwork"
	"fmt"
	"strconv"

	pb "cs244_cs244b/raftprotos"

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

	return rc
}

func (rc *RaftClient) SendDNSCommand(dnsCommand pb.DNSCommand) pb.DNSResponse {
	//fmt.Println("1")
	serializedCommand, err := proto.Marshal(&dnsCommand)
	if err != nil {
		fmt.Println(err)
	}
	//fmt.Println("2")
	currentCommandID := rc.commandID
	clientRequest := &pb.ClientRequest{
		CommandID:    rc.commandID,
		Command:      string(serializedCommand),
		ReplyAddress: rc.myIP,
		ReplyPort:    int32(rc.myPort),
	}
	//fmt.Println("3")
	rc.commandID++
	raftMsg := &pb.RaftMessage{
		Message: &pb.RaftMessage_ClientRequest{clientRequest},
	}
	serializedRequest, err := proto.Marshal(raftMsg)
	if err != nil {
		fmt.Println(err)
	}
	//fmt.Println("4")
	rc.sendToRaftLeader(string(serializedRequest))
	//fmt.Println("5")
	for {
		//fmt.Println("6")
		reply, ok := rc.responses[currentCommandID]
		// If the key exists
		if ok {
			return *reply
		}
		fmt.Println("7")
		msg, ok := <-rc.net.MsgQueue
		if ok {
			//fmt.Println("8")
			var clientReply pb.ClientReply
			if err := proto.Unmarshal([]byte(msg), &clientReply); err != nil {
				fmt.Errorf("failed to unmarshal message: %w", err)
			}
			rc.currentLeaderID = int(clientReply.LeaderId)
			if !clientReply.AmLeader {
				//fmt.Println("9")
				if clientReply.CommandID == currentCommandID {
					rc.sendToRaftLeader(string(serializedRequest))
				}
				continue
			}
			//fmt.Println("10")
			var dnsResponse pb.DNSResponse
			if err := proto.Unmarshal([]byte(clientReply.Output), &dnsResponse); err != nil {
				fmt.Errorf("failed to unmarshal message: %w", err)
			}
			//fmt.Println("11")
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
