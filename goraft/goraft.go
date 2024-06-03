package main

import (
	"cs244_cs244b/goraft/raftserver"
	"cs244_cs244b/raftnetwork"
	"fmt"
	"os"
	"path/filepath"
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
	var peerAddresses []raftnetwork.Address

	for _, str := range peerAddressesStrs {
		parts := strings.Split(str, ":")
		if len(parts) == 2 {
			addr := raftnetwork.Address{
				Ip:   parts[0],
				Port: parts[1],
			}
			peerAddresses = append(peerAddresses, addr)
		} else {
			// TODO: error
		}
	}

	backupDir := "./backups"

	// TODO: Take in shouldRestore from command line
	shouldRestore := false
	rs := raftserver.NewRaftServer(server_id, peerAddresses, filepath.Join(backupDir, strconv.Itoa(server_id)), shouldRestore)
	rs.Run()
}
