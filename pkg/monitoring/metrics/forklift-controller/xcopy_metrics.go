package forklift_controller

import (
	"context"
	"strconv"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var processedXcopyCompletions = make(map[string]struct{})

func RecordXcopyMetrics(c client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			populators := api.VSphereXcopyVolumePopulatorList{}
			err := c.List(context.TODO(), &populators, client.MatchingLabels{"forklift.konveyor.io/offload-completed": "true"})
			if err != nil {
				klog.Errorf("Metrics xcopy populators list error: %v", err)
				continue
			}

			for _, p := range populators.Items {
				if p.Status.Result == "" {
					continue
				}

				key := string(p.UID)
				if _, exists := processedXcopyCompletions[key]; exists {
					continue
				}

				migration := p.Labels["migration"]
				ownerUID := ""
				if len(p.OwnerReferences) > 0 {
					ownerUID = string(p.OwnerReferences[0].UID)
				}

				result := p.Status.Result
				vendor := p.Status.StorageVendor
				method := p.Status.CloneMethod
				xcopy := p.Status.XcopyUsed
				protocol := p.Status.StorageProtocol
				vibVersion := p.Status.VibVersion

				if p.Status.CopyDurationSeconds != "" {
					duration, err := strconv.ParseFloat(p.Status.CopyDurationSeconds, 64)
					if err == nil {
						xcopyDurationGauge.With(prometheus.Labels{
							"result":           result,
							"migration":        migration,
							"owner_uid":        ownerUID,
							"storage_vendor":   vendor,
							"clone_method":     method,
							"xcopy_used":       xcopy,
							"storage_protocol": protocol,
							"vib_version":      vibVersion,
						}).Set(duration)
					}
				}

				if p.Status.ProvisionedBytes != "" {
					prov, err := strconv.ParseFloat(p.Status.ProvisionedBytes, 64)
					if err == nil && prov > 0 {
						xcopySourceDiskBytesGauge.With(prometheus.Labels{
							"result":           result,
							"migration":        migration,
							"owner_uid":        ownerUID,
							"storage_vendor":   vendor,
							"clone_method":     method,
							"xcopy_used":       xcopy,
							"storage_protocol": protocol,
							"vib_version":      vibVersion,
							"type":             "provisioned",
						}).Set(prov)
					}
				}

				if p.Status.AllocatedBytes != "" {
					alloc, err := strconv.ParseFloat(p.Status.AllocatedBytes, 64)
					if err == nil && alloc > 0 {
						xcopySourceDiskBytesGauge.With(prometheus.Labels{
							"result":           result,
							"migration":        migration,
							"owner_uid":        ownerUID,
							"storage_vendor":   vendor,
							"clone_method":     method,
							"xcopy_used":       xcopy,
							"storage_protocol": protocol,
							"vib_version":      vibVersion,
							"type":             "datastore_allocated",
						}).Set(alloc)
					}
				}

				processedXcopyCompletions[key] = struct{}{}
			}
		}
	}()
}
