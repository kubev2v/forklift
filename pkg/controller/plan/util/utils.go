package util

import (
	"math"
	"strconv"

	"github.com/konveyor/forklift-controller/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	ocpclient "github.com/konveyor/forklift-controller/pkg/lib/client/openshift"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
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

func CalculateAPIGroup(kind string, provider *api.Provider, secret *core.Secret) (*schema.GroupVersionKind, error) {
	// If OCP version is >= 4.16 use forklift.cdi.konveyor.io
	// Otherwise use forklift.konveyor.io
	restCfg := ocpclient.RestCfg(provider, secret)
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	discoveryClient := clientset.Discovery()
	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	major, err := strconv.Atoi(version.Major)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	minor, err := strconv.Atoi(version.Minor)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	if major < 1 || (major == 1 && minor <= 28) {
		return &schema.GroupVersionKind{Group: "forklift.konveyor.io", Version: "v1beta1", Kind: kind}, nil
	}

	return &schema.GroupVersionKind{Group: "forklift.cdi.konveyor.io", Version: "v1beta1", Kind: kind}, nil
}
