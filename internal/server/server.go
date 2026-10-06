package server

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync/atomic"

	"ngonx/internal/headers"
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
		slog.Warn("error at listening", "err", err.Error())
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
		slog.Warn("error at closing listener", "err", err.Error())
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
			slog.Warn("error at accepting connection", "err", err.Error())
			continue
		}

		// same as: go func(conn net.Conn) { s.handle(conn) }(conn)
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	req, err, statusCode := request.RequestFromReader(conn)
	if err != nil {
		response.WriteStatusLine(conn, statusCode)

		connection := "close"
		response.WriteHeaders(conn, headers.Headers{"content-length": "0", "connection": connection})
		slog.Warn("error at parsing request", "err", err.Error())

		return
	}

	// buffer for handler to write reponse
	buf := new(bytes.Buffer)

	handlerErr := s.handler(buf, req)
	if handlerErr != nil {
		buf.Reset() // <-- reset handler error residual bytes from buf
		response.WriteStatusLine(conn, handlerErr.StatusCode)
		response.WriteHeaders(conn, nil)
		return
	}

	body := buf.Bytes()
	err = response.WriteStatusLine(conn, response.StatusOK)
	if err != nil {
		slog.Warn("error at writing status line", "err", err.Error())
		return
	}
	contentLengthStr := strconv.Itoa(len(body))
	err = response.WriteHeaders(conn, headers.Headers{"content-length": contentLengthStr})
	if err != nil {
		slog.Warn("error at writing headers", "err", err.Error())
		return
	}
	_, err = conn.Write(body)
	if err != nil {
		slog.Warn("error at writing body", "err", err.Error())
		return
	}

	slog.Info("wrote", "bytes", len(body), "toaddr", conn.RemoteAddr().String())
}
