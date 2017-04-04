package main

// This file contains all the attributes that the resrouces and blocks have in
// Terraform, represented as set-like maps.

// Annoyingly, a couple of things like the size have different attribute names
// depending on wehere they're defined.

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
