package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	var ngonx string = "-- NGonx UDP--"
	fmt.Println(ngonx)

	// connect to server (receiver)
	udpAddr, err := net.ResolveUDPAddr("udp", "localhost:4002")
	if err != nil {
		log.Fatalf("error at resolving udp addr, err: %v\n", err.Error())
	}

	fmt.Printf("udp resolved addr: %s\n", udpAddr.String())
	// net.ListenUDP(udpAddr.Network(), nil) can be use instead
	udpConn, err := net.DialUDP(udpAddr.Network(), nil, udpAddr)
	if err != nil {
		log.Fatalf("error at dialing to udp receiver, err: %v", err.Error())
	}
	defer udpConn.Close()

	sndBuffer := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		line, err := sndBuffer.ReadString(byte('\n'))

		if err != nil {
			if err == io.EOF {
				log.Println("end of file")
			}
			log.Printf("error at reading stream, err: %v\n", err.Error())
		}
		// udpConn.WriteToUDP([]byte(line), udpAddr) can be use instead
		_, err = udpConn.Write([]byte(line))
		if err != nil {
			log.Printf("error at writing line: '%s' to udp conn, err: %v", line, err.Error())
			continue
		}

		udpConn.SetDeadline(time.Now().Add(5 * time.Second))
		rcvBuffer := make([]byte, 1024)
		n, rcvAddr, err := udpConn.ReadFromUDP(rcvBuffer)
		if err != nil {
			log.Printf("receiver error: %v", err.Error())
			break
		}

		log.Printf("response from %s: %s\n", rcvAddr, string(rcvBuffer[:n]))

	}

}
