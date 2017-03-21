package main
import (
	"bytes"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	tf "github.com/hashicorp/terraform/terraform"
	tfstate "github.com/hashicorp/terraform/state"
	tfhash "github.com/hashicorp/terraform/helper/hashcode"
	flatmap "github.com/hashicorp/terraform/flatmap"
)

func main() {
	switch(os.Args[1]) {
	case "tf":
		tfStateStuff(os.Args[2])
	case "ec2":
		ec2Stuff(os.Args[2])
	}
}

type volAtt struct {
	name, instance, volume, device string
}

type ebsBlockDevice struct {
	volumeType, deviceName, snapshotID string
	deleteOnTermination, encrypted bool
	volumeSize, iops int
}

func ebdFromMap(m map[string]interface{}) (res *ebsBlockDevice, ok bool) {
	volumeType, ok := m["volume_type"].(string)
	if !ok { return }
	deviceName, ok := m["device_name"].(string)
	if !ok { return }
	snapshotID, ok := m["snapshot_id"].(string)
	if !ok { return }
	deleteOnTermination, ok := m["delete_on_termination"].(bool)
	if !ok { return }
	encrypted, ok := m["encrypted"].(bool)
	if !ok { return }
	volumeSizeStr, ok := m["volume_size"].(string)
	if !ok { return}
	volumeSize, err := strconv.Atoi(volumeSizeStr)
	if err != nil { return }
	iopsStr, ok := m["iops"].(string)
	iops, err := strconv.Atoi(iopsStr)
	if err != nil { return }

	res = &ebsBlockDevice{
		volumeType: volumeType,
		deviceName: deviceName,
		snapshotID: snapshotID,
		deleteOnTermination: deleteOnTermination,
		encrypted: encrypted,
		volumeSize: volumeSize,
		iops: iops,
	}
	ok = true
	return
}

// TODO: comment where this comes from.
func (v *volAtt) id() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s-", v.name))
	buf.WriteString(fmt.Sprintf("%s-", v.instance))
	buf.WriteString(fmt.Sprintf("%s-", v.volume))

	return fmt.Sprintf("vai-%d", tfhash.String(buf.String()))
}

func (v *volAtt) instanceState() *tf.InstanceState {
	return &tf.InstanceState {
		ID: v.id(),
		Attributes: map[string]string{
			"device_name": v.device,
			"instance_id": v.instance,
			"volume_id": v.volume,
		},
		Tainted: false,
	}
}


func ec2Stuff(inst string) {
	sess, err := session.NewSession()
	if err != nil {
		panic(err.Error())
	}
	svc := ec2.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String(inst),
				},
			},
		},
	}

	resp, err:= svc.DescribeInstances(params)
	if err != nil {
		panic(err.Error())
	}

	for _, resv := range resp.Reservations {
		for _, inst := range resv.Instances {
			for _, blkDev := range inst.BlockDeviceMappings {
				instance := inst.InstanceId
				volume := blkDev.Ebs.VolumeId
				device := blkDev.DeviceName
				att := volAtt {
					name: fmt.Sprintf("%v-%v-%v", *instance, *volume, *device),
					instance: *instance,
					volume: *volume,
					device: *device,
				}
				fmt.Printf("%v %v\n", att.id(), att)
				fmt.Println(att.instanceState())
			}
		}
	}
}

func tfStateStuff(fn string) {
	localState := tfstate.LocalState { Path: fn, PathOut: "" }
	localState.RefreshState()

	root := localState.State().Modules[0]

	for name, res := range root.Resources {
		if res.Type != "aws_instance" {
			continue
		}
		fmt.Printf("id: %v\n", res.Primary.ID)
		devices, ok := flatmap.Expand(res.Primary.Attributes, "ebs_block_device").([]interface{})
		if !ok {
			log.Fatalf("could not expand ebs_block_device for %v", name)
		}
		for _, dev := range devices {
			dev, ok := dev.(map[string]interface{})
			if !ok {
				log.Fatal("cast failed")
			}
			for k, v := range dev {
				fmt.Printf("%v: %v\n", k, reflect.TypeOf(v))
			}
		}

		fmt.Println("-----")
	}
}
