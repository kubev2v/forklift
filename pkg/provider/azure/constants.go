package azure

const (
	// AnnSourceID stores the full Azure ARM resource ID on K8s resources for traceability.
	AnnSourceID = "forklift.konveyor.io/source-id"

	// AnnSourceDiskID stores the full ARM ID of the source managed disk on PVCs.
	AnnSourceDiskID = "forklift.konveyor.io/source-disk-id"

	// AnnDiskSource identifies the source disk on a PVC (required by common getPVCs filter).
	AnnDiskSource = "forklift.konveyor.io/disk-source"

	// Kubernetes label and annotation keys used across Azure migration resources.
	LabelVMID         = "forklift.konveyor.io/vmID"
	LabelDiskIndex    = "forklift.konveyor.io/disk-index"
	AnnDiskIndex      = "forklift.konveyor.io/disk-index"
	AnnVolumeSnapshot = "forklift.konveyor.io/volumeSnapshot"

	// Azure resource tag keys (use '-' instead of '/' because Azure forbids '/' in tag names).
	TagVMID        = "forklift.konveyor.io-vmID"
	TagVMName      = "forklift.konveyor.io-vm-name"
	TagDisk        = "forklift.konveyor.io-disk"
	TagIndex       = "forklift.konveyor.io-index"
	TagCrossRegion = "forklift.konveyor.io-cross-region"
	TagSource      = "forklift.konveyor.io-source"
)
