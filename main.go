package main

import (
	"fmt"
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
	var arr []byte = make([]byte, 8)

	_, err = file.Read(arr)
	if err != nil {
		log.Fatalf("error at reading file, err: %v", err.Error())
	}

	fmt.Printf("%s\n", arr)

}
