package migrator

// EC2-specific migration phases define the internal state machine steps.
//
// These phase constants control the execution flow of the migration itinerary.
// Each phase corresponds to a step in ExecutePhase() that performs specific
// migration operations. The itinerary in Itinerary() defines the order of
// phases and their conditional execution.
const (
	// PhaseCreateSnapshots controls the EBS snapshot creation phase.
	// During this phase, the migrator:
	//   - Extracts EBS volume IDs from the EC2 instance
	//   - Creates snapshots for all EBS volumes via the EC2 API
	//   - Tags snapshots with VM name and volume ID for tracking
	// This phase advances to PhaseWaitForSnapshots when snapshots are initiated.
	PhaseCreateSnapshots = "CreateSnapshots"

	// PhaseWaitForSnapshots controls the snapshot completion polling phase.
	// During this phase, the migrator:
	//   - Queries AWS for snapshots tagged with VM name
	//   - Polls the EC2 API to check snapshot status
	//   - Waits until all snapshots reach "completed" state
	// This phase advances to PhaseShareSnapshots (cross-account) or PhaseCreateVolumes.
	PhaseWaitForSnapshots = "WaitForSnapshots"

	// PhaseShareSnapshots controls the cross-account snapshot sharing phase.
	// This phase is only executed when cross-account mode is enabled.
	// During this phase, the migrator:
	//   - Retrieves the target account ID using STS
	//   - Shares all snapshots with the target account
	//   - Enables the target account to create volumes from the shared snapshots
	// This phase advances to PhaseCreateVolumes when sharing is complete.
	PhaseShareSnapshots = "ShareSnapshots"

	// PhaseCreateVolumes controls the EBS volume creation phase.
	// During this phase, the migrator:
	//   - Queries AWS for snapshots tagged with VM name
	//   - Creates EBS volumes from snapshots in the target availability zone
	//   - Tags volumes with VM name and original volume ID for tracking
	// This phase advances to PhaseWaitForVolumes when volumes are initiated.
	PhaseCreateVolumes = "CreateVolumes"

	// PhaseWaitForVolumes controls the volume availability polling phase.
	// During this phase, the migrator:
	//   - Queries AWS for volumes tagged with VM name
	//   - Polls the EC2 API to check volume status
	//   - Waits until all volumes reach "available" state
	// This phase advances to PhaseCreatePVsAndPVCs when all volumes are ready.
	PhaseWaitForVolumes = "WaitForVolumes"

	// PhaseCreatePVsAndPVCs controls the Kubernetes PV/PVC creation phase.
	// During this phase, the migrator:
	//   - Creates PersistentVolumes with CSI volume source pointing to EBS volumes
	//   - Creates PersistentVolumeClaims pre-bound to the PVs
	//   - Waits until all PVCs are bound
	// This phase advances to PhaseFinalize when all PVCs are bound.
	PhaseCreatePVsAndPVCs = "CreatePVsAndPVCs"

	// PhaseRemoveSnapshots controls the cleanup phase for EBS snapshots.
	// During this phase, the migrator:
	//   - Queries AWS for snapshots tagged with VM name
	//   - Deletes all EBS snapshots via the EC2 API
	// This phase runs after VM creation and advances to PostHook or Completed.
	// Errors during cleanup are logged but don't fail the migration.
	PhaseRemoveSnapshots = "RemoveSnapshots"
)

// UI pipeline steps shown in migration status for progress reporting, error tracking, logs.
// Each maps to internal phases in Pipeline() method.
const (
	Initialize = "Initialize" // First step: validates VM, initializes tracking (PhaseStarted)

	// PrepareSource indicates the source VM is being prepared for migration.
	// Corresponds to: PhasePowerOffSource, PhaseWaitForPowerOff
	// This step stops the EC2 instance to ensure data consistency.
	PrepareSource = "PrepareSource"

	// CreateSnapshots indicates EBS snapshots are being created and verified.
	// Corresponds to: PhaseCreateSnapshots, PhaseWaitForSnapshots
	// This step creates snapshots and waits for them to complete.
	CreateSnapshots = "CreateSnapshots"

	// ShareSnapshots indicates EBS snapshots are being shared with the target account.
	// Corresponds to: PhaseShareSnapshots
	// This step is only shown for cross-account migrations.
	ShareSnapshots = "ShareSnapshots"

	// DiskTransfer indicates disks are being transferred/populated.
	// Corresponds to: PhaseCreateVolumes, PhaseWaitForVolumes, PhaseCreatePVsAndPVCs
	// This step creates EBS volumes from snapshots, then creates PV/PVC pairs
	// with CSI volume sources pointing directly to the EBS volumes.
	DiskTransfer = "DiskTransfer"

	// CreateVM indicates the KubeVirt VirtualMachine is being created.
	// Corresponds to: PhaseFinalize, PhaseCreateVM
	// This step builds the VirtualMachine spec and creates it in the target cluster.
	CreateVM = "CreateVM"

	// ImageConversion indicates the image is being converted to KubeVirt format.
	// Corresponds to: PhaseCreateGuestConversionPod, PhaseConvertGuest
	ImageConversion = "ImageConversion"

	// Cleanup indicates EBS snapshots and temporary resources are being removed.
	// Corresponds to: PhaseRemoveSnapshots
	// This step deletes the EBS snapshots after migration.
	Cleanup = "Cleanup"
)
