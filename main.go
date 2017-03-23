package main

import (
	"bytes"
	"fmt"
	"encoding/json"
	"log"
	"os"

	tf "github.com/hashicorp/terraform/terraform"
	"github.com/hashicorp/terraform/flatmap"
	tfhash "github.com/hashicorp/terraform/helper/hashcode"
	tfstate "github.com/hashicorp/terraform/state"
	"github.com/davecgh/go-spew/spew"
)

func main() {
	tfStateStuff(os.Args[1])
}

// aws_ebs_volume attributes from
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_ebs_volume.go#L27-L68
var awsEbsVolumeAttrs = map[string]struct{}{
	"availability_zone": struct{}{},
	"encrypted": struct{}{},
	"iops": struct{}{},
	"kms_key_id": struct{}{},
	"size": struct{}{},
	"snapshot_id": struct{}{},
	"type": struct{}{},
	"tags": struct{}{},
}

// aws_volume_attachment attrs from
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_volume_attachment.go#L23-L52
var awsVolumeAttachmentAttrs = map[string]struct{}{
	"device_name": struct{}{},
	"instance_id": struct{}{},
	"volume_id": struct{}{},
	"force_detach": struct{}{},
	"skip_destroy": struct{}{},
}

// aws_instance.ebs_block_device attributes from
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_instance.go#L214-L262
var awsInstanceEbsBlockDeviceAttrs = map[string]struct{}{
	"delete_on_termination": struct{}{},
	"device_name": struct{}{},
	"encrypted": struct{}{},
	"iops": struct{}{},
	"snapshot_id": struct{}{},
	"volume_size": struct{}{},
	"volume_type": struct{}{},
}

// From
//    https://github.com/hashicorp/terraform/blob/ef94acbf1f753dd1d03d3249cd58f4876cd19682/builtin/providers/aws/resource_aws_volume_attachment.go#L244-L251
// with hat-tip to:
//  - https://github.com/hashicorp/terraform/issues/8458#issuecomment-258831650
//  - https://github.com/foxsy/tfvolattid
func volumeAttachmentID(name, volumeID, instanceID string) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s-", name))
	buf.WriteString(fmt.Sprintf("%s-", instanceID))
	buf.WriteString(fmt.Sprintf("%s-", volumeID))

	return fmt.Sprintf("vai-%d", tfhash.String(buf.String()))
}

func makeVolumeRes(res *tf.ResourceState, dev map[string]interface{}) *tf.ResourceState {
	var attrs = make(map[string]string)
	for k, v := range dev {
		if _, ok := awsEbsVolumeAttrs[k]; ok {
			attrs[k] = fmt.Sprintf("%v", v)
		}
	}
	spew.Printf("vol attrs: %v\n", attrs)
	newRes := &tf.ResourceState{
		Type: "aws_ebs_volume",
		Primary: &tf.InstanceState{
			ID: "NEED TO GET FROM AWS",
			Attributes: attrs,
		},
	}
	return newRes
}

func makeAttachmentRes(res *tf.ResourceState, dev map[string]interface{}) *tf.ResourceState {
	attrs := make(map[string]string)
	for k, v := range dev {
		if _, ok := awsVolumeAttachmentAttrs[k]; ok {
			attrs[k] = fmt.Sprintf("%v", v)
		}
	}
	newRes := &tf.ResourceState{
		Type: "aws_volume_attachment",
		Primary: &tf.InstanceState{
			ID: "NEED TO GET FROM AWS",
			Attributes: attrs,
		},
	}
	return newRes
}

func tfStateStuff(fn string) {
	localState := tfstate.LocalState{Path: fn, PathOut: "/tmp/out.tfstate"}
	localState.RefreshState()
	root := localState.State().Modules[0]

	var newResources []*tf.ResourceState

	for name, res := range root.Resources {
		if res.Type != "aws_instance" {
			continue
		}
		instanceRes := res
		fmt.Printf("id: %v\n", res.Primary.ID)
		devices, ok := flatmap.Expand(res.Primary.Attributes, "ebs_block_device").([]interface{})
		if !ok {
			log.Fatalf("could not expand ebs_block_device for %v", name)
		}
		for _, dev := range devices {
			dev, ok := dev.(map[string]interface{})
			if !ok {
				log.Fatalf("Bad ebs_block_device block in %v", name)
			}
			volumeRes := makeVolumeRes(instanceRes, dev)
			attachmentRes := makeAttachmentRes(instanceRes, dev)
			newResources = append(newResources, volumeRes, attachmentRes)
		}

		json, _ := json.MarshalIndent(newResources, "", "  ")
		os.Stdout.Write(json)

		fmt.Println("-----")
		log.Fatal("DIE")
	}
}
