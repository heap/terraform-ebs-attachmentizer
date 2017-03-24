package main

import (
	"os"

	"blkdev2volatt/ec2"
	"blkdev2volatt/terraform"
	// "github.com/davecgh/go-spew/spew"
)

func main() {
	instDevMap := ec2.EC2Stuff(os.Args[1])
	terraform.TFStateStuff(os.Args[2], instDevMap)
}
