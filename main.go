package main

import (
	flags "github.com/jessevdk/go-flags"
	"log"
	"os"
)

type Options struct {
	Region          string         `short:"r" long:"region" description:"AWS region" required:"true"`
	ToFile          bool           `short:"w" long:"write" description:"Whether to write state to file"`
	InstancePattern string         `short:"p" long:"pattern" description:"EC2 instance name pattern" required:"true"`
	OutPath         flags.Filename `short:"o" long:"outpath" default:"/tmp/out.tfstate" description:"File out path"`
	StatePath       flags.Filename `short:"s" long:"statepath" description:"Current .tfstate location" required:"true"`
}

func main() {
	opts := new(Options)
	parser := flags.NewParser(opts, flags.Default)

	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	instDevMap, err := GetEC2AWSState(opts.InstancePattern, opts.AvailabilityZone)
	if err != nil {
		log.Fatalf("ec2 failed: %v", err)
	}

	ConvertTFState(string(opts.StatePath), string(opts.OutPath), opts.ToFile, instDevMap)
}
