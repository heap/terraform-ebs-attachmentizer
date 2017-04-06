package main

import (
	"bytes"
	"fmt"
	"encoding/json"
	"log"
	"os"
	"strconv"

	"github.com/hashicorp/terraform/flatmap"
	tf "github.com/hashicorp/terraform/terraform"
	tfhash "github.com/hashicorp/terraform/helper/hashcode"
	tfstate "github.com/hashicorp/terraform/state"
	"github.com/davecgh/go-spew/spew"
)

// Get the ID Terraform synthesises for a volume attachment.
//
// From
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_volume_attachment.go#L244-L251
// with hat-tip to:
//  - https://github.com/hashicorp/terraform/issues/8458#issuecomment-258831650
//  - https://github.com/foxsy/tfvolattid
func volumeAttachmentID(dev BlockDevice) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s-", dev.instanceName))
	buf.WriteString(fmt.Sprintf("%s-", *dev.instanceID))
	buf.WriteString(fmt.Sprintf("%s-", *dev.volumeID))

	return fmt.Sprintf("vai-%d", tfhash.String(buf.String()))
}

// Type conversion for some []interface{} we know is actually a
// []map[string]interface{}, and convert all the values to strings.
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

func createDeviceMap(slice []map[string]string) (map[DeviceName]BlockDevice, error) {
	output := make(map[DeviceName]BlockDevice)
	for _, dev:= range slice {
		size, err := strconv.Atoi(dev["volume_size"])
		if err != nil {
			return nil, err
		}
    deviceName := NewDeviceName(dev["device_name"])
		output[deviceName] = BlockDevice{
      deviceName: deviceName.LongName(),
		  volumeType: dev["device_type"],
			size: size,
		}
	}
	return output, nil
}

// Do The Conversion on the Terraform state file given the extra resource ID
// information from EC2.
func ConvertTFState(stateFilePath string, instMap map[string]Instance) {
	localState := tfstate.LocalState{Path: stateFilePath, PathOut: "/tmp/out.tfstate"}
	localState.RefreshState()
	root := localState.State().Modules[0]

	newResources := make(map[string]*tf.ResourceState)

	for name, res := range root.Resources {
		if res.Type != "aws_instance" {
			// Do nothing if the resource isn't an instance.
			continue
		}
		inst, ok := instMap[res.Primary.ID]
		if !ok {
			// Do nothing if the instance wasn't one of the ones that the EC2
			// query returned.
			continue
		}

		interfaceDevices, ok := flatmap.Expand(
			res.Primary.Attributes,
			"ebs_block_device").([]interface{})

    spew.Dump(interfaceDevices)

		if !ok {
			log.Fatalf("could not expand ebs_block_device for %v", name)
		}

		// Delete the `ebs_block_device`s from the instance's state.
		attrs := flatmap.Map(res.Primary.Attributes)
		attrs.Delete("ebs_block_device")

		devices, ok := mapify(interfaceDevices)
		if !ok {
			log.Fatalf("Could not mapify")
		}

		devMap, err := createDeviceMap(devices)
		if err != nil {
			log.Fatalf("Could not create device map: %v", err)
		}

		for devName, dev := range devMap {
			// Merge in the volume ID.
			dev.volumeID = inst.BlockDevices[devName].volumeID
			volumeRes := dev.makeVolumeRes()
			attachmentRes := dev.makeAttachmentRes()
			newResources[fmt.Sprintf("aws_ebs_volume.%s-%s", name, devName.ShortName())] = volumeRes
			newResources[fmt.Sprintf("aws_volume_attachment.%s-%s", name, devName.ShortName())] = attachmentRes
		}
	}

	for k, v := range newResources {
		root.Resources[k] = v
	}

	json, _ := json.MarshalIndent(root.Resources, "", "  ")
	os.Stdout.Write(json)
}
