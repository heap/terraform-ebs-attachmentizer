package common

import (
	"fmt"
	"strings"
)

// TODO: make this more robust.
func NormalizeDeviceName(dev string) string {
	if strings.HasPrefix(dev, "/dev/") {
		return dev
	} else {
		return fmt.Sprintf("/dev/%v", dev)
	}
}
