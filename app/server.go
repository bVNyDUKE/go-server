package main

import (
	"fmt"
	"net"
	"os"
)

func handleConnection(conn net.Conn) {
	fmt.Println("Handling connection", conn.RemoteAddr().String())
	fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\n\r\n")
}

func main() {
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		fmt.Println("accepting a connection from", conn.RemoteAddr().String())
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(conn)
	}
}
