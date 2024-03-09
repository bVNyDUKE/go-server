package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	ADDR = "0.0.0.0"
	PORT = "4221"
)

func main() {
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", ADDR+":"+PORT)
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
		req, err := getRequest(conn)
		if err != nil {
			fmt.Println("Error making request object: ", err.Error())
			os.Exit(1)
		}

		fmt.Println("Handling connection", conn.RemoteAddr().String())
		go handleReq(req)
	}
}

func handleReq(req request) {
	if req.Path == "/" {
		req.sendOk()
	} else {
		req.sendNotFound()
	}
}

type request struct {
	Method  string
	Path    string
	Version string
	Conn    net.Conn
}

func (r *request) sendOk() {
	fmt.Fprintf(r.Conn, "HTTP/1.1 200 OK\r\n\r\n")
}

func (r *request) sendNotFound() {
	fmt.Fprintf(r.Conn, "HTTP/1.1 404 Not Found\r\n\r\n")
}

func getRequest(conn net.Conn) (request, error) {
	header, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return request{}, err
	}
	vals := strings.Fields(header)

	r := request{
		Method:  vals[0],
		Path:    vals[1],
		Version: vals[2],
		Conn:    conn,
	}

	return r, nil
}
