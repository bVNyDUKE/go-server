package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	ADDR = "0.0.0.0"
	PORT = "4221"
)

var directory string

func init() {
	flag.StringVar(&directory, "directory", "", "directory to look into for files")
	flag.Parse()
}

func main() {
	s := NewServer(ADDR, PORT)

	s.AddHandler("/", func(res Response, req *Request) {
		res.Ok()
	})
	s.AddHandler("/echo", func(res Response, req *Request) {
		if _, match, found := strings.Cut(req.Path, "/echo/"); found {
			res.Text(match)
		}
	})
	s.AddHandler("/user-agent", func(res Response, req *Request) {
		res.Text(req.Headers["User-Agent"])
	})
	s.AddHandler("/files", func(res Response, req *Request) {
		if directory == "" {
			res.NotFound()
			return
		}

		_, fileName, found := strings.Cut(req.Path, "/files/")
		if !found || fileName == "" {
			res.NotFound()
			return
		}

		// puts the whole shebang into memory, think about streaming
		fileContent, err := os.ReadFile(directory + string(os.PathSeparator) + fileName)
		if err != nil {
			res.NotFound()
			return
		}

		res.File(fileContent)
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
				res.NotFound()
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
		r.Headers[headerType] = headerValue
	}

	return r, nil
}

type Response struct {
	conn          net.Conn
	status        string
	contentType   string
	contentLength int
	body          []byte
}

func NewResponse(conn net.Conn) Response {
	return Response{
		status: "200 OK",
		conn:   conn,
	}
}

func (r *Response) Ok() {
	r.status = "200 OK"
	r.Write()
}

func (r *Response) NotFound() {
	r.status = "404 Not Found"
	r.Write()
}

func (r *Response) Text(content string) {
	r.contentType = "text/plain"
	r.contentLength = len(content)
	r.body = []byte(content)
	r.Write()
}

func (r *Response) File(fileContent []byte) {
	r.contentType = "application/octet-stream"
	r.contentLength = len(fileContent)
	r.body = fileContent
	r.Write()
}

func (r Response) Write() {
	var resBuilder strings.Builder

	resBuilder.WriteString(fmt.Sprintf("HTTP/1.1 %s\r\n", r.status))
	if r.contentType != "" {
		resBuilder.WriteString(fmt.Sprintf("Content-Type: %s\r\n", r.contentType))
	}

	if r.contentLength != 0 {
		resBuilder.WriteString(fmt.Sprintf("Content-Length: %v\r\n", r.contentLength))
	}

	resBuilder.WriteString("\r\n")

	if len(r.body) != 0 {
		resBuilder.WriteString(string(r.body))
	}

	_, err := fmt.Fprint(r.conn, resBuilder.String())
	r.conn.Close()
	if err != nil {
		fmt.Println("Error sending response")
		os.Exit(1)
	}
}
