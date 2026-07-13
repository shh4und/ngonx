package server

import (
	"log"
	"net"
	"os"
	"strconv"
	"sync/atomic"

	"ngonx/internal/response"
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

	file, err := os.Open("static/landing.html")
	if err != nil {
		log.Printf("error at opening file, err: %v", err.Error())
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("error at getting file info, err: %v", err.Error())
		return
	}
	fileSize := int(fileInfo.Size())
	buffer := make([]byte, fileSize)
	_, err = file.Read(buffer)
	if err != nil {
		log.Printf("error at reading file, err: %v", err.Error())
		return
	}
	err = response.WriteResponse(conn, 200, fileSize, nil, nil, nil, nil, nil, nil, buffer)
	if err != nil {
		log.Printf("error at writing response, err: %v", err.Error())
	}
	log.Printf("wrote %d bytes to %s", fileSize, conn.RemoteAddr().String())
}
