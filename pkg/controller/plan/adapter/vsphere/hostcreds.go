package vsphere

import (
	"slices"
	"sort"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	inventorymodel "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
)

// HostIDsForCopy returns the hosts used by the selected disks' offload mappings.
// An empty ID means a selected disk has no resolvable host.
func (r *Builder) HostIDsForCopy(vm *model.VM) ([]string, error) {
	if r.Map.Storage == nil {
		return nil, nil
	}
	// Apply disk exclusions without changing the inventory object.
	copyVM := *vm
	copyVM.Disks = slices.Clone(vm.Disks)
	if !r.shouldMigrateSharedDisks(&copyVM) {
		copyVM.RemoveSharedDisks()
	}
	r.removeExcludedDisks(&copyVM)
	disks := copyVM.SortedDisksAsVmware()
	mappings := r.Map.Storage.Spec.Map
	rdmMapped := r.rdmStorageMappings(mappings, disks, loadNAAPrefixes(r.Client))
	hosts := map[string]bool{}
	for i := range mappings {
		mapped := &mappings[i]
		if mapped.OffloadPlugin == nil || mapped.OffloadPlugin.VSphereXcopyPluginConfig == nil {
			continue
		}
		ds := &model.Datastore{}
		if err := r.Source.Inventory.Find(ds, mapped.Source); err != nil {
			return nil, err
		}
		for diskIndex, disk := range disks {
			if entry, found := rdmMapped[diskIndex]; found {
				if entry != mapped {
					continue
				}
			} else if disk.Datastore.ID != ds.ID {
				continue
			}
			dedicated := mapped.OffloadPlugin.VSphereXcopyPluginConfig.DedicatedMigrationHosts
			if len(dedicated) == 0 {
				hosts[vm.Host] = true
			} else {
				for _, host := range dedicated {
					if host == "" {
						host = vm.Host
					}
					hosts[host] = true
				}
			}
		}
	}
	ids := make([]string, 0, len(hosts))
	for id := range hosts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (r *Builder) rdmStorageMappings(dsMapIn []api.StoragePair, sortedDisks []inventorymodel.Disk, naaPrefixes []naaVendorEntry) map[int]*api.StoragePair {
	// Pre-pass: resolve RDM disks by NAA vendor prefix.
	// Builds a map from disk index to the storage map entry whose offload
	// plugin matches the RDM's vendor. Disks that fail NAA resolution are
	// left out and will fall back to datastore-based matching in the main loop.
	rdmMapped := make(map[int]*api.StoragePair)
	for diskIndex, disk := range sortedDisks {
		if !disk.RDM || disk.DeviceName == "" {
			continue
		}
		rdmVendor, matched := vendorFromNAA(disk.DeviceName, naaPrefixes)
		if !matched {
			r.Log.Info("RDM disk NAA prefix not recognized, falling back to datastore matching",
				"diskKey", disk.Key, "deviceName", disk.DeviceName)
			continue
		}
		candidates := findStorageMapEntriesForVendor(dsMapIn, rdmVendor)
		switch len(candidates) {
		case 0:
			r.Log.Info("No storage map entry found for RDM vendor, falling back to datastore matching",
				"diskKey", disk.Key, "vendor", rdmVendor)
		case 1:
			rdmMapped[diskIndex] = candidates[0]
			r.Log.Info("RDM disk resolved to storage vendor by NAA",
				"diskKey", disk.Key, "deviceName", disk.DeviceName,
				"vendor", rdmVendor, "storageClass", candidates[0].Destination.StorageClass)
		default:
			entry, disambigErr := disambiguateRDMByNAA(r.Source.Inventory, candidates, disk.DeviceName, naaPrefixes)
			if disambigErr != nil {
				r.Log.Info("RDM NAA disambiguation failed, falling back to datastore matching",
					"diskKey", disk.Key, "vendor", rdmVendor, "error", disambigErr)
			} else {
				rdmMapped[diskIndex] = entry
				r.Log.Info("RDM disk resolved to storage vendor by NAA",
					"diskKey", disk.Key, "deviceName", disk.DeviceName,
					"vendor", rdmVendor, "storageClass", entry.Destination.StorageClass)
			}
		}
	}

	return rdmMapped
}
