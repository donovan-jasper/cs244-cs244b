package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run . <server_id> <server0_address:port> <server1_address:port> ... <serverN_address:port>")
		return
	}

	server_id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	peerAddressesStrs := os.Args[2:]
	var peerAddresses []Address

	for _, str := range peerAddressesStrs {
		parts := strings.Split(str, ":")
		if len(parts) == 2 {
			addr := Address{
				ip:   parts[0],
				port: parts[1],
			}
			peerAddresses = append(peerAddresses, addr)
		} else {
			// TODO: error
		}
	}

	// TODO: Take in shouldRestore from command line
	shouldRestore := false
	rs := NewRaftServer(server_id, peerAddresses, shouldRestore)
	rs.run()
}
