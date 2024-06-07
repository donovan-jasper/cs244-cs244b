package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"goraft/raftserver"
	"raftnetwork"
)

func main() {
	var shouldRestore bool
	var backupDir string
	var electionTimeoutMin int
	var electionTimeoutMax int
	var heartbeatTimeoutMax int
	var heartbeatTimeoutMin int
	var interval int
	var seperateBackupDir string
	flag.BoolVar(&shouldRestore, "restore", false, "Restore from backup")
	flag.StringVar(&backupDir, "backup", "./backups", "Backup directory for the server")
	flag.StringVar(&seperateBackupDir, "seperate-backup", "", "Seperate backup to load from. Ignores -restore flag")
	flag.IntVar(&electionTimeoutMin, "electionTimeoutMin", 150, "Minimum timeout for the server (ms)")
	flag.IntVar(&electionTimeoutMax, "electionTimeoutMax", 300, "Minimum timeout for the server (ms)")
	flag.IntVar(&heartbeatTimeoutMin, "hearteatTimeoutMin", 150, "Min heartbeat timeout for the server (ms)")
	flag.IntVar(&heartbeatTimeoutMax, "heartbeatTimeoutMax", 300, "Max heartbeat timeout for the server (ms)")
	flag.IntVar(&interval, "interval", 75, "Heartbeat interval for the server (ms)")
	flag.Usage = func() {
		w := flag.CommandLine.Output() // may be os.Stderr - but not necessarily
		fmt.Fprintf(w, "Usage of %s: <server_id> <server0_address:port> <server1_address:port> ... <serverN_address:port> \n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if len(flag.Args()) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	server_id, err := strconv.Atoi(flag.Args()[0])
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	peerAddressesStrs := flag.Args()[1:]
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
			log.Fatalf("Error: Invalid address + port combination %s", str)
			os.Exit(1)
		}
	}

	const MILLISECONDS_TO_NANOSECONDS = 1000000
	config := raftserver.RaftServerConfig{
		ID:                       server_id,
		PeerAddresses:            peerAddresses,
		BackupFilepath:           backupDir,
		RestoreFromDisk:          shouldRestore,
		ElectionTimeoutMin:       electionTimeoutMin * MILLISECONDS_TO_NANOSECONDS,
		ElectionTimeoutMax:       electionTimeoutMax * MILLISECONDS_TO_NANOSECONDS,
		HeartbeatTimeoutMin:      heartbeatTimeoutMin * MILLISECONDS_TO_NANOSECONDS,
		HeartbeatTimeoutMax:      heartbeatTimeoutMax * MILLISECONDS_TO_NANOSECONDS,
		HeartbeatTimeoutInterval: interval * MILLISECONDS_TO_NANOSECONDS,
	}
	log.Printf("%+v\n", config)
	rs := raftserver.NewRaftServerFromConfig(config)
	rs.Run()
}
