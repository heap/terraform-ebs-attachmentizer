package main
import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	// "github.com/davecgh/go-spew/spew"

	"github.com/hashicorp/terraform/flatmap"
	tf "github.com/hashicorp/terraform/terraform"
	tfhash "github.com/hashicorp/terraform/helper/hashcode"
	tfstate "github.com/hashicorp/terraform/state"
)

func main() {
	instDevMap := ec2Stuff(os.Args[1])
	tfStateStuff(os.Args[2], instDevMap)
}

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

type instanceDeviceMap map[string]map[string]string

func ec2Stuff(inst string) instanceDeviceMap {
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

	instDevMap := make(instanceDeviceMap)
	for _, resv := range resp.Reservations {
		for _, inst := range resv.Instances {
			devMap := make(map[string]string)
			instDevMap[*inst.InstanceId] = devMap
			for _, blkDev := range inst.BlockDeviceMappings {
				devMap[normalizeDeviceName(*blkDev.DeviceName)] = *blkDev.Ebs.VolumeId
			}
		}
	}
	return instDevMap
}

// TODO: make this more robust.
func normalizeDeviceName(dev string) string {
	if strings.HasPrefix(dev, "/dev/") {
		return dev
	} else {
		return fmt.Sprintf("/dev/%v", dev)
	}
}

/////////////////
// NEW STUFF HERE
/////////////////

// aws_ebs_volume attributes from
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_ebs_volume.go#L27-L68
var awsEbsVolumeAttrs = map[string]struct{}{
	"availability_zone": struct{}{},
	"encrypted": struct{}{},
	"iops": struct{}{},
	"kms_key_id": struct{}{},
	"size": struct{}{},
	"snapshot_id": struct{}{},
	"type": struct{}{},
	"tags": struct{}{},
}

// aws_volume_attachment attrs from
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_volume_attachment.go#L23-L52
var awsVolumeAttachmentAttrs = map[string]struct{}{
	"device_name": struct{}{},
	"instance_id": struct{}{},
	"volume_id": struct{}{},
	"force_detach": struct{}{},
	"skip_destroy": struct{}{},
}

// aws_instance.ebs_block_device attributes from
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_instance.go#L214-L262
var awsInstanceEbsBlockDeviceAttrs = map[string]struct{}{
	"delete_on_termination": struct{}{},
	"device_name": struct{}{},
	"encrypted": struct{}{},
	"iops": struct{}{},
	"snapshot_id": struct{}{},
	"volume_size": struct{}{},
	"volume_type": struct{}{},
}

// From
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_volume_attachment.go#L244-L251
// with hat-tip to:
//  - https://github.com/hashicorp/terraform/issues/8458#issuecomment-258831650
//  - https://github.com/foxsy/tfvolattid
func volumeAttachmentID(name, volumeID, instanceID string) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s-", name))
	buf.WriteString(fmt.Sprintf("%s-", instanceID))
	buf.WriteString(fmt.Sprintf("%s-", volumeID))

	return fmt.Sprintf("vai-%d", tfhash.String(buf.String()))
}

func makeVolumeRes(volID string, dev map[string]string) *tf.ResourceState {
	var attrs = make(map[string]string)
	for k, v := range dev {
		if _, ok := awsEbsVolumeAttrs[k]; ok {
			attrs[k] = v
		}
	}
	newRes := &tf.ResourceState{
		Type: "aws_ebs_volume",
		Primary: &tf.InstanceState{
			ID: volID,
			Attributes: attrs,
		},
	}
	return newRes
}

func makeAttachmentRes(instanceName, instanceID, volumeID string,
dev map[string]string) *tf.ResourceState {
	attrs := make(map[string]string)
	for k, v := range dev {
		if _, ok := awsVolumeAttachmentAttrs[k]; ok {
			attrs[k] = fmt.Sprintf("%v", v)
		}
	}
	attrs["instance_id"] = instanceID
	attrs["volume_id"] = volumeID
	newRes := &tf.ResourceState{
		Type: "aws_volume_attachment",
		Primary: &tf.InstanceState{
			// TODO: Generate this correctly.
			ID: volumeAttachmentID(instanceName, volumeID, instanceID),
			Attributes: attrs,
		},
	}
	return newRes
}

// TODO: This must be easier with some kind of reflection thing.
func mapify(slice []interface{}) ([]map[string]string, bool) {
	var output []map[string]string
	for _, input := range slice {
		stringMap := make(map[string]string)
		interfaceMap, ok := input.(map[string]interface{})
		if !ok {
			return nil, false
		}
		for k, v := range interfaceMap {
			stringMap[k] = fmt.Sprintf("%v", v)
		}

		output = append(output, stringMap)
	}
	return output, true
}

func tfStateStuff(fn string, instDevMap instanceDeviceMap) {
	localState := tfstate.LocalState{Path: fn, PathOut: "/tmp/out.tfstate"}
	localState.RefreshState()
	root := localState.State().Modules[0]

	var newResources []*tf.ResourceState

	for name, res := range root.Resources {
		if res.Type != "aws_instance" {
			continue
		}
		devMap, ok := instDevMap[res.Primary.ID]
		if !ok {
			continue
		}
		instanceName := name
		instanceRes := res
		instanceID := instanceRes.Primary.ID

		interfaceDevices, ok := flatmap.Expand(
			res.Primary.Attributes,
			"ebs_block_device").([]interface{})


		if !ok {
			log.Fatalf("could not expand ebs_block_device for %v", name)
		}

		devices, ok := mapify(interfaceDevices)
		if !ok {
			log.Fatalf("Could not mapify")
		}
		for _, dev := range devices {
			dev["device_name"] = normalizeDeviceName(dev["device_name"])
		}
		for _, dev := range devices {
			devName := normalizeDeviceName(dev["device_name"])
			volumeRes := makeVolumeRes(devMap[devName], dev)
			volumeID := volumeRes.Primary.ID
			attachmentRes := makeAttachmentRes(instanceName, instanceID, volumeID, dev)
			newResources = append(newResources, volumeRes, attachmentRes)
		}
	}

	json, _ := json.MarshalIndent(newResources, "", "  ")
	os.Stdout.Write(json)
}
