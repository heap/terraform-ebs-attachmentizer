package blkdev2volatt

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	ec2 "github.com/aws/aws-sdk-go/service/ec2"
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
type InstanceDeviceMap map[string]map[DeviceName]string

type EC2Interface interface {
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
	resp, err:= c.svc.DescribeInstances(params)
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
					ID: blkDev.Ebs.VolumeId,
				}
			}
			instMap[id] = Instance{ID: id, BlockDevices: devMap}
		}
	}
	return instMap, nil
}

// Connect to EC2 and create the `InstanceDeviceMap` for instances matching the
// pattern.
func EC2Stuff(instanceNamePattern string) (map[string]Instance, error)  {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	ec2 := EC2{svc: ec2.New(sess, &aws.Config{Region: aws.String("us-east-1")})}

	return ec2.GetInstances(instanceNamePattern)
}
