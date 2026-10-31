package forklift_controller

import (
	"context"
	"strconv"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var processedXcopyCompletions = make(map[string]struct{})

func RecordXcopyMetrics(c client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			populators, err := listOffloadCompletions(c)
			if err != nil {
				klog.Errorf("Metrics xcopy populators list error: %v", err)
				continue
			}

			for _, p := range populators {
				if p.result == "" {
					continue
				}

				key := p.uid
				if _, exists := processedXcopyCompletions[key]; exists {
					continue
				}

				migration := p.migration
				ownerUID := p.ownerUID

				result := p.result
				vendor := p.storageVendor
				method := p.cloneMethod
				xcopy := p.xcopyUsed
				protocol := p.storageProtocol
				vibVersion := p.vibVersion

				if p.copyDurationSeconds != "" {
					duration, err := strconv.ParseFloat(p.copyDurationSeconds, 64)
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

				if p.provisionedBytes != "" {
					prov, err := strconv.ParseFloat(p.provisionedBytes, 64)
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

				if p.allocatedBytes != "" {
					alloc, err := strconv.ParseFloat(p.allocatedBytes, 64)
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

type offloadCompletion struct {
	uid                 string
	migration           string
	ownerUID            string
	result              string
	storageVendor       string
	cloneMethod         string
	xcopyUsed           string
	storageProtocol     string
	vibVersion          string
	copyDurationSeconds string
	provisionedBytes    string
	allocatedBytes      string
}

func firstOwnerUID(refs []metav1.OwnerReference) string {
	if len(refs) > 0 {
		return string(refs[0].UID)
	}
	return ""
}

func listOffloadCompletions(c client.Client) (completions []offloadCompletion, err error) {
	completed := client.MatchingLabels{"forklift.konveyor.io/offload-completed": "true"}

	xcopyList := api.VSphereXcopyVolumePopulatorList{}
	if err = c.List(context.TODO(), &xcopyList, completed); err != nil {
		return nil, err
	}
	for _, p := range xcopyList.Items {
		completions = append(completions, offloadCompletion{
			uid:                 string(p.UID),
			migration:           p.Labels["migration"],
			ownerUID:            firstOwnerUID(p.OwnerReferences),
			result:              p.Status.Result,
			storageVendor:       p.Status.StorageVendor,
			cloneMethod:         p.Status.CloneMethod,
			xcopyUsed:           p.Status.XcopyUsed,
			storageProtocol:     p.Status.StorageProtocol,
			vibVersion:          p.Status.VibVersion,
			copyDurationSeconds: p.Status.CopyDurationSeconds,
			provisionedBytes:    p.Status.ProvisionedBytes,
			allocatedBytes:      p.Status.AllocatedBytes,
		})
	}

	nfsList := api.VSphereNfsCopyVolumePopulatorList{}
	if err = c.List(context.TODO(), &nfsList, completed); err != nil {
		return nil, err
	}
	for _, p := range nfsList.Items {
		completions = append(completions, offloadCompletion{
			uid:                 string(p.UID),
			migration:           p.Labels["migration"],
			ownerUID:            firstOwnerUID(p.OwnerReferences),
			result:              p.Status.Result,
			storageVendor:       p.Status.StorageVendor,
			cloneMethod:         p.Status.CloneMethod,
			storageProtocol:     p.Status.StorageProtocol,
			copyDurationSeconds: p.Status.CopyDurationSeconds,
			provisionedBytes:    p.Status.ProvisionedBytes,
			allocatedBytes:      p.Status.AllocatedBytes,
		})
	}

	return completions, nil
}
