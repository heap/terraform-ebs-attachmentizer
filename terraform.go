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
func validateEC2andTFDevs(devFromTF BlockDevice, devFromEC2 BlockDevice) bool {
	// There are 2 fields we obtain from both the Terraform state file and
	// the EC2 lookup:
	// 1. device name
	// 2. delete on termination
	names_match := NewDeviceName(devFromTF.deviceName).ShortName() == NewDeviceName(devFromEC2.deviceName).ShortName()
	deletes_match := devFromTF.deleteOnTermination == devFromEC2.deleteOnTermination
	validated := names_match && deletes_match
	return validated
}

// Sanity check of various fields on a block device.
func validateBlockDev(dev BlockDevice) bool {
	nameValid := dev.deviceName != ""
	IDValid := dev.volumeID != ""
	AZValid := dev.availabilityZone != ""
	typeValid := dev.volumeType != ""
	sizeValid := dev.size != 0
	instanceValid := dev.instanceID != ""
	return nameValid && IDValid && AZValid && typeValid && sizeValid && instanceValid
}

// Do the following:
// 1. Check that the relevant fields match between EC2 and TF
// 2. Merge the block devices into one
// 3. Validate relevant fields on the resulting block device
func mergeAndValidateBlockDevs(devFromTF BlockDevice, devFromEC2 BlockDevice) (BlockDevice, error) {
	fields_match := validateEC2andTFDevs(devFromTF, devFromEC2)
	if !fields_match {
		return BlockDevice{}, fmt.Errorf("EC2 and TF State discrepancy:\nFrom EC2:\n%+v\nFrom TF:\n%+v", devFromEC2, devFromTF)
	}

	dev := devFromTF
	dev.volumeID = devFromEC2.volumeID
	dev.instanceID = devFromEC2.instanceID
	dev.availabilityZone = devFromEC2.availabilityZone

	dev_ok := validateBlockDev(dev)
	if !dev_ok {
		return BlockDevice{}, fmt.Errorf("Invalid block device field detected:\n%+v", dev)
	}

	return dev, nil
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
			log.Fatalf("Could not expand ebs_block_device for %v", name)
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
			// Get the corresponding block device information from EC2.
			devFromEC2Info, ok := inst.BlockDevices[devName]
			if !ok {
				log.Fatalf("Could not find corresponding block device in EC2 for %v", devName)
			}

			// Merge in the relevant fields, and check that everything looks reasonable.
			dev, err := mergeAndValidateBlockDevs(devFromTFState, devFromEC2Info)
			if err != nil {
				log.Fatal(err)
			}

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
