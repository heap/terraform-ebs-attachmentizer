package terraform

import (
	"bytes"
	"fmt"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/terraform/flatmap"
	tf "github.com/hashicorp/terraform/terraform"
	tfhash "github.com/hashicorp/terraform/helper/hashcode"
	tfstate "github.com/hashicorp/terraform/state"
	// "github.com/davecgh/go-spew/spew"

	"blkdev2volatt/common"
	"blkdev2volatt/ec2"
)

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

func TFStateStuff(fn string, instDevMap ec2.InstanceDeviceMap) {
	localState := tfstate.LocalState{Path: fn, PathOut: "/tmp/out.tfstate"}
	localState.RefreshState()
	root := localState.State().Modules[0]

	newResources := make(map[string]*tf.ResourceState)

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

		attrs := flatmap.Map(res.Primary.Attributes)
		attrs.Delete("ebs_block_device")


		if !ok {
			log.Fatalf("could not expand ebs_block_device for %v", name)
		}

		devices, ok := mapify(interfaceDevices)
		if !ok {
			log.Fatalf("Could not mapify")
		}
		for _, dev := range devices {
			dev["device_name"] = common.NormalizeDeviceName(dev["device_name"])
		}
		for _, dev := range devices {
			devName := common.NormalizeDeviceName(dev["device_name"])
			volumeRes := makeVolumeRes(devMap[devName], dev)
			volumeID := volumeRes.Primary.ID
			attachmentRes := makeAttachmentRes(instanceName, instanceID, volumeID, dev)
			newResources[fmt.Sprintf("aws_ebs_volume.%s-%s", name, strings.TrimPrefix(devName, "/dev/"))] = volumeRes
			newResources[fmt.Sprintf("aws_volume_attachment.%s-%s", name, strings.TrimPrefix(devName, "/dev/"))] = attachmentRes
		}
	}

	for k, v := range newResources {
		root.Resources[k] = v
	}

	json, _ := json.MarshalIndent(root.Resources, "", "  ")
	os.Stdout.Write(json)
}
