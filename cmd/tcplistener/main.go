package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", "localhost:4002")
	if err != nil {
		log.Fatalf("error at listening, err: %v", err.Error())
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("error at accepting conn, err: %v", err.Error())
		}

		chLines := getLinesChannel(conn)
		for line := range chLines {
			fmt.Printf("%s\n", string(line))
		}

	}

}

func getLinesChannel(conn net.Conn) <-chan []byte {
	chLine := make(chan []byte, 1)

	go func() {
		var curr_line []byte = nil
		for {
			buf := make([]byte, 8)
			n, err := conn.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Fatalf("error at reading conn, err: %v", err.Error())
			}

			buf = buf[:n]

			parts := bytes.Split(buf, []byte("\r\n"))
			last_part := parts[len(parts)-1]
			for p := range parts[:len(parts)-1] {
				chLine <- bytes.Join([][]byte{curr_line, parts[p]}, []byte{})

				curr_line = nil
			}
			curr_line = bytes.Join([][]byte{curr_line, last_part}, []byte{})
		}

		if len(curr_line) != 0 {
			chLine <- curr_line
		}
		defer conn.Close()
		defer close(chLine)
	}()

	return chLine
}
