package main

import (
	"fmt"
	"net"
)

type Address struct {
	ip   string
	port string
}

type NetworkModule struct {
	msgQueue chan string
}

func NewNetworkModule() *NetworkModule {
	return &NetworkModule{
		msgQueue: make(chan string)
	}
} 

func (n *Network) handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading:", err.Error())
		return
	}

	n <- string(buf[:n])

	fmt.Printf("Received message from client: %s\n", string(buf[:n]))
}

func (n *Network) listen(port string) {

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
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

		go handleConnection(conn)
	}
}

func (n *Network) send(serverAddr string, message string) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("Error connecting to server:", err.Error())
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Println("Error writing to server:", err.Error())
		return
	}

	fmt.Println("Message sent to server:", message)
}
