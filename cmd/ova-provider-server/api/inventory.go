package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/inventory"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/ova"
)

// Version response structures
type Version struct {
	Major    string `json:"major"`
	Minor    string `json:"minor"`
	Build    string `json:"build"`
	Revision string `json:"revision"`
}

type VersionResponse struct {
	Version Version `json:"version"`
}

// DataVolume source response structures
type HTTPSource struct {
	URL string `json:"url"`
}

type DataVolumeSource struct {
	HTTP *HTTPSource `json:"http,omitempty"`
}

type DataVolumeSpec struct {
	Source DataVolumeSource `json:"source"`
}

type DataVolumeSourceResponse struct {
	Spec DataVolumeSpec `json:"spec"`
}

const (
	VMsRoute            = "/vms"
	NetworksRoute       = "/networks"
	DisksRoute          = "/disks"
	StoragesRoute       = "/storages"
	TestConnectionRoute = "/test_connection"
	VersionRoute        = "/version"
)

// InventoryHandler serves routes consumed by the Forklift inventory service.
type InventoryHandler struct{}

// AddRoutes adds inventory routes to a gin router.
func (h InventoryHandler) AddRoutes(e *gin.Engine) {
	router := e.Group("/")
	router.GET(VMsRoute, h.VMs)
	router.GET(NetworksRoute, h.Networks)
	router.GET(DisksRoute, h.Disks)
	router.GET(StoragesRoute, h.Storages)
	router.GET(TestConnectionRoute, h.TestConnection)
	router.GET(VersionRoute, h.Version)
	router.POST("/vms/:vmId/disks/:diskId/datavolume-source", h.DataVolumeSource)
}

// VMs godoc
// @summary List all VMs structs that can be extracted from all OVAs/OVFs in the catalog.
// @description List all VMs structs that can be extracted from all OVAs/OVFs in the catalog.
// @tags inventory
// @produce json
// @success 200 {array} ova.VM
// @router /vms [get]
func (h InventoryHandler) VMs(ctx *gin.Context) {
	envelopes, paths := inventory.ScanForAppliances(Settings.CatalogPath)
	vms := inventory.ConvertToVmStruct(envelopes, paths)
	ctx.JSON(http.StatusOK, vms)
}

// Networks godoc
// @summary List all network structs that can be extracted from all OVAs/OVFs in the catalog.
// @description List all network structs that can be extracted from all OVAs/OVFs in the catalog.
// @tags inventory
// @produce json
// @success 200 {array} ova.VmNetwork
// @router /networks [get]
func (h InventoryHandler) Networks(ctx *gin.Context) {
	envelopes, _ := inventory.ScanForAppliances(Settings.CatalogPath)
	networks := inventory.ConvertToNetworkStruct(envelopes)
	ctx.JSON(http.StatusOK, networks)
}

// Disks godoc
// @summary List all disk structs that can be extracted from all OVAs/OVFs in the catalog.
// @description List all disk structs that can be extracted from all OVAs/OVFs in the catalog.
// @tags inventory
// @produce json
// @success 200 {array} ova.VmDisk
// @router /disks [get]
func (h InventoryHandler) Disks(ctx *gin.Context) {
	envelopes, paths := inventory.ScanForAppliances(Settings.CatalogPath)
	disks := inventory.ConvertToDiskStruct(envelopes, paths)
	ctx.JSON(http.StatusOK, disks)
}

func (h InventoryHandler) TestConnection(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, "Test connection successful")
}

// Storages godoc
// @summary List storage resources (NFS mounts)
// @description Returns storage information for the NFS mount containing OVA files.
// @tags inventory
// @produce json
// @success 200 {array} map[string]interface{}
// @router /storages [get]
func (h InventoryHandler) Storages(ctx *gin.Context) {
	// Return NFS storage information
	storage := map[string]interface{}{
		"id":       "nfs-storage",
		"name":     Settings.CatalogPath,
		"capacity": int64(0), // NFS doesn't report capacity easily
	}
	ctx.JSON(http.StatusOK, []interface{}{storage})
}

// Version godoc
// @summary Get provider server version information
// @description Returns version information for the OVA provider server.
// @tags inventory
// @produce json
// @success 200 {object} VersionResponse
// @router /version [get]
func (h InventoryHandler) Version(ctx *gin.Context) {
	version := VersionResponse{
		Version: Version{
			Major:    "2",
			Minor:    "7",
			Build:    "0",
			Revision: "latest",
		},
	}
	ctx.JSON(http.StatusOK, version)
}

// DataVolumeSource godoc
// @summary Get DataVolume source specification for a VM disk
// @description Returns CDI DataVolume source spec for migrating a disk from NFS.
// @tags inventory
// @produce json
// @param vmId path string true "VM ID"
// @param diskId path string true "Disk ID"
// @success 200 {object} DataVolumeSourceResponse
// @router /vms/{vmId}/disks/{diskId}/datavolume-source [post]
func (h InventoryHandler) DataVolumeSource(ctx *gin.Context) {
	vmId := ctx.Param("vmId")
	diskId := ctx.Param("diskId")

	// Get disk information
	envelopes, paths := inventory.ScanForAppliances(Settings.CatalogPath)
	vms := inventory.ConvertToVmStruct(envelopes, paths)

	// Find the VM and disk
	var targetDisk *ova.VmDisk
	var ovaPath string
	for _, vm := range vms {
		if vm.UUID == vmId {
			ovaPath = vm.OvaPath
			for i := range vm.Disks {
				if vm.Disks[i].ID == diskId {
					targetDisk = &vm.Disks[i]
					break
				}
			}
			break
		}
	}

	if targetDisk == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Disk not found"})
		return
	}

	// Return DataVolume source specification for NFS
	// The URL format should be: nfs://server/path/to/disk
	response := DataVolumeSourceResponse{
		Spec: DataVolumeSpec{
			Source: DataVolumeSource{
				HTTP: &HTTPSource{
					URL: "nfs://" + Settings.CatalogPath + "/" + ovaPath + "/" + targetDisk.FilePath,
				},
			},
		},
	}

	ctx.JSON(http.StatusOK, response)
}
