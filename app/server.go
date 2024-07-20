package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
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

		filePath := directory + string(os.PathSeparator) + fileName

		if req.Method == "GET" {
			// puts the whole shebang into memory, think about streaming
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				res.NotFound()
				return
			}

			res.File(fileContent)
		}

		if req.Method == "POST" {
			f, err := os.Create(filePath)
			if err != nil {
				fmt.Println("Error creating file", filePath, err)
				res.NotFound()
				return
			}
			written, writeErr := f.Write(req.Body)
			if writeErr != nil {
				fmt.Println("Error writing file", filePath, writeErr)
				res.NotFound()
				return
			}
			fmt.Println("Written bytes", written)
			res.Created()
		}
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
			res := NewResponse(conn, &req)

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
	Body    []byte
}

func NewRequest(conn net.Conn) (Request, error) {
	r := Request{
		Headers: make(map[string]string),
	}
	// parse statusLine line
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading statusline", err)
		os.Exit(1)
	}

	vals := strings.Fields(statusLine)
	r.Method = vals[0]
	r.Path = vals[1]
	r.Version = vals[2]

	// parse headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" {
			break
		}
		headerType, headerValue, ok := strings.Cut(line, ": ")
		if !ok {
			fmt.Println("Malformed header")
			continue
		}
		r.Headers[headerType] = strings.TrimSpace(headerValue)
	}

	if contentLength, ok := r.Headers["Content-Length"]; ok && len(contentLength) > 0 {
		length, err := strconv.Atoi(contentLength)
		if err != nil {
			fmt.Println("Error parsing content length", err)
			os.Exit(1)
		}

		r.Body = make([]byte, length)
		read, err := reader.Read(r.Body)
		if err != nil {
			fmt.Println("Error reading request body", err)
			os.Exit(1)
		}
		fmt.Println("Read bytes", read)

	}

	return r, nil
}

type Response struct {
	req           *Request
	conn          net.Conn
	status        string
	contentType   string
	contentLength int
	body          []byte
}

func NewResponse(conn net.Conn, req *Request) Response {
	return Response{
		req:    req,
		status: "200 OK",
		conn:   conn,
	}
}

func (r *Response) Ok() {
	r.status = "200 OK"
	r.Write()
}

func (r *Response) Created() {
	r.status = "201 Created"
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

func (r Response) compressionEnabled() bool {
	enc, ok := r.req.Headers["Accept-Encoding"]
	if !ok {
		return false
	}

	if enc == "gzip" {
		return true
	}

	if strings.Contains(enc, "gzip,") {
		return true
	}

	return false
}

func (r Response) compressBody(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(body)
	if err != nil {
		return []byte{}, err
	}
	if err := zw.Close(); err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

func (r Response) Write() {
	var resBuilder strings.Builder
	body := r.body

	resBuilder.WriteString(fmt.Sprintf("HTTP/1.1 %s\r\n", r.status))
	if r.contentType != "" {
		resBuilder.WriteString(fmt.Sprintf("Content-Type: %s\r\n", r.contentType))
	}
	compress := r.compressionEnabled()
	if compress {
		resBuilder.WriteString("Content-Encoding: gzip\r\n")
		compressed, err := r.compressBody(r.body)
		if err != nil {
			fmt.Println("Error compressing response")
			r.conn.Close()
			return
		}
		body = compressed
	}

	contentLen := len(body)
	if contentLen != 0 {
		resBuilder.WriteString(fmt.Sprintf("Content-Length: %v\r\n", contentLen))
	}

	resBuilder.WriteString("\r\n")

	if contentLen != 0 {
		resBuilder.WriteString(string(body))
	}

	_, err := fmt.Fprint(r.conn, resBuilder.String())
	r.conn.Close()
	if err != nil {
		fmt.Println("Error sending response")
		os.Exit(1)
	}
}
