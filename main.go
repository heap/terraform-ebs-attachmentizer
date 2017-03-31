package blkdev2volatt

import (
	"log"
	"os"

	// "github.com/davecgh/go-spew/spew"
)

func main() {
	instDevMap, err := EC2Stuff(os.Args[1])
	if err != nil {
		log.Fatalf("ec2 failed: %v", err)
	}
	TFStateStuff(os.Args[2], instDevMap)
}
