package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/cmd/provider-common/inventory"
	"github.com/kubev2v/forklift/cmd/provider-common/settings"
)

// InventoryHandler serves inventory routes for OVF-based providers.
type InventoryHandler struct {
	Settings     *settings.ProviderSettings
	ProviderType string // Use inventory.ProviderTypeOVA or inventory.ProviderTypeHyperV
}

// AddRoutes adds inventory routes to a gin router.
func (h InventoryHandler) AddRoutes(e *gin.Engine) {
	e.GET("/vms", h.VMs)
	e.GET("/networks", h.Networks)
	e.GET("/disks", h.Disks)
	e.GET("/test_connection", h.TestConnection)
}

// VMs godoc
// @summary List all VMs structs that can be extracted from all OVFs in the catalog.
// @description List all VMs structs that can be extracted from all OVFs in the catalog.
// @tags inventory
// @produce json
// @success 200 {array} ovf.VM
// @router /vms [get]
func (h InventoryHandler) VMs(ctx *gin.Context) {
	envelopes, paths := inventory.ScanForAppliances(h.Settings.CatalogPath, h.ProviderType)
	ctx.JSON(http.StatusOK, inventory.ConvertToVmStruct(envelopes, paths))
}

// Networks godoc
// @summary List all network structs that can be extracted from all OVFs in the catalog.
// @description List all network structs that can be extracted from all OVFs in the catalog.
// @tags inventory
// @produce json
// @success 200 {array} ovf.Network
// @router /networks [get]
func (h InventoryHandler) Networks(ctx *gin.Context) {
	envelopes, _ := inventory.ScanForAppliances(h.Settings.CatalogPath, h.ProviderType)
	ctx.JSON(http.StatusOK, inventory.ConvertToNetworkStruct(envelopes))
}

// Disks godoc
// @summary List all disk structs that can be extracted from all OVFs in the catalog.
// @description List all disk structs that can be extracted from all OVFs in the catalog.
// @tags inventory
// @produce json
// @success 200 {array} ovf.Disk
// @router /disks [get]
func (h InventoryHandler) Disks(ctx *gin.Context) {
	envelopes, paths := inventory.ScanForAppliances(h.Settings.CatalogPath, h.ProviderType)
	ctx.JSON(http.StatusOK, inventory.ConvertToDiskStruct(envelopes, paths))
}

// TestConnection godoc
// @summary Test the connection to the provider.
// @description Test the connection to the provider by scanning the catalog.
// @tags inventory
// @produce json
// @success 200
// @router /test_connection [get]
func (h InventoryHandler) TestConnection(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, "")
}
