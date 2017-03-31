// Package ec2 handles collecting volume and instance information from EC2.
package ec2

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"

	"github.com/heap/blkdev2volatt/common"
)

type volAtt struct {
	name, instance, volume, device string
}

// Build a filter on the instance `Name` tags. A `*` wildcard is allowed.
func nameFilter(instanceNamePattern string) *awsec2.Filter {
	return &awsec2.Filter{
		Name: aws.String("tag:Name"),
		Values: []*string{
			aws.String(instanceNamePattern),
		},
	}
}

// Map from instance ID to a mapping from device name to volume ID.
type InstanceDeviceMap map[string]map[common.DeviceName]string

type EC2Interface interface {
	GetInstances(instanceNamePattern string) (map[string]common.Instance, error)
}

type EC2 struct {
	svc *awsec2.EC2
}

func (ec2 *EC2) GetInstances(instanceNamePattern string) (map[string]common.Instance, error) {
	params := &awsec2.DescribeInstancesInput{
		Filters: []*awsec2.Filter{
			nameFilter(instanceNamePattern),
		},
	}
	resp, err:= ec2.svc.DescribeInstances(params)
	if err != nil {
		return nil, err
	}

	instMap := make(map[string]common.Instance)
	for _, resv := range resp.Reservations {
		for _, instance := range resv.Instances {
			id := *instance.InstanceId
			devMap := make(map[common.DeviceName]common.BlockDevice)
			for _, blkDev := range instance.BlockDeviceMappings {
				devMap[common.NewDeviceName(*blkDev.DeviceName)] = common.BlockDevice{
					ID: blkDev.Ebs.VolumeId,
				}
			}
			instMap[id] = common.Instance{ID: id, BlockDevices: devMap}
		}
	}
	return instMap, nil
}

// Connect to EC2 and create the `InstanceDeviceMap` for instances matching the
// pattern.
func EC2Stuff(instanceNamePattern string) (map[string]common.Instance, error)  {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	ec2 := EC2{svc: awsec2.New(sess, &aws.Config{Region: aws.String("us-east-1")})}

	return ec2.GetInstances(instanceNamePattern)
}
