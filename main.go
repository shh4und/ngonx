package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
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

	var curr_line string = ""
	for {
		buf := make([]byte, 8)
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("erro at reading file, err: %v", err.Error())
		}

		buf = buf[:n]

		parts := bytes.Split(buf, []byte{'\n'})

		last_part := string(parts[len(parts)-1])
		for p := range parts[:len(parts)-1] {
			fmt.Printf("read: %s\n", curr_line+string(parts[p]))
			curr_line = ""
		}
		curr_line += last_part
	}
	if len(curr_line) != 0 {
		fmt.Printf("read: %s", curr_line)
	}
}

// func getLinesChannel(f io.ReadCloser) <-chan string
