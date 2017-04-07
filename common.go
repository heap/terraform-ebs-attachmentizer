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
	deviceName          string
	encrypted           string
	iops                int
	snapshotId          string

	// Relevant instance information
	instanceID       string
	availabilityZone string
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
	buf.WriteString(fmt.Sprintf("%s-", dev.deviceName))
	buf.WriteString(fmt.Sprintf("%s-", dev.instanceID))
	buf.WriteString(fmt.Sprintf("%s-", dev.volumeID))

	return fmt.Sprintf("vai-%d", tfhash.String(buf.String()))
}

// Make a Terraform `aws_ebs_volume` resource from the attributes from an
// `ebs_block_device` block.
// TODO: Some attributes need to be translated; see comment at top of attrs.go.
func (dev *BlockDevice) makeVolumeRes() *tf.ResourceState {
	var attrs = make(map[string]string)
	attrs["size"] = strconv.Itoa(dev.size)
	attrs["type"] = dev.volumeType
	attrs["id"] = dev.volumeID
	attrs["encrypted"] = dev.encrypted
	attrs["availability_zone"] = dev.availabilityZone
	attrs["snapshot_id"] = dev.snapshotId

	// TODO verify attrs
	newRes := &tf.ResourceState{
		Type: "aws_ebs_volume",
		Primary: &tf.InstanceState{
			ID:         dev.volumeID,
			Attributes: attrs,
		},
	}
	return newRes
}

// Make a Terraform `aws_volume_attachment` resource from the attributes from an
// `ebs_block_device` block, which incldues the relevant instance information.
func (dev *BlockDevice) makeAttachmentRes() *tf.ResourceState {
	attrs := make(map[string]string)
	attachmentName := dev.volumeAttachmentID()

	// TODO verify attrs
	attrs["device_name"] = dev.deviceName
	attrs["instance_id"] = dev.instanceID
	attrs["volume_id"] = dev.volumeID
	attrs["id"] = attachmentName

	newRes := &tf.ResourceState{
		Type: "aws_volume_attachment",
		Primary: &tf.InstanceState{
			// TODO: Generate this correctly.
			ID:         attachmentName,
			Attributes: attrs,
		},
	}
	return newRes
}
