package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run goraft.go <server_id> <server0_address:port> <server1_address:port> ... <serverN_address:port>")
		return
	}

	server_id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	peerAddresses := os.Args[2:]

	shouldRestore := false
	rs := NewRaftServer(server_id, peerAddresses, shouldRestore)
	rs.run()
}
