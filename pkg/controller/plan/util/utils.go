package util

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Disk alignment size used to align FS overhead,
// its a multiple of all known hardware block sizes 512/4k/8k/32k/64k
const (
	DefaultAlignBlockSize = 1024 * 1024
)

// KubeVirt resource kind used in owner references and type metadata.
const VirtualMachineKind = "VirtualMachine"

// NetApp Shift PVC annotations.
const (
	AnnNfsServer = "forklift.konveyor.io/nfs-server"
	AnnNfsPath   = "forklift.konveyor.io/nfs-path"
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

const (
	cdiConfigName         = "config"
	defaultGlobalOverhead = 0.055
)

// CalculateSpaceWithCDIOverhead computes the PVC size the same way CDI does:
// it reads CDIConfig.Status.FilesystemOverhead from the destination cluster,
// uses the per-StorageClass overhead when that key exists (invalid values return
// an error), otherwise uses Global when set (invalid values return an error),
// otherwise the CDI default of 5.5%, aligns the raw capacity, and inflates it by
// ceil(aligned / (1 - overhead)).
func CalculateSpaceWithCDIOverhead(client k8sclient.Client, storageClassName string, rawCapacity int64) (int64, error) {
	if client == nil {
		return 0, fmt.Errorf("destination client is nil, cannot read CDIConfig")
	}
	overhead := defaultGlobalOverhead

	cfg := &cdi.CDIConfig{}
	if err := client.Get(context.TODO(), k8sclient.ObjectKey{Name: cdiConfigName}, cfg); err != nil {
		return 0, err
	}
	if fo := cfg.Status.FilesystemOverhead; fo != nil {
		if scPercent, hasSC := fo.StorageClass[storageClassName]; hasSC {
			v, err := strconv.ParseFloat(string(scPercent), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid filesystem overhead for storage class %q: %w", storageClassName, err)
			}
			if v < 0 || v >= 1 {
				return 0, fmt.Errorf("filesystem overhead for storage class %q must be in [0,1), got %g", storageClassName, v)
			}
			overhead = v
		} else if fo.Global != "" {
			v, err := strconv.ParseFloat(string(fo.Global), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid global filesystem overhead: %w", err)
			}
			if v < 0 || v >= 1 {
				return 0, fmt.Errorf("global filesystem overhead must be in [0,1), got %g", v)
			}
			overhead = v
		}
	}

	aligned := RoundUp(rawCapacity, DefaultAlignBlockSize)
	inflated := int64(math.Ceil(float64(aligned) / (1.0 - overhead)))
	return inflated, nil
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

// ChangeVmName changes VM name to match DNS1123 RFC convention.
func ChangeVmName(currName string) string {
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
		newName = "vm-" + GenerateRandomSuffix()
	}

	return newName
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

// IsNetAppShiftPersistentVolumeClaim reports whether a PVC is a NetApp Shift volume.
func IsNetAppShiftPersistentVolumeClaim(ann map[string]string) bool {
	if ann == nil {
		return false
	}
	_, hasServer := ann[AnnNfsServer]
	_, hasExport := ann[AnnNfsPath]
	return hasServer && hasExport
}

func AnyNetAppShiftPersistentVolumeClaim(pvcs []*core.PersistentVolumeClaim) bool {
	for _, pvc := range pvcs {
		if pvc != nil && IsNetAppShiftPersistentVolumeClaim(pvc.Annotations) {
			return true
		}
	}
	return false
}
