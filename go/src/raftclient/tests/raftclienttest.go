package main

import (
	"fmt"
	"os"
	"raftclient"
	pb "raftprotos"
	"strconv"
	"time"

	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run . <my_ip> <my_port> <server0_address:port> <server1_address:port> ... <serverN_address:port>")
		return
	}

	my_addr := os.Args[1]

	my_port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	peerAddresses := os.Args[3:]

	rc := raftclient.NewRaftClient(peerAddresses, my_addr, my_port)

	var dnsCommandType int32 = 2

	dnsCommand := pb.DNSCommand{
		CommandType: dnsCommandType, //Add record
		Domain:      "yeah.com",
		Hostname:    "yeah.com",
		Ip:          "127.0.0.1",
		Ttl:         durationpb.New(time.Duration(60 * 1e9)),
	}

	response := rc.SendDNSCommand(dnsCommand)
	fmt.Println(response.Success)

	if response.Success && dnsCommandType == 2 {
		dnsRecord := response.DnsRecord
		fmt.Println(dnsRecord.Hostname, dnsRecord.Ip, dnsRecord.Ttl, dnsRecord.Added)
	}

}
