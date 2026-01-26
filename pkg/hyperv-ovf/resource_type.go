package hypervovf

type ResourceType int

const (
	ResourceTypeProcessor       ResourceType = 3
	ResourceTypeMemory          ResourceType = 4
	ResourceTypeIDEController   ResourceType = 5
	ResourceTypeSCSIController  ResourceType = 6
	ResourceTypeEthernetAdapter ResourceType = 10
	ResourceTypeHardDisk        ResourceType = 17
)

func (r ResourceType) String() string {
	switch r {
	case ResourceTypeProcessor:
		return "Processor"
	case ResourceTypeMemory:
		return "Memory"
	case ResourceTypeIDEController:
		return "IDE Controller"
	case ResourceTypeSCSIController:
		return "SCSI Controller"
	case ResourceTypeEthernetAdapter:
		return "Ethernet Adapter"
	case ResourceTypeHardDisk:
		return "Hard Disk"
	default:
		return "Other"
	}
}
