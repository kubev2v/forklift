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
	//   - Stores snapshot IDs in step annotations for later use
	// This phase advances to PhaseWaitForSnapshots when snapshots are initiated.
	PhaseCreateSnapshots = "CreateSnapshots"

	// PhaseWaitForSnapshots controls the snapshot completion polling phase.
	// During this phase, the migrator:
	//   - Retrieves snapshot IDs from step annotations
	//   - Polls the EC2 API to check snapshot status
	//   - Waits until all snapshots reach "completed" state
	// This phase advances to PhaseCreateDataVolumes when all snapshots are ready.
	PhaseWaitForSnapshots = "WaitForSnapshots"

	// PhaseRemoveSnapshots controls the cleanup phase for EBS snapshots.
	// During this phase, the migrator:
	//   - Retrieves snapshot IDs from step annotations
	//   - Deletes all EBS snapshots via the EC2 API
	//   - Removes populator secrets from the target namespace
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

	// DiskTransfer indicates disks are being transferred/populated.
	// Corresponds to: PhaseCreateDataVolumes
	// This step creates PVCs with Ec2VolumePopulator references and waits for
	// the populator controller to create volumes from snapshots.
	DiskTransfer = "DiskTransfer"

	// CreateVM indicates the KubeVirt VirtualMachine is being created.
	// Corresponds to: PhaseFinalize, PhaseCreateVM
	// This step builds the VirtualMachine spec and creates it in the target cluster.
	CreateVM = "CreateVM"

	// Cleanup indicates EBS snapshots and temporary resources are being removed.
	// Corresponds to: PhaseRemoveSnapshots
	// This step deletes the EBS snapshots and populator secrets after migration.
	Cleanup = "Cleanup"
)
