package server

import (
	"bytes"
	"io"
	"log/slog"
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

func writeHandlerError(w io.Writer, handlerErr *HandlerError) {
	body := []byte(handlerErr.Message)
	response.WriteStatusLine(w, handlerErr.StatusCode)
	response.WriteHeaders(w, len(body), nil, nil, nil, nil, nil, nil)
	w.Write(body)
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	req, err, statusCode := request.RequestFromReader(conn)
	if err != nil {
		response.WriteStatusLine(conn, statusCode)

		connection := "close"
		response.WriteHeaders(conn, len("Not supported"), nil, &connection, nil, nil, nil, nil)
		conn.Write([]byte("Not supported"))
		slog.Warn("error at parsing request", "err", err.Error())

		// if err != nil {
		// 	slog.Warn("error at writing body for RequestFromReader error response", "error", err.Error())
		// 	return
		// }
		return
	}

	// buffer for handler to write reponse
	buf := new(bytes.Buffer)

	handlerErr := s.handler(buf, req)
	if handlerErr != nil {
		buf.Reset() // <-- reset handler error residual bytes from buf
		response.WriteStatusLine(conn, handlerErr.StatusCode)
		response.WriteHeaders(conn, len(handlerErr.Message), nil, nil, nil, nil, nil, nil)
		body := []byte(handlerErr.Message)
		_, err = conn.Write(body)
		if err != nil {
			slog.Warn("func (s *Server) handle", response.ErrWritingBody, err.Error())
			return
		}
		return
	}

	body := buf.Bytes()
	err = response.WriteStatusLine(conn, response.StatusOK)
	if err != nil {
		slog.Warn("error at writing status line", "err", err.Error())
		return
	}
	err = response.WriteHeaders(conn, len(body), nil, nil, nil, nil, nil, nil)
	if err != nil {
		slog.Warn("error at writing headers", response.ErrWritingHeaders, err.Error())
		return
	}
	_, err = conn.Write(body)
	if err != nil {
		slog.Warn("error at writing body", response.ErrWritingBody, err.Error())
		return
	}

	slog.Info("wrote", "bytes", len(body), "toaddr", conn.RemoteAddr().String())
}
