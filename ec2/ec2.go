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

func nameFilter(inst string) *ec2.Filter {
	return &ec2.Filter{
		Name: aws.String("tag:Name"),
		Values: []*string{
			aws.String(inst),
		},
	}
}

type InstanceDeviceMap map[string]map[common.DeviceName]string

func EC2Stuff(inst string) InstanceDeviceMap {
	sess, err := session.NewSession()
	if err != nil {
		panic(err.Error())
	}
	svc := ec2.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			nameFilter(inst),
		},
	}
	resp, err:= svc.DescribeInstances(params)
	if err != nil {
		panic(err.Error())
	}

	instDevMap := make(InstanceDeviceMap)
	for _, resv := range resp.Reservations {
		for _, inst := range resv.Instances {
			devMap := make(map[common.DeviceName]string)
			instDevMap[*inst.InstanceId] = devMap
			for _, blkDev := range inst.BlockDeviceMappings {
				devMap[common.NewDeviceName(*blkDev.DeviceName)] = *blkDev.Ebs.VolumeId
			}
		}
	}
	return instDevMap
}
