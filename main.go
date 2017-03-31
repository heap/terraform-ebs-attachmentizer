package main

import (
	"log"
	"os"

	"github.com/heap/blkdev2volatt/ec2"
	"github.com/heap/blkdev2volatt/terraform"
	// "github.com/davecgh/go-spew/spew"
)

func main() {
	instDevMap, err := ec2.EC2Stuff(os.Args[1])
	if err != nil {
		log.Fatalf("ec2 failed: %v", err)
	}
	terraform.TFStateStuff(os.Args[2], instDevMap)
}
