package router

import (
	"fmt"
	"ngonx/internal/server"
)

type radixNode struct {
	path     string
	indices  string
	children []*radixNode
	handler  server.Handler
}

type RadixTree struct {
	root *radixNode
}

func NewRadixTree() *RadixTree {
	return &RadixTree{root: &radixNode{}}
}
func longestCommonPrefix(a, b string) int {
	max := min(len(a), len(b))

	for i := range max {
		if a[i] != b[i] {
			return i
		}
	}

	return max
}
func (r *RadixTree) Insert(path string, handler server.Handler) {
	curr := r.root

	// Árvore vazia
	if curr.path == "" && len(curr.children) == 0 {
		curr.path = path
		curr.handler = handler
		return
	}

walk:
	for {
		i := longestCommonPrefix(path, curr.path)

		// O prefixo diverge do nó atual: precisamos dividir (split) o nó
		if i < len(curr.path) {
			child := &radixNode{
				path:     curr.path[i:],
				indices:  curr.indices,
				children: curr.children,
				handler:  curr.handler,
			}

			curr.children = []*radixNode{child}
			curr.indices = string(curr.path[i])
			curr.path = path[:i]
			curr.handler = nil
		}

		// O caminho a ser inserido tem bytes restantes após o prefixo comum
		if i < len(path) {
			path = path[i:]
			c := path[0]

			// Verifica se já existe um filho iniciando com o caractere 'c'
			for j := 0; j < len(curr.indices); j++ {
				if curr.indices[j] == c {
					curr = curr.children[j]
					continue walk
				}
			}

			// Cria um novo filho
			child := &radixNode{path: path, handler: handler}
			curr.children = append(curr.children, child)
			curr.indices += string(c)
			return
		}

		// Caminho exato bateu no nó atual
		curr.handler = handler
		return
	}
}

func (r *RadixTree) Search(path string) (string, server.Handler) {
	curr := r.root

walk:
	for {
		if len(path) >= len(curr.path) && path[:len(curr.path)] == curr.path {
			pathToShow := path
			path = path[len(curr.path):]

			if len(path) == 0 {
				return fmt.Sprintf("path: %s", pathToShow), curr.handler
			}

			c := path[0]
			for i := 0; i < len(curr.indices); i++ {
				if curr.indices[i] == c {
					curr = curr.children[i]
					continue walk
				}
			}
		}
		return "<notFound>", nil
	}
}

// PrintTree imprime a estrutura da Radix Tree no formato do utilitário 'tree' do Unix.
func (r *RadixTree) PrintTree() {
	if r.root == nil || (r.root.path == "" && len(r.root.children) == 0) {
		fmt.Println("(árvore vazia)")
		return
	}

	// Imprime a raiz
	status := ""
	if r.root.handler != nil {
		status = "[HANDLER]"
	}
	fmt.Printf("%q%s\n", r.root.path, status)

	// Imprime os ramos recursivamente
	for i, child := range r.root.children {
		isLast := i == len(r.root.children)-1
		printRadixNode(child, "", isLast)
	}
}

func printRadixNode(n *radixNode, prefix string, isLast bool) {
	connector := "├── "
	childPrefix := prefix + "│   "

	if isLast {
		connector = "└── "
		childPrefix = prefix + "    "
	}

	status := ""
	if n.handler != nil {
		status = "[HANDLER]"
	}
	fmt.Printf("%s%s%q%s\n", prefix, connector, n.path, status)

	for i, child := range n.children {
		printRadixNode(child, childPrefix, i == len(n.children)-1)
	}
}
