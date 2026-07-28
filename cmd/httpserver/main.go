package main

import (
	"io"
	"log"
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
	handler := fileHandler(".")
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

func fileHandler(baseDir string) server.Handler {
	return func(w io.Writer, req *request.Request) *server.HandlerError {

		filePath := filepath.Join(baseDir, req.RequestLine.RequestURI)

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
}
