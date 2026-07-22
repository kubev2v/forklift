package migrator

const (
	PhaseCreatePreSnapshot           = "CreatePreSnapshot"
	PhaseWaitForPreSnapshot          = "WaitForPreSnapshot"
	PhaseDeallocateVM                = "DeallocateVM"
	PhaseWaitForDeallocation         = "WaitForDeallocation"
	PhaseCreateSnapshots             = "CreateSnapshots"
	PhaseWaitForSnapshots            = "WaitForSnapshots"
	PhaseDeletePreSnapshots          = "DeletePreSnapshots"
	PhaseCopySnapshotsCrossRegion    = "CopySnapshotsCrossRegion"
	PhaseWaitForCrossRegionSnapshots = "WaitForCrossRegionSnapshots"
	PhaseCreateSnapshotContent       = "CreateSnapshotContent"
	PhaseCreateVolumeSnapshot        = "CreateVolumeSnapshot"
	PhaseCreatePVCs                  = "CreatePVCs"
	PhaseWaitForPVCsBound            = "WaitForPVCsBound"
	PhaseInjectOwnerRefs             = "InjectOwnerRefs"
)

const (
	Initialize      = "Initialize"
	PrepareSource   = "PrepareSource"
	CreateSnapshots = "CreateSnapshots"
	DiskTransfer    = "DiskTransfer"
	CreateVM        = "CreateVM"
)
