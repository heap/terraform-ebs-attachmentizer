package main

import (
	"fmt"
	"strings"
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
	ID string
	BlockDevices map[DeviceName]BlockDevice
}

// This struct includes all attributes present in the tfstate representation of
// a block device (in tf_attrs.go), as well as the ec2 instance attributes necessary
// to generate the relevant ebs attachment resource.
type BlockDevice struct {
	volumeID *string // Pointer so it can be nil in the case where we don't know what it is.
	size int
	volumeType string
  deleteOnTermination string
  deviceName string
  encrypted string
  iops int
  snapshotId string

  // Relevant instance information
  instanceName string
  instanceID *string
  availabilityZone *string
}
