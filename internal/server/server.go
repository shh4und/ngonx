package server

import (
	"bytes"
	"io"
	"log"
	"net"
	"strconv"
	"sync/atomic"

	"ngonx/internal/request"
	"ngonx/internal/response"
)

type Handler func(w io.Writer, req *request.Request) *HandlerError

type HandlerError struct {
	StatusCode response.StatusCode
	Message    string
}

type Server struct {
	port       int
	listener   net.Listener
	inShutdown atomic.Bool
	handler    Handler
}

func Serve(port int, handler Handler) (*Server, error) {

	addressPort := strconv.Itoa(port)

	listener, err := net.Listen("tcp", ":"+addressPort)
	if err != nil {
		log.Printf("error at listening, err: %v", err.Error())
		return nil, err
	}
	server := &Server{
		port:     port,
		listener: listener,
		handler:  handler,
	}
	server.inShutdown.Store(false)

	go server.listen()

	return server, nil
}

func (s *Server) Close() error {
	s.inShutdown.Store(true)
	err := s.listener.Close()
	if err != nil {
		log.Printf("error at closing listener, err: %v", err.Error())
	}

	return err
}

func (s *Server) listen() {
	for !s.inShutdown.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.inShutdown.Load() {
				return
			}
			log.Printf("error at accepting connection, err: %v", err.Error())
			continue
		}

		// same as: go func(conn net.Conn) { s.handle(conn) }(conn)
		go s.handle(conn)
	}
}

func writeHandlerError(w io.Writer, handlerErr *HandlerError) {
	body := []byte(handlerErr.Message)
	response.WriteStatusLine(w, handlerErr.StatusCode)
	response.WriteHeaders(w, len(body), nil, nil, nil, nil, nil, nil)
	w.Write(body)
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	req, err := request.RequestFromReader(conn)
	if err != nil {
		log.Printf("error at parsing request, err: %v", err.Error())
		return
	}

	// buffer for handler to write reponse
	buf := new(bytes.Buffer)

	handlerErr := s.handler(buf, req)
	if handlerErr != nil {
		writeHandlerError(buf, handlerErr)
		body := buf.Bytes()
		_, err = conn.Write(body)
		if err != nil {
			log.Printf("%v: %v", response.ErrWritingBody, err.Error())
			return
		}
		return
	}

	body := buf.Bytes()
	err = response.WriteStatusLine(conn, response.StatusOK)
	if err != nil {
		log.Printf("error at writing status line, err: %v", err.Error())
		return
	}
	err = response.WriteHeaders(conn, len(body), nil, nil, nil, nil, nil, nil)
	if err != nil {
		log.Printf("%v: %v", response.ErrWritingHeaders, err.Error())
		return
	}
	_, err = conn.Write(body)
	if err != nil {
		log.Printf("%v: %v", response.ErrWritingBody, err.Error())
		return
	}

	log.Printf("wrote %d bytes to %s", len(body), conn.RemoteAddr().String())
}
