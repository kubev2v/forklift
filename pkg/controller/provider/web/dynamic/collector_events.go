package dynamic

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// VMEventHandler implements libmodel.EventHandler for VM model events.
// It logs VM lifecycle events (created, updated, deleted) for observability.
type VMEventHandler struct {
	Provider *api.Provider
	log      logging.LevelLogger
}

func (h *VMEventHandler) Options() libmodel.WatchOptions {
	return libmodel.WatchOptions{}
}

func (h *VMEventHandler) Started(watchID uint64) {
	h.log.V(3).Info("VM watch started",
		"provider", h.Provider.Name,
		"watchID", watchID)
}

func (h *VMEventHandler) Parity() {
	h.log.V(3).Info("VM watch parity reached",
		"provider", h.Provider.Name)
}

func (h *VMEventHandler) Updated(e libmodel.Event) {
	vm := e.Model.(*dynamic.VM)
	h.log.V(3).Info("VM updated",
		"provider", h.Provider.Name,
		"vm", vm.Name,
		"id", vm.ID)
}

func (h *VMEventHandler) Created(e libmodel.Event) {
	vm := e.Model.(*dynamic.VM)
	h.log.Info("VM created",
		"provider", h.Provider.Name,
		"vm", vm.Name,
		"id", vm.ID)
}

func (h *VMEventHandler) Deleted(e libmodel.Event) {
	vm := e.Model.(*dynamic.VM)
	h.log.Info("VM deleted",
		"provider", h.Provider.Name,
		"vm", vm.Name,
		"id", vm.ID)
}

func (h *VMEventHandler) Error(err error) {
	h.log.Error(err, "VM watch error",
		"provider", h.Provider.Name)
}

func (h *VMEventHandler) End() {
	h.log.V(3).Info("VM watch ended",
		"provider", h.Provider.Name)
}

// NetworkEventHandler implements libmodel.EventHandler for Network model events.
// It logs Network lifecycle events (created, updated, deleted) for observability.
type NetworkEventHandler struct {
	Provider *api.Provider
	log      logging.LevelLogger
}

func (h *NetworkEventHandler) Options() libmodel.WatchOptions {
	return libmodel.WatchOptions{}
}

func (h *NetworkEventHandler) Started(watchID uint64) {
	h.log.V(3).Info("Network watch started",
		"provider", h.Provider.Name,
		"watchID", watchID)
}

func (h *NetworkEventHandler) Parity() {
	h.log.V(3).Info("Network watch parity reached",
		"provider", h.Provider.Name)
}

func (h *NetworkEventHandler) Updated(e libmodel.Event) {
	network := e.Model.(*dynamic.Network)
	h.log.V(3).Info("Network updated",
		"provider", h.Provider.Name,
		"network", network.Name,
		"id", network.ID)
}

func (h *NetworkEventHandler) Created(e libmodel.Event) {
	network := e.Model.(*dynamic.Network)
	h.log.Info("Network created",
		"provider", h.Provider.Name,
		"network", network.Name,
		"id", network.ID)
}

func (h *NetworkEventHandler) Deleted(e libmodel.Event) {
	network := e.Model.(*dynamic.Network)
	h.log.Info("Network deleted",
		"provider", h.Provider.Name,
		"network", network.Name,
		"id", network.ID)
}

func (h *NetworkEventHandler) Error(err error) {
	h.log.Error(err, "Network watch error",
		"provider", h.Provider.Name)
}

func (h *NetworkEventHandler) End() {
	h.log.V(3).Info("Network watch ended",
		"provider", h.Provider.Name)
}

// StorageEventHandler implements libmodel.EventHandler for Storage model events.
// It logs Storage lifecycle events (created, updated, deleted) for observability.
type StorageEventHandler struct {
	Provider *api.Provider
	log      logging.LevelLogger
}

func (h *StorageEventHandler) Options() libmodel.WatchOptions {
	return libmodel.WatchOptions{}
}

func (h *StorageEventHandler) Started(watchID uint64) {
	h.log.V(3).Info("Storage watch started",
		"provider", h.Provider.Name,
		"watchID", watchID)
}

func (h *StorageEventHandler) Parity() {
	h.log.V(3).Info("Storage watch parity reached",
		"provider", h.Provider.Name)
}

func (h *StorageEventHandler) Updated(e libmodel.Event) {
	storage := e.Model.(*dynamic.Storage)
	h.log.V(3).Info("Storage updated",
		"provider", h.Provider.Name,
		"storage", storage.Name,
		"id", storage.ID)
}

func (h *StorageEventHandler) Created(e libmodel.Event) {
	storage := e.Model.(*dynamic.Storage)
	h.log.Info("Storage created",
		"provider", h.Provider.Name,
		"storage", storage.Name,
		"id", storage.ID)
}

func (h *StorageEventHandler) Deleted(e libmodel.Event) {
	storage := e.Model.(*dynamic.Storage)
	h.log.Info("Storage deleted",
		"provider", h.Provider.Name,
		"storage", storage.Name,
		"id", storage.ID)
}

func (h *StorageEventHandler) Error(err error) {
	h.log.Error(err, "Storage watch error",
		"provider", h.Provider.Name)
}

func (h *StorageEventHandler) End() {
	h.log.V(3).Info("Storage watch ended",
		"provider", h.Provider.Name)
}

// DiskEventHandler implements libmodel.EventHandler for Disk model events.
// It logs Disk lifecycle events (created, updated, deleted) for observability.
type DiskEventHandler struct {
	Provider *api.Provider
	log      logging.LevelLogger
}

func (h *DiskEventHandler) Options() libmodel.WatchOptions {
	return libmodel.WatchOptions{}
}

func (h *DiskEventHandler) Started(watchID uint64) {
	h.log.V(3).Info("Disk watch started",
		"provider", h.Provider.Name,
		"watchID", watchID)
}

func (h *DiskEventHandler) Parity() {
	h.log.V(3).Info("Disk watch parity reached",
		"provider", h.Provider.Name)
}

func (h *DiskEventHandler) Updated(e libmodel.Event) {
	disk := e.Model.(*dynamic.Disk)
	h.log.V(3).Info("Disk updated",
		"provider", h.Provider.Name,
		"disk", disk.Name,
		"id", disk.ID)
}

func (h *DiskEventHandler) Created(e libmodel.Event) {
	disk := e.Model.(*dynamic.Disk)
	h.log.Info("Disk created",
		"provider", h.Provider.Name,
		"disk", disk.Name,
		"id", disk.ID)
}

func (h *DiskEventHandler) Deleted(e libmodel.Event) {
	disk := e.Model.(*dynamic.Disk)
	h.log.Info("Disk deleted",
		"provider", h.Provider.Name,
		"disk", disk.Name,
		"id", disk.ID)
}

func (h *DiskEventHandler) Error(err error) {
	h.log.Error(err, "Disk watch error",
		"provider", h.Provider.Name)
}

func (h *DiskEventHandler) End() {
	h.log.V(3).Info("Disk watch ended",
		"provider", h.Provider.Name)
}
