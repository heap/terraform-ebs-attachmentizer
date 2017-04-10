package main

import (
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	ec2 "github.com/aws/aws-sdk-go/service/ec2"
	// "github.com/davecgh/go-spew/spew"
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

type EC2Interface interface {
	// Get the instances matching the pattern, keyed by their ID.
	GetInstances(instanceNamePattern string) (map[string]Instance, error)
}

type EC2 struct {
	svc *ec2.EC2
}

func (c *EC2) GetInstances(instanceNamePattern string) (map[string]Instance, error) {
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			nameFilter(instanceNamePattern),
		},
	}
	resp, err := c.svc.DescribeInstances(params)
	if err != nil {
		return nil, err
	}

	instMap := make(map[string]Instance)
	for _, resv := range resp.Reservations {
		for _, instance := range resv.Instances {
			id := *instance.InstanceId
			devMap := make(map[DeviceName]BlockDevice)
			for _, blkDev := range instance.BlockDeviceMappings {
				devMap[NewDeviceName(*blkDev.DeviceName)] = BlockDevice{
					volumeID:            *blkDev.Ebs.VolumeId,
					deviceName:          NewDeviceName(*blkDev.DeviceName),
					deleteOnTermination: strconv.FormatBool(*blkDev.Ebs.DeleteOnTermination),

					instanceID:       id,
					availabilityZone: *instance.Placement.AvailabilityZone,
				}
			}
			instMap[id] = Instance{ID: id, BlockDevices: devMap}
		}
	}
	return instMap, nil
}

// Connect to EC2 and create the `InstanceDeviceMap` for instances matching the
// pattern.
func GetEC2AWSState(instanceNamePattern string, availabilityZone string) (map[string]Instance, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	ec2 := EC2{svc: ec2.New(sess, &aws.Config{Region: aws.String(availabilityZone)})}

	return ec2.GetInstances(instanceNamePattern)
}
