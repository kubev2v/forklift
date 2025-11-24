package dynamic

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Cache-supported resources that should be served from SQLite instead of proxied
var cachedResources = map[string]bool{
	"/vms":      true,
	"/networks": true,
	"/storages": true,
	"/disks":    true,
}

// isCachedResource checks if the path should be served from cache
func isCachedResource(path string) bool {
	return cachedResources[path]
}

// serveFromCache responds with cached data and triggers background refresh
func (h *Handler) serveFromCache(ctx *gin.Context, providerUID, path string) {
	// Get collector for this provider
	collector := h.getCollector(providerUID)
	if collector == nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	// Get the database
	db := collector.DB()
	if db == nil {
		// No cache available, fall through to proxy
		log.V(3).Info("No cache available, will proxy",
			"provider", collector.Owner().GetName(),
			"path", path)
		h.proxyRequest(ctx, providerUID, path)
		return
	}

	// Route to appropriate cache handler
	var err error
	switch path {
	case "/vms":
		err = h.serveVMsFromCache(ctx, db)
	case "/networks":
		err = h.serveNetworksFromCache(ctx, db)
	case "/storages":
		err = h.serveStorageFromCache(ctx, db)
	case "/disks":
		err = h.serveDisksFromCache(ctx, db)
	default:
		// Should not happen due to isCachedResource check
		ctx.Status(http.StatusNotFound)
		return
	}

	if err != nil {
		log.Error(err, "Failed to serve from cache",
			"provider", collector.Owner().GetName(),
			"path", path)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	// Trigger background refresh asynchronously
	// This ensures the cache is updated for the next request
	go func() {
		log.V(3).Info("Triggering background refresh after cache response",
			"provider", collector.Owner().GetName(),
			"path", path)
		collector.Reset()
	}()
}

// serveVMsFromCache serves VMs from SQLite cache
func (h *Handler) serveVMsFromCache(ctx *gin.Context, db libmodel.DB) error {
	vms := []dynamic.VM{}

	// Build list options with query parameter filters
	opts := libmodel.ListOptions{}

	// Build predicates for filtering
	var predicates []libmodel.Predicate

	// Filter by ID if specified
	if id := ctx.Query("id"); id != "" {
		predicates = append(predicates, libmodel.Eq("ID", id))
	}

	// Filter by name if specified
	if name := ctx.Query("name"); name != "" {
		predicates = append(predicates, libmodel.Eq("Name", name))
	}

	// Combine predicates with AND
	if len(predicates) > 0 {
		if len(predicates) == 1 {
			opts.Predicate = predicates[0]
		} else {
			opts.Predicate = libmodel.And(predicates...)
		}
	}

	err := db.List(&vms, opts)
	if err != nil {
		return err
	}

	// Convert model structs to raw JSON objects for REST API response
	objects := make([]map[string]interface{}, 0, len(vms))
	for _, vm := range vms {
		obj, err := vm.GetObject()
		if err != nil {
			log.V(1).Info("Failed to get object for VM, skipping",
				"id", vm.ID,
				"name", vm.Name,
				"error", err)
			continue
		}
		objects = append(objects, obj)
	}

	ctx.JSON(http.StatusOK, objects)
	return nil
}

// serveNetworksFromCache serves networks from SQLite cache
func (h *Handler) serveNetworksFromCache(ctx *gin.Context, db libmodel.DB) error {
	networks := []dynamic.Network{}

	// Build list options with query parameter filters
	opts := libmodel.ListOptions{}

	// Build predicates for filtering
	var predicates []libmodel.Predicate

	// Filter by ID if specified
	if id := ctx.Query("id"); id != "" {
		predicates = append(predicates, libmodel.Eq("ID", id))
	}

	// Filter by name if specified
	if name := ctx.Query("name"); name != "" {
		predicates = append(predicates, libmodel.Eq("Name", name))
	}

	// Combine predicates with AND
	if len(predicates) > 0 {
		if len(predicates) == 1 {
			opts.Predicate = predicates[0]
		} else {
			opts.Predicate = libmodel.And(predicates...)
		}
	}

	err := db.List(&networks, opts)
	if err != nil {
		return err
	}

	// Convert model structs to raw JSON objects for REST API response
	objects := make([]map[string]interface{}, 0, len(networks))
	for _, network := range networks {
		obj, err := network.GetObject()
		if err != nil {
			log.V(1).Info("Failed to get object for Network, skipping",
				"id", network.ID,
				"name", network.Name,
				"error", err)
			continue
		}
		objects = append(objects, obj)
	}

	ctx.JSON(http.StatusOK, objects)
	return nil
}

// serveStorageFromCache serves storage from SQLite cache
func (h *Handler) serveStorageFromCache(ctx *gin.Context, db libmodel.DB) error {
	storages := []dynamic.Storage{}

	// Build list options with query parameter filters
	opts := libmodel.ListOptions{}

	// Build predicates for filtering
	var predicates []libmodel.Predicate

	// Filter by ID if specified
	if id := ctx.Query("id"); id != "" {
		predicates = append(predicates, libmodel.Eq("ID", id))
	}

	// Filter by name if specified
	if name := ctx.Query("name"); name != "" {
		predicates = append(predicates, libmodel.Eq("Name", name))
	}

	// Combine predicates with AND
	if len(predicates) > 0 {
		if len(predicates) == 1 {
			opts.Predicate = predicates[0]
		} else {
			opts.Predicate = libmodel.And(predicates...)
		}
	}

	err := db.List(&storages, opts)
	if err != nil {
		return err
	}

	// Convert model structs to raw JSON objects for REST API response
	objects := make([]map[string]interface{}, 0, len(storages))
	for _, storage := range storages {
		obj, err := storage.GetObject()
		if err != nil {
			log.V(1).Info("Failed to get object for Storage, skipping",
				"id", storage.ID,
				"name", storage.Name,
				"error", err)
			continue
		}
		objects = append(objects, obj)
	}

	ctx.JSON(http.StatusOK, objects)
	return nil
}

// serveDisksFromCache serves disks from SQLite cache
func (h *Handler) serveDisksFromCache(ctx *gin.Context, db libmodel.DB) error {
	disks := []dynamic.Disk{}

	// Build list options with query parameter filters
	opts := libmodel.ListOptions{}

	// Build predicates for filtering
	var predicates []libmodel.Predicate

	// Filter by ID if specified
	if id := ctx.Query("id"); id != "" {
		predicates = append(predicates, libmodel.Eq("ID", id))
	}

	// Filter by name if specified
	if name := ctx.Query("name"); name != "" {
		predicates = append(predicates, libmodel.Eq("Name", name))
	}

	// Combine predicates with AND
	if len(predicates) > 0 {
		if len(predicates) == 1 {
			opts.Predicate = predicates[0]
		} else {
			opts.Predicate = libmodel.And(predicates...)
		}
	}

	err := db.List(&disks, opts)
	if err != nil {
		return err
	}

	// Convert model structs to raw JSON objects for REST API response
	objects := make([]map[string]interface{}, 0, len(disks))
	for _, disk := range disks {
		obj, err := disk.GetObject()
		if err != nil {
			log.V(1).Info("Failed to get object for Disk, skipping",
				"id", disk.ID,
				"name", disk.Name,
				"error", err)
			continue
		}
		objects = append(objects, obj)
	}

	ctx.JSON(http.StatusOK, objects)
	return nil
}

// getCollector retrieves a collector by provider UID
func (h *Handler) getCollector(providerUID string) *Collector {
	collectors := h.Container.List()
	for _, collector := range collectors {
		if string(collector.Owner().GetUID()) == providerUID {
			if c, ok := collector.(*Collector); ok {
				return c
			}
		}
	}
	return nil
}
