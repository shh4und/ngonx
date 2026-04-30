package main

import (
	"fmt"
	"io"
	"log"
	"net"
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
	udpConn, err := net.ListenUDP(udpAddr.Network(), udpAddr)
	if err != nil {
		log.Fatalf("error at listen to udp conn, err: %v", err.Error())
	}
	defer udpConn.Close()

	buf := make([]byte, 1024)

	for {
		n, clientAddr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			if err == io.EOF {
				log.Println("end of file")
			}
			log.Printf("error at reading stream, err: %v\n", err.Error())
			continue
		}

		fmt.Printf("Got packet from %s: %s of size: %d\n", clientAddr.String(), string(buf), n)
		// udpConn.WriteToUDP([]byte(line), udpAddr) can be use instead
		_, err = udpConn.WriteToUDP([]byte("ok!"), clientAddr)
		if err != nil {
			log.Printf("error at writing response to client: '%s' to udp conn, err: %v", clientAddr.String(), err.Error())
		}

	}

}
