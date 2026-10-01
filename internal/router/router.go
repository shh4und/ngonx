package router

import (
	"io"
	"log/slog"
	"maps"
	"ngonx/internal/request"
	"ngonx/internal/response"
	"ngonx/internal/server"
)

type Router struct {
	// Array plano indexado diretamente por enum numérico: O(1) puro, cache-friendly
	trees map[string]*RadixTree
}

const (
	methodGET    = "GET"
	methodPOST   = "POST"
	methodPUT    = "PUT"
	methodDELETE = "DELETE"
)

func NewRouter() *Router {
	return &Router{
		trees: make(map[string]*RadixTree),
	}
}

func (r *Router) Handle(method, path string, handler server.Handler) {
	tree := r.trees[method]
	if tree == nil {
		tree = NewRadixTree()
		r.trees[method] = tree
	}

	tree.Insert(path, handler)
}

func (r *Router) Router(w io.Writer, req *request.Request) *server.HandlerError {

	// Caminho Crítico (Hot Path): alta velocidade
	if root := r.trees[req.RequestLine.Method]; root != nil {
		if _, handler := root.Search(req.RequestLine.RequestURI); handler != nil {
			handlerErr := handler(w, req)
			return handlerErr
		}
	}

	// Caminho Frio (Cold Path): Rota não bateu no método solicitado.
	// Verifica se a rota existe nas árvores dos outros métodos para compor o cabeçalho 'Allow'.
	var allowed []string
	for method := range maps.Keys(request.MethodsSet) {
		root := r.trees[method]
		if root != nil && method != req.RequestLine.Method {
			if _, h := root.Search(req.RequestLine.RequestURI); h != nil {
				allowed = append(allowed, method)
			}
		}
	}

	slog.Info("methodNotFound", "alloweds", allowed, "method", req.RequestLine.Method, "uri", req.RequestLine.RequestURI)

	if len(allowed) > 0 {
		return &server.HandlerError{
			StatusCode: 405,
			Message:    "Method Not Allowed\n",
		}
	}

	return &server.HandlerError{
		StatusCode: response.StatusNotFound,
		Message:    "Not found\n",
	}
}

func (r *Router) GET(path string, handler server.Handler) {
	r.Handle(methodGET, path, handler)
}
func (r *Router) POST(path string, handler server.Handler) {
	r.Handle(methodPOST, path, handler)
}
func (r *Router) PUT(path string, handler server.Handler) {
	r.Handle(methodPUT, path, handler)
}
func (r *Router) DELETE(path string, handler server.Handler) {
	r.Handle(methodDELETE, path, handler)
}
