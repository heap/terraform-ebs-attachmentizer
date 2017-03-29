# blkdev2volatt

Convert `ebs_block_device` blocks in `aws_instance` resource to `aws_ebs_volume`
and `aws_volume_attachment` resources.

(I'm bad at names.)


## Building

Should build with

    go build -i


## Dependencies

Using [dep][0] despite its non-production status because it's the first one
I tried that I could make work.

To add a dependency:

    dep ensure depname@version

[0]: https://github.com/golang/dep
