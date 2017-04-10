package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	tfhash "github.com/hashicorp/terraform/helper/hashcode"
	tf "github.com/hashicorp/terraform/terraform"
)

// TODO: make this more robust.
func normalizeDeviceName(dev string) string {
	if strings.HasPrefix(dev, "/dev/") {
		return dev
	} else {
		return fmt.Sprintf("/dev/%v", dev)
	}
}

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

// A wrapper around a device name that allows getting it with and without the
// `/dev/` prefix.
type DeviceName struct {
	longName string
}

func NewDeviceName(dev string) DeviceName {
	return DeviceName{normalizeDeviceName(dev)}
}

// Without `/dev/`.
func (dn DeviceName) ShortName() string {
	return strings.TrimPrefix(dn.longName, "/dev/")
}

// With `/dev/`.
func (dn DeviceName) LongName() string {
	return dn.longName
}

func (dn DeviceName) String() string {
	return dn.longName
}

type Instance struct {
	ID           string
	BlockDevices map[DeviceName]BlockDevice
}

// This struct includes all attributes present in the tfstate representation of
// a block device (in tf_attrs.go), as well as the ec2 instance attributes necessary
// to generate the relevant ebs attachment resource.
type BlockDevice struct {
	volumeID            string
	size                int
	volumeType          string
	deleteOnTermination string
	deviceName          DeviceName
	encrypted           string
	iops                int
	snapshotId          string

	// Relevant instance information
	instanceID       string
	availabilityZone string
	instanceResName  *TerraformName
}

func (dev *BlockDevice) UniqueName() string {
	indexPart := ""
	if dev.instanceResName.index != -1 {
		indexPart = fmt.Sprintf(".%v", dev.instanceResName.index)
	}

	return fmt.Sprintf("%s-%s%s",
		dev.instanceResName.name,
		dev.deviceName.ShortName(),
		indexPart)
}

func (dev *BlockDevice) VolumeName() string {
	resourceName := dev.UniqueName()
	return fmt.Sprintf("aws_ebs_volume.%s", resourceName)
}

func (dev *BlockDevice) VolumeAttachmentName() string {
	resourceName := dev.UniqueName()
	return fmt.Sprintf("aws_volume_attachment.%s", resourceName)
}

// Get the ID Terraform synthesises for a volume attachment.
//
// From
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_volume_attachment.go#L244-L251
// with hat-tip to:
//  - https://github.com/hashicorp/terraform/issues/8458#issuecomment-258831650
//  - https://github.com/foxsy/tfvolattid
func (dev *BlockDevice) volumeAttachmentID() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s-", dev.deviceName.LongName()))
	buf.WriteString(fmt.Sprintf("%s-", dev.instanceID))
	buf.WriteString(fmt.Sprintf("%s-", dev.volumeID))

	return fmt.Sprintf("vai-%d", tfhash.String(buf.String()))
}

// Make a map of relevant volume attributes from an `ebs_block_device` block.
// used in `dev.makeVolumeRes` and `dev.makeVolumeConfig`
func (dev *BlockDevice) makeVolumeAttrs() map[string]string {
	var attrs = make(map[string]string)

	attrs["size"] = strconv.Itoa(dev.size)
	attrs["type"] = dev.volumeType
	attrs["id"] = dev.volumeID
	attrs["encrypted"] = dev.encrypted
	attrs["availability_zone"] = dev.availabilityZone
	attrs["snapshot_id"] = dev.snapshotId

	return attrs
}

// Make a Terraform `aws_ebs_volume` resource from the attributes from an
// `ebs_block_device` block.
func (dev *BlockDevice) makeVolumeRes() *tf.ResourceState {
	attrs := dev.makeVolumeAttrs()

	newRes := &tf.ResourceState{
		Type: "aws_ebs_volume",
		Primary: &tf.InstanceState{
			ID:         dev.volumeID,
			Attributes: attrs,
		},
	}
	return newRes
}

// Make a map of relevant attachment attributes from an `ebs_block_device` block.
// used in `dev.makeAttachmentRes` and `dev.makeAttachmentConfig`
func (dev *BlockDevice) makeAttachmentAttrs() map[string]string {
	attrs := make(map[string]string)

	attrs["device_name"] = dev.deviceName.LongName()
	attrs["instance_id"] = dev.instanceID
	attrs["volume_id"] = dev.volumeID
	attrs["id"] = dev.volumeAttachmentID()

	return attrs
}

// Make a Terraform `aws_volume_attachment` resource from the attributes from an
// `ebs_block_device` block, which incldues the relevant instance information.
func (dev *BlockDevice) makeAttachmentRes() *tf.ResourceState {
	attrs := dev.makeAttachmentAttrs()

	newRes := &tf.ResourceState{
		Type: "aws_volume_attachment",
		Primary: &tf.InstanceState{
			ID:         dev.volumeAttachmentID(),
			Attributes: attrs,
		},
	}
	return newRes
}

// Generate a configuration for the `aws_ebs_volume` resource from a specific
// block device.
func (dev *BlockDevice) makeVolumeConfig() string {
	attrs := dev.makeVolumeAttrs()
	return generateResourceConfig("aws_ebs_volume", dev.UniqueName(), attrs)
}

// Generate a configuration for the `aws_volume_attachment` resource from a specific
// block device.
// :TODO: Make this print `instance_id` and `volume_id` via resource reference rather
//        than the raw ids.
func (dev *BlockDevice) makeAttachmentConfig() string {
	attrs := dev.makeAttachmentAttrs()
	return generateResourceConfig("aws_volume_attachment", dev.UniqueName(), attrs)
}
