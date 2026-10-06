package blockdev

import (
	"encoding/json"
	"testing"
)

func TestMountedAnywhere(t *testing.T) {
	const fakeLsblk = `{
  "blockdevices": [
    { "name": "sda", "type": "disk", "ro": false, "size": 1000000, "mountpoint": null,
      "children": [
        { "name": "sda1", "type": "part", "ro": false, "size": 1000000, "mountpoint": "/" }
      ] },
    { "name": "sdb", "type": "disk", "ro": false, "size": 2048, "mountpoint": null,
      "children": [
        { "name": "sdb1", "type": "part", "ro": false, "size": 2000, "mountpoint": null }
      ] }
  ]
}`

	var parsed struct {
		BlockDevices []lsblkNode `json:"blockdevices"`
	}
	if err := json.Unmarshal([]byte(fakeLsblk), &parsed); err != nil {
		t.Fatal(err)
	}

	if !mountedAnywhere(parsed.BlockDevices[0]) {
		t.Error("sda should count as mounted (partition at /)")
	}
	if mountedAnywhere(parsed.BlockDevices[1]) {
		t.Error("sdb should not count as mounted")
	}
}
