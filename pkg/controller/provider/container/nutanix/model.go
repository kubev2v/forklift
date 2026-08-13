package nutanix

import model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"

func applyCluster(entity map[string]interface{}, m *model.Cluster) {
	var e clusterEntity
	if err := decodeEntity(entity, &e); err != nil {
		return
	}
	e.ApplyTo(m)
}

func applyHost(entity map[string]interface{}, m *model.Host) {
	var e hostEntity
	if err := decodeEntity(entity, &e); err != nil {
		return
	}
	e.ApplyTo(m)
}

func applyNetwork(entity map[string]interface{}, m *model.Network) {
	var e networkEntity
	if err := decodeEntity(entity, &e); err != nil {
		return
	}
	e.ApplyTo(m)
}

func applyStorageContainer(entity map[string]interface{}, m *model.StorageContainer) {
	var e storageContainerEntity
	if err := decodeEntity(entity, &e); err != nil {
		return
	}
	e.ApplyTo(m)
}

func applyImage(entity map[string]interface{}, m *model.Image) {
	var e imageEntity
	if err := decodeEntity(entity, &e); err != nil {
		return
	}
	e.ApplyTo(m)
}

func applyVM(entity map[string]interface{}, m *model.VM) {
	var e vmEntity
	if err := decodeEntity(entity, &e); err != nil {
		return
	}
	e.ApplyTo(m)
}

func applyGuestTools(specResources, statusResources map[string]interface{}, m *model.VM) {
	var spec vmResources
	var status vmResources
	_ = decodeEntity(specResources, &spec)
	_ = decodeEntity(statusResources, &status)
	mergeGuestTools(spec.GuestTools, status.GuestTools, m)
}

func applyDiskFromMap(diskData map[string]interface{}) model.Disk {
	var disk diskEntity
	if err := decodeEntity(diskData, &disk); err != nil {
		return model.Disk{}
	}
	return disk.ApplyTo()
}

func applyStorageContainerRef(data map[string]interface{}, disk *model.Disk) {
	var container ref
	if storageConfig, ok := data["storage_config"].(map[string]interface{}); ok {
		if scRef, ok := storageConfig["storage_container_reference"].(map[string]interface{}); ok {
			_ = decodeEntity(scRef, &container)
		}
	}
	if container.UUID == "" {
		if scRef, ok := data["storage_container_reference"].(map[string]interface{}); ok {
			_ = decodeEntity(scRef, &container)
		}
	}
	disk.StorageContainerUUID = container.UUID
	disk.StorageContainerName = container.Name
}
