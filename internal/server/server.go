package server

import (
	"log"
	"net"
	"strconv"
	"sync/atomic"
)

type Server struct {
	port       int
	listener   net.Listener
	inShutdown atomic.Bool
}

func Serve(port int) (*Server, error) {
	server := &Server{
		inShutdown: atomic.Bool{},
	}

	server.inShutdown.Store(false)

	addressPort := strconv.Itoa(port)

	listener, err := net.Listen("tcp", ":"+addressPort)
	if err != nil {
		log.Printf("error at listening, err: %v", err.Error())
	}
	server.port = port
	server.listener = listener

	go func() {
		server.listen()
	}()

	return server, nil
}

func (s *Server) Close() error {
	err := s.listener.Close()
	if err != nil {
		log.Printf("error at closing listener, err: %v", err.Error())
	}

	s.inShutdown.Store(true)
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

		go func(conn net.Conn) {
			s.handle(conn)
		}(conn)

	}
}

func (s *Server) handle(conn net.Conn) {

	response := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nHello <b>World!!</b>")

	n, err := conn.Write(response)
	if err != nil {
		log.Printf("error at writing response, err: %v", err.Error())
	}
	log.Printf("wrote %d bytes", n)
	err = conn.Close()
	if err != nil {
		log.Printf("error at closing connection, err: %v", err.Error())
	}
	s.inShutdown.Store(true)
}
