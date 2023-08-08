package util

import "math"

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

func CalculateSpaceWithOverhead(requestedSpace int64, filesystemOverhead float64) int64 {
	alignedSize := roundUp(requestedSpace, DefaultAlignBlockSize)
	spaceWithOverhead := int64(math.Ceil(float64(alignedSize) / (1 - filesystemOverhead)))
	return spaceWithOverhead
}
