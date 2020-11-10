package main

import (
	"fmt"
	"log"
)

func handleError(no int, err error) {
	if err != nil {
		log.Fatalf("error %d: %s", no, err)
	}
}

func main() {
	// The purpose of this is to do poc testing of small pieces. DO NOT COMMIT your work here
}
