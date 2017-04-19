package main

import (
	"bytes"
	"fmt"
	"strconv"
)

// Generates a .tf resource configuration string from a type, name, and mapping of attributes.
func generateResourceConfig(resourceType string, resourceName string, attrMap map[string]string) string {
	var configBuf bytes.Buffer
	configBuf.WriteString(fmt.Sprintf("resource \"%v\" \"%v\" {", resourceType, resourceName))

	for attribute, value := range attrMap {
		// The attribute map will contain the id, but it doesn't belong in the config.
		if attribute != "id" {
			configBuf.WriteString(fmt.Sprintf("\n\t%s = \"%s\"", attribute, value))
		}
	}

	configBuf.WriteString("\n}")
	return configBuf.String()
}

// Generates a .tf count variable configuration string
func genCountConfig(countVarName string, count int) string {
	var configBuf bytes.Buffer
	configBuf.WriteString(fmt.Sprintf("variable \"%s\" {", countVarName))
	configBuf.WriteString(fmt.Sprintf("\n\tdefault = %v\n}\n", count))
	return configBuf.String()
}

// Generates a string referencing a count variable, e.g., "${var.countVarName}"
func genCountVarReference(countVarName string) string {
	return fmt.Sprintf("${var.%s}", countVarName)
}

// Generates a string referencing the aws_instance resource for a given BlockDevice
// e.g., "${aws_instance.instanceName.id}", or "%{element(aws_instance.instanceName.*.id, count.index)}"
func genInstanceReference(dev BlockDevice, count int) string {
	instanceName := dev.instanceResName.name
	if count == 1 {
		return fmt.Sprintf("${aws_instance.%s.id}", instanceName)
	} else {
		return fmt.Sprintf("${element(aws_instance.%s.*.id, count.index)}", instanceName)
	}
}

// Similar to `genInstanceReference` for the the relevant ebs volume resource
func genVolumeReference(dev BlockDevice, count int) string {
	volumeName := fmt.Sprintf("%s-%s", dev.instanceResName.name, dev.deviceName.ShortName())
	if count == 1 {
		return fmt.Sprintf("${aws_ebs_volume.%s.id}", volumeName)
	} else {
		return fmt.Sprintf("${element(aws_ebs_volume.%s.*.id, count.index)}", volumeName)
	}
}

// Make a map of relevant volume attributes from an `ebs_block_device` block.
// used in generating the config for a volume
func makeVolumeAttrs(dev BlockDevice, countVarName string, count int) map[string]string {
	var attrs = make(map[string]string)

	attrs["size"] = strconv.Itoa(dev.size)
	attrs["type"] = dev.volumeType
	attrs["id"] = dev.volumeID
	attrs["encrypted"] = dev.encrypted
	attrs["availability_zone"] = dev.availabilityZone
	attrs["snapshot_id"] = dev.snapshotId

	if count > 1 {
		attrs["count"] = genCountVarReference(countVarName)
	}

	return attrs
}

// Make a map of relevant attachment attributes from an `ebs_block_device` block.
// used in generating the config for an attachment
func makeAttachmentAttrs(dev BlockDevice, countVarName string, count int) map[string]string {
	attrs := make(map[string]string)

	attrs["device_name"] = dev.deviceName.LongName()
	attrs["instance_id"] = genInstanceReference(dev, count)
	attrs["volume_id"] = genVolumeReference(dev, count)
	attrs["id"] = dev.volumeAttachmentID()

	if count > 1 {
		attrs["count"] = genCountVarReference(countVarName)
	}

	return attrs
}

// Takes a resource name and a list of Block Devices sharing the name and returns
// the appropriate config. There's 2 cases here:
// 1. len(devList) = 1: In this case, we just print a simple volume and attachment
//    config.
// 2. len(devList) > 1: In this case, we need to print a count variable and the
//    relevant count lookups for each resource.
func getConfigForDevGroup(groupName string, devList []BlockDevice) string {
	numDevs := len(devList)
	dev := devList[0] // All of these should be identical except for the count.
	countVarName := fmt.Sprintf("num_%s", dev.instanceResName.name)

	volumeAttrs := makeVolumeAttrs(dev, countVarName, numDevs)
	attachmentAttrs := makeAttachmentAttrs(dev, countVarName, numDevs)

	volumeConfig := generateResourceConfig("aws_ebs_volume", dev.NameWithoutCount(), volumeAttrs)
	attachmentConfig := generateResourceConfig("aws_volume_attachment", dev.NameWithoutCount(), attachmentAttrs)

	countConfig := ""
	if numDevs > 1 {
		countConfig = genCountConfig(countVarName, numDevs)
	}

	return fmt.Sprintf("%s%s\n%s", countConfig, volumeConfig, attachmentConfig)
}

// This is a bit janky, but here goes. We'd like to make a mapping from resource
// name to the block devices that share that name. The reason for this is to
// group resources generated through a `count` variable.
func getDevMapping(devs []BlockDevice) map[string][]BlockDevice {
	devMap := make(map[string][]BlockDevice)

	for _, dev := range devs {
		nameWithoutCount := dev.NameWithoutCount()
		devMap[nameWithoutCount] = append(devMap[nameWithoutCount], dev)
	}

	return devMap
}

// Take a list of block devices and generate a config.
func genConfig(devs []BlockDevice) string {
	var configBuf bytes.Buffer
	devNameMapping := getDevMapping(devs)

	for devName, devList := range devNameMapping {
		configChunk := getConfigForDevGroup(devName, devList)
		configBuf.WriteString(fmt.Sprintf("%s\n", configChunk))
	}

	return configBuf.String()
}
