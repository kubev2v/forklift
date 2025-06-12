package util

import (
	"math"
	"strings"
	"unicode"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
)

// Disk alignment size used to align FS overhead,
// its a multiple of all known hardware block sizes 512/4k/8k/32k/64k
const (
	DefaultAlignBlockSize = 1024 * 1024
)

// RootDisk prefix for boot order.
const (
	diskPrefix = "/dev/sd"
)

func RoundUp(requestedSpace, multiple int64) int64 {
	if multiple == 0 {
		return requestedSpace
	}
	partitions := math.Ceil(float64(requestedSpace) / float64(multiple))
	return int64(partitions) * multiple
}

func CalculateSpaceWithOverhead(requestedSpace int64, volumeMode *core.PersistentVolumeMode) int64 {
	alignedSize := RoundUp(requestedSpace, DefaultAlignBlockSize)
	var spaceWithOverhead int64
	if *volumeMode == core.PersistentVolumeFilesystem {
		spaceWithOverhead = int64(math.Ceil(float64(alignedSize) / (1 - float64(settings.Settings.FileSystemOverhead)/100)))
	} else {
		spaceWithOverhead = alignedSize + settings.Settings.BlockOverhead
	}
	return spaceWithOverhead
}

func GetBootDiskNumber(deviceString string) int {
	deviceNumber := GetDeviceNumber(deviceString)
	if deviceNumber == 0 {
		return 0
	} else {
		return deviceNumber - 1
	}
}

func GetDeviceNumber(deviceString string) int {
	if !(strings.HasPrefix(deviceString, diskPrefix) && len(deviceString) > len(diskPrefix)) {
		// In case we encounter an issue detecting the root disk order,
		// we will return zero to avoid failing the migration due to boot orde
		return 0
	}

	for i := len(diskPrefix); i < len(deviceString); i++ {
		if unicode.IsLetter(rune(deviceString[i])) {
			return int(deviceString[i] - 'a' + 1)
		}
	}
	return 0
}

type HostsFunc func() (map[string]*api.Host, error)
