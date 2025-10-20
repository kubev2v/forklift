package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/inventory"
)

const (
	VMsRoute            = "/vms"
	NetworksRoute       = "/networks"
	DisksRoute          = "/disks"
	TestConnectionRoute = "/test_connection"
)

// InventoryHandler serves routes consumed by the Forklift inventory service.
type InventoryHandler struct{}

// AddRoutes adds inventory routes to a gin router.
func (h InventoryHandler) AddRoutes(e *gin.Engine) {
	router := e.Group("/")
	router.GET(VMsRoute, h.VMs)
	router.GET(NetworksRoute, h.Networks)
	router.GET(DisksRoute, h.Disks)
	router.GET(TestConnectionRoute, h.TestConnection)
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
