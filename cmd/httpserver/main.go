package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"ngonx/internal/request"
	"ngonx/internal/response"
	"ngonx/internal/server"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
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

	slog.Info("Server started on", "addr", fmt.Sprintf("http://localhost:%d", port))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	slog.Info("Server gracefully stopped")
}

func fakeRouterHandler(w io.Writer, req *request.Request) *server.HandlerError {
	slog.Info("req line ->", "method", req.RequestLine.Method, "uri", req.RequestLine.RequestURI)
	switch req.RequestLine.RequestURI {
	case "/public/landing.html":
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
	baseDir := "./static"
	routePrefix := "/public"

	cleanedURI := path.Clean(req.RequestLine.RequestURI)

	relPath := strings.TrimPrefix(cleanedURI, routePrefix)
	relPath = strings.TrimPrefix(relPath, "/")

	systemRelPath := filepath.FromSlash(relPath)

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return &server.HandlerError{StatusCode: response.StatusNotFound,
			Message: "File not found\n",
		}
	}

	targetPath := filepath.Join(absBaseDir, systemRelPath)
	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return &server.HandlerError{StatusCode: response.StatusNotFound,
			Message: "File not found\n",
		}
	}

	prefix := absBaseDir + string(filepath.Separator)
	if absTargetPath != absBaseDir && !strings.HasPrefix(absTargetPath, prefix) {
		return &server.HandlerError{StatusCode: response.StatusForbidden,
			Message: "Forbidden\n",
		}

	}

	pathInfo, err := os.Stat(absTargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &server.HandlerError{StatusCode: response.StatusNotFound,
				Message: "File not found\n",
			}
		}

		return &server.HandlerError{StatusCode: response.StatusInternalServerError,
			Message: "Internal Server Error\n",
		}

	}

	if pathInfo.IsDir() {
		return &server.HandlerError{StatusCode: response.StatusForbidden,
			Message: "Forbidden\n",
		}
	}

	file, err := os.Open(absTargetPath)
	if err != nil {
		return &server.HandlerError{StatusCode: response.StatusInternalServerError,
			Message: "Failed to read file\n",
		}
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	if err != nil {
		return &server.HandlerError{StatusCode: response.StatusInternalServerError,
			Message: "Failed to send file\n",
		}
	}

	return nil
}
