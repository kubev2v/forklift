package base

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
)

// Predicates.
var (
	HasPreHook              libitr.Flag = 0x01
	HasPostHook             libitr.Flag = 0x02
	RequiresConversion      libitr.Flag = 0x04
	CDIDiskCopy             libitr.Flag = 0x08
	VirtV2vDiskCopy         libitr.Flag = 0x10
	OpenstackImageMigration libitr.Flag = 0x20
	VSphere                 libitr.Flag = 0x40
)

// Steps.
const (
	Initialize      = "Initialize"
	Cutover         = "Cutover"
	DiskAllocation  = "DiskAllocation"
	DiskTransfer    = "DiskTransfer"
	ImageConversion = "ImageConversion"
	DiskTransferV2v = "DiskTransferV2v"
	VMCreation      = "VirtualMachineCreation"
	Unknown         = "Unknown"
)

var (
	ColdItinerary = libitr.Itinerary{
		Name: "",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhasePreHook, All: HasPreHook},
			{Name: api.PhaseStorePowerState},
			{Name: api.PhasePowerOffSource},
			{Name: api.PhaseWaitForPowerOff},
			{Name: api.PhaseCreateDataVolumes},
			{Name: api.PhaseCopyDisks, All: CDIDiskCopy},
			{Name: api.PhaseAllocateDisks, All: VirtV2vDiskCopy},
			{Name: api.PhaseCreateGuestConversionPod, All: RequiresConversion},
			{Name: api.PhaseConvertGuest, All: RequiresConversion},
			{Name: api.PhaseCopyDisksVirtV2V, All: RequiresConversion},
			{Name: api.PhaseConvertOpenstackSnapshot, All: OpenstackImageMigration},
			{Name: api.PhaseCreateVM},
			{Name: api.PhasePostHook, All: HasPostHook},
			{Name: api.PhaseCompleted},
		},
	}
	WarmItinerary = libitr.Itinerary{
		Name: "Warm",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseStarted},
			{Name: api.PhasePreHook, All: HasPreHook},
			{Name: api.PhaseCreateInitialSnapshot},
			{Name: api.PhaseWaitForInitialSnapshot},
			{Name: api.PhaseStoreInitialSnapshotDeltas, All: VSphere},
			{Name: api.PhaseCreateDataVolumes},
			// Precopy loop start
			{Name: api.PhaseWaitForDataVolumesStatus},
			{Name: api.PhaseCopyDisks},
			{Name: api.PhaseCopyingPaused},
			{Name: api.PhaseRemovePreviousSnapshot, All: VSphere},
			{Name: api.PhaseWaitForPreviousSnapshotRemoval, All: VSphere},
			{Name: api.PhaseCreateSnapshot},
			{Name: api.PhaseWaitForSnapshot},
			{Name: api.PhaseStoreSnapshotDeltas, All: VSphere},
			{Name: api.PhaseAddCheckpoint},
			// Precopy loop end
			{Name: api.PhaseStorePowerState},
			{Name: api.PhasePowerOffSource},
			{Name: api.PhaseWaitForPowerOff},
			{Name: api.PhaseRemovePenultimateSnapshot, All: VSphere},
			{Name: api.PhaseWaitForPenultimateSnapshotRemoval, All: VSphere},
			{Name: api.PhaseCreateFinalSnapshot},
			{Name: api.PhaseWaitForFinalSnapshot},
			{Name: api.PhaseAddFinalCheckpoint},
			{Name: api.PhaseWaitForFinalDataVolumesStatus},
			{Name: api.PhaseFinalize},
			{Name: api.PhaseRemoveFinalSnapshot, All: VSphere},
			{Name: api.PhaseWaitForFinalSnapshotRemoval, All: VSphere},
			{Name: api.PhaseCreateGuestConversionPod, All: RequiresConversion},
			{Name: api.PhaseConvertGuest, All: RequiresConversion},
			{Name: api.PhaseCreateVM},
			{Name: api.PhasePostHook, All: HasPostHook},
			{Name: api.PhaseCompleted},
		},
	}
)

type Migrator interface {
	Init() error
	Status(plan.VM) *plan.VMStatus
	Reset(*plan.VMStatus, []*plan.Step)
	Pipeline(plan.VM) ([]*plan.Step, error)
	ExecutePhase(*plan.VMStatus) (bool, error)
	Step(*plan.VMStatus) string
	Next(status *plan.VMStatus) (next string)
	Cleanup(status *plan.VMStatus, successful bool) (err error)
}
