package main

import (
	flags "github.com/jessevdk/go-flags"
	"log"
	"os"
)

type Options struct {
	Region          string         `short:"r" long:"region" description:"AWS region" required:"true"`
	InstancePattern string         `short:"p" long:"pattern" description:"EC2 instance name pattern" required:"true"`
	StatePath       flags.Filename `short:"s" long:"statepath" description:"Current .tfstate location" required:"true"`
	StateOutPath    flags.Filename `short:"o" long:"stateoutpath" default:"/tmp/out.tfstate" description:"State file out path"`
	ConfigOutPath   flags.Filename `short:"c" long:"configoutpath" default:"/tmp/config.tf" description:"Config out path"`
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

	instDevMap, err := GetEC2AWSState(opts.InstancePattern, opts.Region)
	if err != nil {
		log.Fatalf("ec2 failed: %v", err)
	}

	ConvertTFState(string(opts.StatePath), string(opts.StateOutPath), string(opts.ConfigOutPath), instDevMap)
}
