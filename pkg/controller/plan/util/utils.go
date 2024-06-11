package util

import (
	"math"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/settings"
	core "k8s.io/api/core/v1"
)

// Disk alignment size used to align FS overhead,
// its a multiple of all known hardware block sizes 512/4k/8k/32k/64k
const (
	DefaultAlignBlockSize = 1024 * 1024
)

func roundUp(requestedSpace, multiple int64) int64 {
	if multiple == 0 {
		return requestedSpace
	}
	partitions := math.Ceil(float64(requestedSpace) / float64(multiple))
	return int64(partitions) * multiple
}

func CalculateSpaceWithOverhead(requestedSpace int64, volumeMode *core.PersistentVolumeMode) int64 {
	alignedSize := roundUp(requestedSpace, DefaultAlignBlockSize)
	var spaceWithOverhead int64
	if *volumeMode == core.PersistentVolumeFilesystem {
		spaceWithOverhead = int64(math.Ceil(float64(alignedSize) / (1 - float64(settings.Settings.FileSystemOverhead)/100)))
	} else {
		spaceWithOverhead = alignedSize + settings.Settings.BlockOverhead
	}
	return spaceWithOverhead
}

type HostsFunc func() (map[string]*api.Host, error)
