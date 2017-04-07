package main

import (
	"log"
	"os"
	// "github.com/davecgh/go-spew/spew"
)

func main() {
	instDevMap, err := GetEC2AWSState(os.Args[1])
	if err != nil {
		log.Fatalf("ec2 failed: %v", err)
	}
	ConvertTFState(os.Args[2], instDevMap)
}
