# TODO

## Terraform

- Handle the attribute name differences mentioned in attrs.go. Probably easiest
  to change from set-like maps to actual maps of attribute names to attribute
  names and use that to replace the attribute keys?
- Figure out how to handle resources with a `count` where an extra `.<index>` is
  added to the name.
- Figure out how to write the changes out to state including bumping the version.
- Creating terraform resource names for the new resources. Maybe something like
  `instance_name_device_name` like `my_instance_xvdb`. Bonus points: allow
  specifying a template for this.

For the `count` one, I didn't see how Terraform did this.


## Testing

Ideally we'd have some unit tests or something. If I'm feeling up to it, I
might do some refactoring to maek that easier. But testing on sample resource
will work too.
