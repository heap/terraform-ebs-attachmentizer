package main

import (
	"encoding/json"
	"testing"

	tf "github.com/hashicorp/terraform/terraform"
)

// Make a string pointer from a string. Works around Go not allowing you to
// take address of a literal.
func sp(s string) *string {
	return &s
}

func TestMakeVolumeRes(t *testing.T) {
	var testCases = []struct {
		in  BlockDevice
		out tf.ResourceState
	}{
		{
			BlockDevice{
				volumeID:   sp("v-abcd"),
				size: 10,
				volumeType: "gp2",
        deleteOnTermination: "false",
        deviceName: "xvdb",
        encrypted: "false",
        iops: 1500,
        snapshotId: "",
        instanceName: "instance01",
        instanceID: sp("i-1d7683bd"),
        availabilityZone: sp("us-east-1"),
			},
			tf.ResourceState{
				Type: "aws_ebs_volume",
				Primary: &tf.InstanceState{
					ID: "v-abcd",
					Attributes: map[string]string{
						"size": "10",
						"type": "gp2",
            "id": "v-abcd",
            "encrypted": "false",
            "availability_zone": "us-east-1",
            "snapshot_id": "",
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		actual := makeVolumeRes(tt.in)
		if !actual.Equal(&tt.out) {
			actual, _ := json.MarshalIndent(actual, "", "  ")
			expected, _ := json.MarshalIndent(tt.out, "", "  ")
			t.Errorf("Expected: %+v\nGot: %+v", string(expected), string(actual))
		}
	}
}

func TestMakeVolumeAttachmentRes(t *testing.T) {
}
