package main

import (
	"os"

	"github.com/heap/blkdev2volatt/ec2"
	"github.com/heap/blkdev2volatt/terraform"
	// "github.com/davecgh/go-spew/spew"
)

func main() {
	instDevMap := ec2.EC2Stuff(os.Args[1])
	terraform.TFStateStuff(os.Args[2], instDevMap)
}
