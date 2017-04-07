package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	// "github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform/flatmap"
	tfstate "github.com/hashicorp/terraform/state"
	tf "github.com/hashicorp/terraform/terraform"
)

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
	for _, dev := range slice {
		size, err := strconv.Atoi(dev["volume_size"])
		if err != nil {
			return nil, err
		}
		iops, err := strconv.Atoi(dev["iops"])
		if err != nil {
			return nil, err
		}
		deviceName := NewDeviceName(dev["device_name"])
		output[deviceName] = BlockDevice{
			size:                size,
			volumeType:          dev["device_type"],
			deleteOnTermination: dev["delete_on_termination"],
			deviceName:          deviceName.LongName(),
			encrypted:           dev["encrypted"],
			iops:                iops,
			snapshotId:          dev["snapshot_id"],
		}
	}
	return output, nil
}

// Check if the fields we pulled from both EC2 and Terraform match. If there are
// conflicts, something smells fishy and we shouldn't continue.
func ValidateEC2andTFDevs(devFromTFState BlockDevice, devFromEC2 BlockDevice) {
	// TODO - Implement this.
}

func mergeEC2andTFStateDevs(devFromTFState BlockDevice, devFromEC2 BlockDevice) BlockDevice {
	dev := devFromTFState
	dev.volumeID = devFromEC2.volumeID
	dev.instanceID = devFromEC2.instanceID
	dev.availabilityZone = devFromEC2.availabilityZone
	return dev
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

		for devName, devFromTFState := range devMap {
			// Merge in the volume ID.
			devFromEC2Info := inst.BlockDevices[devName]
			ValidateEC2andTFDevs(devFromTFState, devFromEC2Info)
			dev := mergeEC2andTFStateDevs(devFromTFState, devFromEC2Info)

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
