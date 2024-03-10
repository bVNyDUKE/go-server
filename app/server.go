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
	s := NewServer(ADDR, PORT)

	s.AddHandler("/", func(res Response, req *Request) {
		fmt.Fprintf(res, "HTTP/1.1 200 OK\r\n\r\n")
	})
	s.AddHandler("/echo", func(res Response, req *Request) {
		if _, match, found := strings.Cut(req.Path, "/echo/"); found {
			res.text(match)
		}
	})
	s.AddHandler("/user-agent", func(res Response, req *Request) {
		res.text(req.Headers["User-Agent"])
	})

	if err := s.Run(); err != nil {
		os.Exit(1)
	}
}

type ReqHandler func(Response, *Request)

type Server struct {
	Address  string
	Port     string
	handlers map[string]ReqHandler
}

func NewServer(address, port string) Server {
	return Server{
		Address:  address,
		Port:     port,
		handlers: make(map[string]ReqHandler),
	}
}

func (s *Server) AddHandler(path string, handler ReqHandler) {
	s.handlers[path] = handler
}

func (s *Server) getHandlerFromPath(path string) (ReqHandler, bool) {
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return s.handlers["/"], true
	}
	handler, ok := s.handlers["/"+segments[1]]

	return handler, ok
}

func (s *Server) Run() error {
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
			return err
		}
		go func(conn net.Conn) {
			req, err := NewRequest(conn)
			if err != nil {
				fmt.Println("Error making request object: ", err.Error())
			}
			res := NewResponse(conn)

			fmt.Println("Handling connection", conn.RemoteAddr().String())
			handler, ok := s.getHandlerFromPath(req.Path)
			if !ok {
				res.sendNotFound()
				return
			}
			handler(res, &req)
		}(conn)
	}
}

type Request struct {
	Method  string
	Path    string
	Version string
	Headers map[string]string
}

func NewRequest(conn net.Conn) (Request, error) {
	r := Request{
		Headers: make(map[string]string),
	}
	// parse statusLine line
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		if scanner.Text() == "" {
			break
		}

		// parse status line
		if r.Method == "" {
			vals := strings.Fields(scanner.Text())
			r.Method = vals[0]
			r.Path = vals[1]
			r.Version = vals[2]
			continue
		}

		// parse headers
		headerType, headerValue, ok := strings.Cut(scanner.Text(), ": ")
		if !ok {
			fmt.Println("Malformed header")
			continue
		}
		fmt.Println("HEADER", headerType, headerValue)
		r.Headers[headerType] = headerValue
	}

	return r, nil
}

type Response struct {
	conn net.Conn
}

func NewResponse(conn net.Conn) Response {
	return Response{
		conn: conn,
	}
}

func (r *Response) sendNotFound() {
	fmt.Fprintf(r.conn, "HTTP/1.1 404 Not Found\r\n\r\n")
	r.conn.Close()
}

func (r *Response) text(content string) {
	_, _ = r.Write([]byte(content))
}

func (r Response) Write(content []byte) (int, error) {
	return fmt.Fprintf(r.conn, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %v\r\n\r\n%s\r\n", len(content), content)
}
