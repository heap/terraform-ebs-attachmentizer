package main

import (
	"encoding/json"
	"reflect"
	"testing"

	tf "github.com/hashicorp/terraform/terraform"
)

func TestMakeVolumeRes(t *testing.T) {
	var testCases = []struct {
		in  BlockDevice
		out tf.ResourceState
	}{
		{
			BlockDevice{
				volumeID:            "v-abcd",
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			tf.ResourceState{
				Type: "aws_ebs_volume",
				Primary: &tf.InstanceState{
					ID: "v-abcd",
					Attributes: map[string]string{
						"size":              "10",
						"type":              "gp2",
						"id":                "v-abcd",
						"encrypted":         "false",
						"availability_zone": "us-east-1",
						"snapshot_id":       "",
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		actual := tt.in.makeVolumeRes()
		if !actual.Equal(&tt.out) {
			actual, _ := json.MarshalIndent(actual, "", "  ")
			expected, _ := json.MarshalIndent(tt.out, "", "  ")
			t.Errorf("Expected: %+v\nGot: %+v", string(expected), string(actual))
		}
	}
}

func TestMakeAttachmentRes(t *testing.T) {
	var testCases = []struct {
		in  BlockDevice
		out tf.ResourceState
	}{
		{
			BlockDevice{
				volumeID:            "v-abcd",
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			tf.ResourceState{
				Type: "aws_volume_attachment",
				Primary: &tf.InstanceState{
					ID: "vai-3194341925",
					Attributes: map[string]string{
						"device_name": "/dev/xvdb",
						"instance_id": "i-1d7683bd",
						"volume_id":   "v-abcd",
						"id":          "vai-3194341925",
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		actual := tt.in.makeAttachmentRes()
		if !actual.Equal(&tt.out) {
			actual, _ := json.MarshalIndent(actual, "", "  ")
			expected, _ := json.MarshalIndent(tt.out, "", "  ")
			t.Errorf("Expected: %+v\nGot: %+v", string(expected), string(actual))
		}
	}
}

func TestBlockDeviceCrossValidation(t *testing.T) {
	var testCases = []struct {
		fromEC2 BlockDevice
		fromTF  BlockDevice
		out     bool
	}{
		{
			BlockDevice{
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
			},
			BlockDevice{
				volumeID:            "v-abcd",
				deviceName:          NewDeviceName("xvdb"),
				deleteOnTermination: "false",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			true,
		},
		{
			BlockDevice{
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
			},
			BlockDevice{
				volumeID:            "v-abcd",
				deviceName:          NewDeviceName("/dev/xvdb"),
				deleteOnTermination: "false",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			true,
		},
		{
			BlockDevice{
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
			},
			BlockDevice{
				volumeID:            "v-abcd",
				deviceName:          NewDeviceName("/dev/xvdb1"),
				deleteOnTermination: "false",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			false,
		},
		{
			BlockDevice{
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
			},
			BlockDevice{
				volumeID:            "v-abcd",
				deviceName:          NewDeviceName("/dev/xvdb"),
				deleteOnTermination: "true",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			false,
		},
	}
	for _, tt := range testCases {
		actual := validateEC2andTFDevs(tt.fromTF, tt.fromEC2)
		if !(actual == tt.out) {
			t.Errorf("Expected cross validation %t, got %t", tt.out, actual)
		}
	}
}

func TestBlockDeviceValidation(t *testing.T) {
	var testCases = []struct {
		in  BlockDevice
		out bool
	}{
		{
			BlockDevice{
				volumeID:            "v-abcd",
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			true,
		},
		{
			BlockDevice{
				volumeID:            "v-abcd",
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName(""),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			false,
		},
		{
			BlockDevice{
				volumeID:            "",
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			false,
		},
		{
			BlockDevice{
				volumeID:            "v-abcd",
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "",
			},
			false,
		},
		{
			BlockDevice{
				volumeID:            "v-abcd",
				size:                10,
				volumeType:          "",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			true,
		},
		{
			BlockDevice{
				volumeID:            "v-abcd",
				size:                0,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "i-1d7683bd",
				availabilityZone:    "us-east-1",
			},
			false,
		},
		{
			BlockDevice{
				volumeID:            "v-abcd",
				size:                10,
				volumeType:          "gp2",
				deleteOnTermination: "false",
				deviceName:          NewDeviceName("xvdb"),
				encrypted:           "false",
				iops:                1500,
				snapshotId:          "",
				instanceID:          "",
				availabilityZone:    "us-east-1",
			},
			false,
		},
	}

	for i, tt := range testCases {
		actual := validateBlockDev(tt.in)
		if !(actual == tt.out) {
			t.Errorf("[%d] Expected validation %t, got %t", i, tt.out, actual)
		}
	}
}

func TestParseTerraformName(t *testing.T) {
	var testCases = []struct {
		in  string
		out *TerraformName
	}{
		{
			"aws_instance.abc",
			&TerraformName{
				resourceType: "aws_instance",
				name:         "abc",
				index:        -1,
			},
		},
		{
			"aws_instance.abc.7",
			&TerraformName{
				resourceType: "aws_instance",
				name:         "abc",
				index:        7,
			},
		},
	}

	for _, tt := range testCases {
		actual, err := ParseTerraformName(tt.in)
		if err != nil && tt.out != nil {
			t.Errorf("Unexptected failure on %v: %v", tt.in, err)
		}
		if !reflect.DeepEqual(*actual, *tt.out) {
			t.Errorf("Expected %+v, got %+v", tt.out, actual)
		}
	}
}
