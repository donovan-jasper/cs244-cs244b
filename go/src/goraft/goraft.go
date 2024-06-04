package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goraft/raftserver"
	"raftnetwork"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run . <should_restore> <server_id> <server0_address:port> <server1_address:port> ... <serverN_address:port>")
		return
	}

	shouldRestore, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		fmt.Println("Error:", err)
	}

	server_id, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	peerAddressesStrs := os.Args[3:]
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

	rs := raftserver.NewRaftServer(server_id, peerAddresses, filepath.Join(backupDir, strconv.Itoa(server_id)), shouldRestore)
	rs.Run()
}
