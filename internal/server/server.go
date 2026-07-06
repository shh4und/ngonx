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

	addressPort := strconv.Itoa(port)

	listener, err := net.Listen("tcp", ":"+addressPort)
	if err != nil {
		log.Printf("error at listening, err: %v", err.Error())
		return nil, err
	}
	server := &Server{
		port:     port,
		listener: listener,
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

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	response := []byte("HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"Content-Length: 116\r\n" +
		"Connection: close\r\n\r\n" +
		"<!DOCTYPE html>\r\n" +
		"<html>\r\n" +
		"<head>\r\n" +
		"<title>Hello World</title>\r\n" +
		"</head>\r\n" +
		"<body>\r\n" +
		"Hello <b>World!!</b>\r\n" +
		"</body>\r\n" +
		"</html>")

	n, err := conn.Write(response)
	if err != nil {
		log.Printf("error at writing response, err: %v", err.Error())
	}
	log.Printf("wrote %d bytes to %s", n, conn.RemoteAddr().String())
}
