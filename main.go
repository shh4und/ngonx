package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func main() {

	var ngonx string = "-- NGonx --"

	fmt.Printf("%v\n", ngonx)
	path := "./messages.txt"
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("error at opening file %s, err: %v", path, err.Error())
	}
	defer file.Close()

	buf := make([]byte, 8)
	var curr_line string
	for {
		_, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("erro at reading file, err: %v", err.Error())
		}
		parts := strings.Split(string(buf), "\n")
		last_part := parts[len(parts)-1]
		for p := range parts[:len(parts)-1] {
			fmt.Printf("read: %s\n", curr_line+parts[p])
			curr_line = ""
		}
		curr_line += last_part
	}
	if len(curr_line) != 0 {
		fmt.Printf("read: %s", curr_line)
	}
}

func getLinesChannel(f io.ReadCloser) <-chan string
