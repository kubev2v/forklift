package util

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"time"
	"unicode"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
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

// SanitizeLabel ensures a string is a valid Kubernetes DNS-1123 label.
func SanitizeLabel(currName string) string {
	var validParts []string
	const labelMax = validation.DNS1123LabelMaxLength

	notAllowedChars := regexp.MustCompile("[^a-z0-9-]")
	newName := strings.ToLower(currName)
	newName = strings.Trim(newName, ".-")

	// Split by dots and process each of the name parts
	parts := strings.Split(newName, ".")
	for _, part := range parts {
		part = strings.ReplaceAll(part, "_", "-")
		part = strings.ReplaceAll(part, "+", "-")
		part = strings.ReplaceAll(part, "*", "-")
		part = strings.ReplaceAll(part, " ", "-")
		part = strings.ReplaceAll(part, "/", "-")
		part = strings.ReplaceAll(part, "\\", "-")

		part = notAllowedChars.ReplaceAllString(part, "")

		part = strings.Trim(part, "-.")

		// Remove multiple dashes
		partsByDashes := strings.Split(part, "-")
		var cleanedParts []string
		for _, p := range partsByDashes {
			if p != "" {
				cleanedParts = append(cleanedParts, p)
			}
		}
		part = strings.Join(cleanedParts, "-")

		// Enforce per-label length and clean trailing separators
		if len(part) > labelMax {
			part = part[:labelMax]
			part = strings.Trim(part, "-")
		}

		// Add part only if not empty
		if part != "" {
			validParts = append(validParts, part)
		}
	}

	// Join valid parts with dots
	newName = strings.Join(validParts, ".")

	// Ensure length does not exceed max
	if len(newName) > labelMax {
		newName = newName[:labelMax]
		newName = strings.Trim(newName, ".-")
	}

	// Handle case where name is empty after all processing
	if newName == "" {
		//This keeps names stable even when the sanitized name is empty
		newName = "vm-" + fnv32String(currName)
	}

	return newName
}

func fnv32String(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum32())
}

// GenerateRandomSuffix generates a random string of length four, consisting of lowercase letters and digits.
func GenerateRandomSuffix() string {
	const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"
	source := rand.NewSource(time.Now().UTC().UnixNano())
	seededRand := rand.New(source)

	b := make([]byte, 4)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
