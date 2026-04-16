package metrics

import (
	"strconv"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/klog/v2"
)

// Label names used for copy metrics.
const (
	labelOwnerUID        = "owner_uid"
	labelResult          = "result"
	labelStorageVendor   = "storage_vendor"
	labelCloneMethod     = "clone_method"
	labelStorageProtocol = "storage_protocol"
	labelXcopyUsed       = "xcopy_used"
	labelArrayVendor     = "storage_array_vendor"
	labelArrayProduct    = "storage_array_product"
	labelArrayModel      = "storage_array_model"
	labelArrayVersion    = "storage_array_version"
	labelDiskSizeType    = "type"
)

// CopyMetrics holds Prometheus metrics for copy operations and provides labeled recording.
type CopyMetrics struct {
	progressCounter       *prometheus.CounterVec
	xcopyUsedGauge        *prometheus.GaugeVec
	storageArrayInfoGauge *prometheus.GaugeVec
	sourceDiskBytesGauge  *prometheus.GaugeVec
	copyDurationGauge     *prometheus.GaugeVec
}

func (m *CopyMetrics) RecordProgress(ownerUID string, progress uint64) {
	metric := dto.Metric{}
	c := m.progressCounter.WithLabelValues(ownerUID)
	if err := c.Write(&metric); err != nil {
		klog.Error(err)
		return
	}
	if float64(progress) > metric.Counter.GetValue() {
		c.Add(float64(progress) - metric.Counter.GetValue())
	}
}

func (m *CopyMetrics) RecordXcopyUsed(ownerUID, storageVendor, cloneMethod string, xcopyUsed int) {
	m.xcopyUsedGauge.WithLabelValues(ownerUID, storageVendor, cloneMethod).Set(float64(xcopyUsed))
}

// RecordStorageArrayInfo sets the info metric once with storage array metadata (constant value 1).
func (m *CopyMetrics) RecordStorageArrayInfo(storageVendor string, arrayInfo populator.StorageArrayInfo) {
	model := arrayInfo.Model
	if model == "" {
		model = "n/a"
	}
	ver := arrayInfo.Version
	if ver == "" {
		ver = "n/a"
	}
	m.storageArrayInfoGauge.WithLabelValues(storageVendor, arrayInfo.Vendor, arrayInfo.Product, model, ver).Set(1)
}

// RecordSourceDiskBytes records source disk size metrics for each available measurement.
func (m *CopyMetrics) RecordSourceDiskBytes(ownerUID, storageVendor, cloneMethod, storageProtocol string, sourceDiskCapacityBytes, sourceDatastoreAllocatedBytes int64) {
	if sourceDatastoreAllocatedBytes > 0 {
		m.sourceDiskBytesGauge.WithLabelValues(ownerUID, storageVendor, cloneMethod, storageProtocol, "datastore_allocated").Set(float64(sourceDatastoreAllocatedBytes))
	}
	if sourceDiskCapacityBytes > 0 {
		m.sourceDiskBytesGauge.WithLabelValues(ownerUID, storageVendor, cloneMethod, storageProtocol, "provisioned").Set(float64(sourceDiskCapacityBytes))
	}
}

// RecordCompletion records copy duration with a result label ("success" or "failure").
func (m *CopyMetrics) RecordCompletion(result, ownerUID, storageVendor, cloneMethod, storageProtocol string, xcopyUsed int, duration time.Duration) {
	xcopyUsedStr := strconv.Itoa(xcopyUsed)
	if xcopyUsedStr != "0" && xcopyUsedStr != "1" {
		xcopyUsedStr = "0"
	}
	labels := []string{ownerUID, result, storageVendor, cloneMethod, xcopyUsedStr, storageProtocol}
	m.copyDurationGauge.WithLabelValues(labels...).Set(duration.Seconds())
}

// NewCopyMetrics creates and registers all Prometheus metrics for copy operations.
func NewCopyMetrics() (*CopyMetrics, error) {
	// Progress is emitted during the copy and scraped by the controller to update CR status.
	// Only owner_uid is needed — the controller matches on it to find the right metric line.
	progressCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vsphere_xcopy_volume_populator_progress",
			Help: "Progress of vsphere XCOPY volume population (percentage 0-100).",
		},
		[]string{labelOwnerUID},
	)
	// xcopy_used is emitted during the copy before storageProtocol is known.
	baseLabels := []string{labelOwnerUID, labelStorageVendor, labelCloneMethod}
	xcopyUsedGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vsphere_xcopy_volume_populator_xcopy_used",
			Help: "Indicates whether XCOPY was used for cloning (0=no, 1=yes). Labels describe what was copied.",
		},
		baseLabels,
	)

	// Info metric (constant value 1) for storage array metadata. Correlate with other metrics via storage_vendor.
	storageArrayInfoGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vsphere_xcopy_volume_populator_storage_array_info",
			Help: "Storage array metadata (constant 1). Join with other metrics on storage_vendor to get array details.",
		},
		[]string{labelStorageVendor, labelArrayVendor, labelArrayProduct, labelArrayModel, labelArrayVersion},
	)

	// Source disk size in bytes, with type label for each measurement source.
	sourceDiskBytesGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vsphere_xcopy_volume_populator_source_disk_bytes",
			Help: "Source disk size in bytes. type=datastore_allocated is actual data on datastore (layoutEx extents); provisioned is guest-visible disk size.",
		},
		[]string{labelOwnerUID, labelStorageVendor, labelCloneMethod, labelStorageProtocol, labelDiskSizeType},
	)

	// Duration of copy in seconds. Its presence signals completion (success or failure).
	completionLabels := []string{labelOwnerUID, labelResult, labelStorageVendor, labelCloneMethod, labelXcopyUsed, labelStorageProtocol}
	copyDurationGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vsphere_xcopy_volume_populator_copy_duration_seconds",
			Help: "Duration of copy operation in seconds. result label indicates success or failure.",
		},
		completionLabels,
	)

	for _, collector := range []prometheus.Collector{progressCounter, xcopyUsedGauge, storageArrayInfoGauge, sourceDiskBytesGauge, copyDurationGauge} {
		if err := prometheus.Register(collector); err != nil {
			return nil, err
		}
	}

	return &CopyMetrics{
		progressCounter:       progressCounter,
		xcopyUsedGauge:        xcopyUsedGauge,
		storageArrayInfoGauge: storageArrayInfoGauge,
		sourceDiskBytesGauge:  sourceDiskBytesGauge,
		copyDurationGauge:     copyDurationGauge,
	}, nil
}
