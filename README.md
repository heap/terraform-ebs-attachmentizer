# blkdev2volatt

Convert `ebs_block_device` blocks in `aws_instance` resource to `aws_ebs_volume`
and `aws_volume_attachment` resources.

(I'm bad at names.)

## Why

Terraform lets you represent the EBS volumes attached to an instance in two
ways. The first is as `ebs_block_device` blocks directly in the instance
resource:

    resource "aws_instance" "my_instance" {
        ...
        ebs_block_device {
            size = 100
            type = "gp2"
            device_name = "/dev/xvdb"
        }

        ebs_block_device {
            size = 500
            type = "gp2"
            device_name = "/dev/xvdc"
        }
        ...
    }

The second is as `aws_ebs_volume` resources, with an `aws_volume_attachment`
each to attach them to an instance:

    resource "aws_instance" "my_instance" {
        ...
    }

    resource "aws_ebs_volume" "vol1" {
        size = 100
        type = "gp2"
        availability_zone = "${aws_instance.my_instance.availability_zone}"
    }

    resource "aws_ebs_volume" "vol2" {
        size = 500
        type = "gp2"
        availability_zone = "${aws_instance.my_instance.availability_zone}"
    }

    resource "aws_volume_attachment" "vol1" {
        instance_id = "${aws_instance.my_instance.id}"
        volume_id = "${aws_ebs_volume.vol1.id}"
        device_name = "/dev/xvdb"
    }

    resource "aws_volume_attachment" "vol2" {
        instance_id = "${aws_instance.my_instance.id}"
        volume_id = "${aws_ebs_volume.vol2.id}"
        device_name = "/dev/xvdc"
    }

The first is obviously less verbose, but has the drawback of making it
impossible to add or remove EBS volumes via Terraform. With the second, it's
easy: just delete the `aws_volume_attachment` resource.


## How

The tl;dr is:

1. gather volume and instance data from AWS
2. read the Terraform state file
3. remove the `ebs_block_device` blocks from the state file, and add the new
   resources

Ideally we could do this without access to AWS, but the Terraform state doesn't
include the volume IDs when you use `ebs_block_device` blocks. So read-only
access to EC2 is required.


## Development

The files have some doc comments explaining what's in each, but the high
level view is

- `terraform.go` handles reading of Terraform state
- `ec2.go` handles reading from the AWS API
- `common.go` has some common things like a utilty for dealing with the
  fact that either Terraform or AWS lets you call a device either
  `/dev/xvdb` or `xvdb` and "does the right thing".

This project uses Terraform as a library, and uses the official AWS Go SDK. The
godoc for these will be handy:

- [AWS SDK's EC2 package](https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/)
- [Terraform's State type](https://godoc.org/github.com/hashicorp/terraform/terraform#State)

It's also *really* instructive to look at some tfstate files and compare to the
corresponding Terraform config.

### Building

You'll need to place this in the right place in your `GOPATH`. After that, it
should build with

    go build -i


### Dependencies

Using [dep][0] despite its non-production status because it's the first one
I tried that I could make work.

To add a dependency:

    dep ensure depname@version

[0]: https://github.com/golang/dep
