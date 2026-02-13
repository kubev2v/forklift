package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/cmd/hyperv-provider-server/collector"
)

// InventoryHandler serves inventory routes for the HyperV provider.
type InventoryHandler struct {
	Collector *collector.Collector
}

// AddRoutes adds inventory routes to a gin router.
func (h *InventoryHandler) AddRoutes(e *gin.Engine) {
	e.GET("/vms", h.VMs)
	e.GET("/networks", h.Networks)
	e.GET("/storages", h.Storages)
	e.GET("/disks", h.Disks)
	e.GET("/test_connection", h.TestConnection)
}

func (h *InventoryHandler) parityRequired(ctx *gin.Context) bool {
	if !h.Collector.HasParity() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "inventory not yet available, initial sync in progress",
		})
		return false
	}
	return true
}

// VMs returns all VMs from the HyperV host.
// @summary List all VMs from the HyperV host.
// @description Returns cached VM inventory collected from HyperV host.
// @tags inventory
// @produce json
// @success 200 {array} collector.VM
// @failure 503 {object} map[string]string "Initial sync not complete"
// @router /vms [get]
func (h *InventoryHandler) VMs(ctx *gin.Context) {
	if !h.parityRequired(ctx) {
		return
	}
	vms := h.Collector.GetVMs()
	ctx.JSON(http.StatusOK, vms)
}

// Networks returns all networks from the HyperV host.
// @summary List all networks from the HyperV host.
// @description Returns cached network inventory collected from HyperV host.
// @tags inventory
// @produce json
// @success 200 {array} collector.Network
// @failure 503 {object} map[string]string "Initial sync not complete"
// @router /networks [get]
func (h *InventoryHandler) Networks(ctx *gin.Context) {
	if !h.parityRequired(ctx) {
		return
	}
	networks := h.Collector.GetNetworks()
	ctx.JSON(http.StatusOK, networks)
}

// Storages returns all storage locations from the HyperV host.
// @summary List all storage locations from the HyperV host.
// @description Returns cached storage inventory extracted from VM disk paths.
// @tags inventory
// @produce json
// @success 200 {array} collector.Storage
// @failure 503 {object} map[string]string "Initial sync not complete"
// @router /storages [get]
func (h *InventoryHandler) Storages(ctx *gin.Context) {
	if !h.parityRequired(ctx) {
		return
	}
	storages := h.Collector.GetStorages()
	ctx.JSON(http.StatusOK, storages)
}

// Disks returns all disks from all VMs.
// @summary List all disks from all VMs.
// @description Returns all disks extracted from VM inventory.
// @tags inventory
// @produce json
// @success 200 {array} collector.Disk
// @failure 503 {object} map[string]string "Initial sync not complete"
// @router /disks [get]
func (h *InventoryHandler) Disks(ctx *gin.Context) {
	if !h.parityRequired(ctx) {
		return
	}
	disks := h.Collector.GetDisks()
	ctx.JSON(http.StatusOK, disks)
}

// TestConnection tests the connection to the HyperV host.
// @summary Test the connection to the HyperV provider.
// @description Performs a live connectivity check (not cached).
// @tags inventory
// @produce json
// @success 200 {object} ConnectionStatus
// @router /test_connection [get]
func (h *InventoryHandler) TestConnection(ctx *gin.Context) {
	status := ConnectionStatus{
		Connected: h.Collector.TestConnection(),
	}
	ctx.JSON(http.StatusOK, status)
}

// ConnectionStatus represents the connection status response.
type ConnectionStatus struct {
	Connected bool `json:"connected"`
}
