package main

import (
	"io"
	"log"
	"log/slog"
	"ngonx/internal/request"
	"ngonx/internal/response"
	"ngonx/internal/server"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const port = 4002

func main() {
	handler := fakeRouterHandler
	server, err := server.Serve(port, handler)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
	defer server.Close()
	log.Printf("Server started on http://localhost:%d\n", port)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Server gracefully stopped")
}

func fakeRouterHandler(w io.Writer, req *request.Request) *server.HandlerError {
	slog.Info("req line ->", "method", req.RequestLine.Method, "uri", req.RequestLine.RequestURI)
	switch req.RequestLine.RequestURI {
	case "/landing.html":
		handlerError := fileHandler(w, req)
		return handlerError

	case "/":
		handlerError := defaultHandler(w, req)
		return handlerError
	}

	return &server.HandlerError{
		StatusCode: response.StatusNotFound,
		Message:    "Not found\n",
	}
}

func defaultHandler(w io.Writer, req *request.Request) *server.HandlerError {
	_, err := w.Write([]byte("<strong>Greetings! :)</strong><br/>" + "Welcome to the '" + req.RequestLine.RequestURI + "'<br/>"))
	if err != nil {
		return &server.HandlerError{StatusCode: response.StatusInternalServerError,
			Message: "Failed to read file\n",
		}
	}
	return nil
}

func fileHandler(w io.Writer, req *request.Request) *server.HandlerError {
	baseDir := "static"
	safePath := filepath.Base(req.RequestLine.RequestURI)
	filePath := filepath.Join(baseDir, safePath)

	file, err := os.Open(filePath)
	if err != nil {
		return &server.HandlerError{StatusCode: response.StatusNotFound,
			Message: "File not found\n",
		}
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	if err != nil {
		return &server.HandlerError{StatusCode: response.StatusInternalServerError,
			Message: "Failed to read file\n",
		}
	}

	return nil
}
