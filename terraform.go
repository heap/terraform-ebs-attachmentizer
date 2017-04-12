package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

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

type TerraformName struct {
	resourceType, name string
	// The index if any; -1 for none.
	index int
}

// Parse a Terraform name like `aws_instance.my_name.3` into its consituent parts.
func ParseTerraformName(name string) (*TerraformName, error) {
	var out TerraformName
	parts := strings.Split(name, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return nil, fmt.Errorf("Invalid resource name: %v", name)
	}
	out.resourceType = parts[0]
	out.name = parts[1]
	out.index = -1

	if len(parts) == 3 {
		index, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("Invalid resource name %v: %v", name, err)
		}
		out.index = index
	}

	return &out, nil
}

func createDeviceMap(instanceRes *TerraformName, slice []map[string]string) (map[DeviceName]BlockDevice, error) {
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
			deviceName:          deviceName,
			encrypted:           dev["encrypted"],
			iops:                iops,
			snapshotId:          dev["snapshot_id"],
			instanceResName:     instanceRes,
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
	names_match := devFromTF.deviceName.ShortName() == devFromEC2.deviceName.ShortName()
	deletes_match := devFromTF.deleteOnTermination == devFromEC2.deleteOnTermination
	validated := names_match && deletes_match
	return validated
}

// Sanity check of various fields on a block device.
func validateBlockDev(dev BlockDevice) bool {
	// Short name because otherwise we get "/dev/".
	nameValid := dev.deviceName.ShortName() != ""
	IDValid := dev.volumeID != ""
	AZValid := dev.availabilityZone != ""
	sizeValid := dev.size != 0
	instanceValid := dev.instanceID != ""
	return nameValid && IDValid && AZValid && sizeValid && instanceValid
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
// information from EC2. Returns the new terraform state, and a suggested configuration
// string for use in the `.tf` source file.
func generateNewTFState(stateToModify *tf.State, instMap map[string]Instance) (*tf.State, string) {
	outState := stateToModify.DeepCopy()
	root := outState.Modules[0]
	var configBuf bytes.Buffer

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

		instanceResName, err := ParseTerraformName(name)
		if err != nil {
			log.Fatal(err)
		}
		devMap, err := createDeviceMap(instanceResName, devices)
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

			newResources[dev.VolumeName()] = volumeRes
			newResources[dev.VolumeAttachmentName()] = attachmentRes

			configBuf.WriteString(fmt.Sprintf("\n%s\n", dev.makeVolumeConfig()))
			configBuf.WriteString(fmt.Sprintf("\n%s\n", dev.makeAttachmentConfig()))
		}
	}

	for k, v := range newResources {
		root.Resources[k] = v
	}

	return outState, configBuf.String()
}

// :TODO: Add the option to print a diff.
func outputState(localState *tfstate.LocalState, newState *tf.State, toFile bool) {
	json, _ := json.MarshalIndent(newState.Modules[0].Resources, "", "  ")
	fmt.Print("\n________New Terraform state (JSON)________\n\n")
	os.Stdout.Write(json)

	if toFile {
		// WriteState updates the state `serial`, so we don't have to worry about it.
		localState.WriteState(newState)
	}
}

// :TODO: Provide option to write this to a user-specified file, rather than just printing.
func outputConfig(config string) {
	fmt.Print("\n________Suggested .tf configuration________\n")
	fmt.Print(config)
}

// Do The Conversion on the Terraform state file given the extra resource ID
// information from EC2.
func ConvertTFState(stateFilePath string, outFilePath string, toFile bool, instMap map[string]Instance) {
	localState := tfstate.LocalState{Path: stateFilePath, PathOut: outFilePath}
	localState.RefreshState()
	stateToModify := localState.State()

	newState, newConfig := generateNewTFState(stateToModify, instMap)
	fmt.Print("========Successfully generated new state========\n")
	outputConfig(newConfig)
	outputState(&localState, newState, toFile)
}
