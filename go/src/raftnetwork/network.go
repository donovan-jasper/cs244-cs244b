package raftnetwork

import (
	"fmt"
	"log"
	"net"
)

type Address struct {
	Ip   string
	Port string
}

type NetworkModule struct {
	MsgQueue chan string
}

func NewNetworkModule() *NetworkModule {
	return &NetworkModule{
		MsgQueue: make(chan string),
	}
}

func (n *NetworkModule) handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 4096)
	msg, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading:", err.Error())
		return
	}

	n.MsgQueue <- string(buf[:msg])
}

func (n *NetworkModule) Listen(port string) {

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		// prob fix later, keeping above sdout print for now
		log.Fatalf("Error listening: %v", err)
		return
	}
	defer ln.Close()

	fmt.Println("Server listening on port", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err.Error())
			continue
		}

		go n.handleConnection(conn)
	}
}

func (n *NetworkModule) Send(serverAddr string, message string) bool {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("Error connecting to server:", err.Error())
		return false
	}
	defer conn.Close()

	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Println("Error writing to server:", err.Error())
		return false
	}
	return true
}
