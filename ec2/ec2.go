// Package ec2 handles collecting volume and instance information from EC2.
package ec2

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/heap/blkdev2volatt/common"
)

type volAtt struct {
	name, instance, volume, device string
}

// Build a filter on the instance `Name` tags. A `*` wildcard is allowed.
func nameFilter(instanceNamePattern string) *ec2.Filter {
	return &ec2.Filter{
		Name: aws.String("tag:Name"),
		Values: []*string{
			aws.String(instanceNamePattern),
		},
	}
}

// Map from instance ID to a mapping from device name to volume ID.
type InstanceDeviceMap map[string]map[common.DeviceName]string

// Connect to EC2 and create the `InstanceDeviceMap` for instances matching the
// pattern.
func EC2Stuff(instanceNamePattern string) InstanceDeviceMap {
	sess, err := session.NewSession()
	if err != nil {
		panic(err.Error())
	}
	svc := ec2.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			nameFilter(instanceNamePattern),
		},
	}
	resp, err:= svc.DescribeInstances(params)
	if err != nil {
		panic(err.Error())
	}

	instDevMap := make(InstanceDeviceMap)
	for _, resv := range resp.Reservations {
		for _, instance := range resv.Instances {
			devMap := make(map[common.DeviceName]string)
			instDevMap[*instance.InstanceId] = devMap
			for _, blkDev := range instance.BlockDeviceMappings {
				devMap[common.NewDeviceName(*blkDev.DeviceName)] = *blkDev.Ebs.VolumeId
			}
		}
	}
	return instDevMap
}
