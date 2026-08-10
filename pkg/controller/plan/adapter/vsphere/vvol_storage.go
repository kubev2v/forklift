package vsphere

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
)

func resolveVVolDisksByDatastore(disks []vsphere.Disk, dsMap map[string]*api.StoragePair) map[int]*api.StoragePair {
	resolved := make(map[int]*api.StoragePair)
	for diskIndex, disk := range disks {
		if disk.VVolID == "" {
			continue
		}
		if entry, found := dsMap[disk.Datastore.ID]; found {
			resolved[diskIndex] = entry
		}
	}
	return resolved
}
