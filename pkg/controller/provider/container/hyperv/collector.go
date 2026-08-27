package hyperv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	liburl "net/url"
	"os"
	libpath "path"
	"sort"
	"sync"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	fb "github.com/kubev2v/forklift/pkg/lib/filebacked"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Settings
const (
	// Retry interval.
	RetryInterval = 5 * time.Second
	// Default refresh interval.
	DefaultRefreshInterval = 10 * time.Second
	// Default timeout for HTTP calls to the provider-server sidecar.
	DefaultValidationTimeout = 30 * time.Second
	// Env var to override refresh interval.
	EnvRefreshInterval = "HYPERV_REFRESH_INTERVAL"
	// Env var to override the SMB disk validation HTTP timeout.
	EnvValidationTimeout = "HYPERV_VALIDATION_TIMEOUT"
)

var RefreshInterval = DefaultRefreshInterval
var ValidationTimeout = DefaultValidationTimeout

func init() {
	if s := os.Getenv(EnvRefreshInterval); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			RefreshInterval = d
		}
	}
	if s := os.Getenv(EnvValidationTimeout); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			ValidationTimeout = d
		}
	}
}

// Phases
const (
	Started = ""
	Load    = "load"
	Loaded  = "loaded"
	Parity  = "parity"
	Refresh = "refresh"
)

// SortNICsByGuestNetworkOrder reorders vm.NICs to match the MAC address order of vm.GuestNetworks.
func SortNICsByGuestNetworkOrder(vm *model.VM) {
	macToDeviceIndex := make(map[string]int)
	for _, gn := range vm.GuestNetworks {
		if _, exists := macToDeviceIndex[gn.MAC]; !exists {
			macToDeviceIndex[gn.MAC] = gn.DeviceIndex
		}
	}

	sort.SliceStable(vm.NICs, func(i, j int) bool {
		iIdx, iOk := macToDeviceIndex[vm.NICs[i].MAC]
		jIdx, jOk := macToDeviceIndex[vm.NICs[j].MAC]

		switch {
		case iOk && jOk:
			return iIdx < jIdx
		case iOk:
			return true
		case jOk:
			return false
		default:
			return vm.NICs[i].DeviceIndex < vm.NICs[j].DeviceIndex
		}
	})
}

// HyperV data collector.
type Collector struct {
	// Provider
	provider *api.Provider
	// Provider secret
	secret *core.Secret
	// DB client.
	db libmodel.DB
	// Logger.
	log logging.LevelLogger
	// has parity.
	parity bool
	// WinRM client.
	client *Client
	// cancel function.
	cancel func()
	// Start time.
	startTime time.Time
	// Phase
	phase string
	// List of watches.
	watches []*libmodel.Watch
	// stateMu guards parity and connTestFailures, which are read/written
	// by the provider reconciler goroutine (ConnTestFailed/ConnTestSucceeded)
	// and the collector's own run goroutine.
	stateMu sync.Mutex
	// firstRefreshDone is true after the first non-LightMode refresh
	// has completed, which fills in disk capacity/RCT data deferred
	// from the LightMode initial load.
	firstRefreshDone bool
	// refreshCount tracks cycles since the last full disk enrichment.
	refreshCount int
	// connTestFailures counts consecutive connection test failures since
	// the last success. Used to limit the transient-failure grace period
	// so that persistent failures (credential revocation, network loss)
	// are surfaced after a bounded number of attempts.
	connTestFailures int
}

// New collector.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) *Collector {
	log := logging.WithName("collector|hyperv").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))
	clientLog := logging.WithName("client|hyperv").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))

	return &Collector{
		client: &Client{
			Secret: secret,
			Log:    clientLog,
		},
		provider: provider,
		secret:   secret,
		db:       db,
		log:      log,
	}
}

// The name.
func (r *Collector) Name() string {
	url, err := liburl.Parse(r.provider.Spec.URL)
	if err == nil {
		return url.Host
	}
	return r.provider.Spec.URL
}

// The owner.
func (r *Collector) Owner() meta.Object {
	return r.provider
}

// Get the DB.
func (r *Collector) DB() libmodel.DB {
	return r.db
}

// Reset.
func (r *Collector) Reset() {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.parity = false
	r.firstRefreshDone = false
	r.refreshCount = 0
	r.connTestFailures = 0
}

// HasParity.
func (r *Collector) HasParity() bool {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.parity
}

// maxConnTestFailures caps how many consecutive WinRM connection-test
// failures are tolerated before the provider is marked NotReady.
const maxConnTestFailures = 3

// ConnTestFailed records a failed connection test and returns true if the
// failure can be suppressed (collector has parity and fewer than
// maxConnTestFailures consecutive failures).
func (r *Collector) ConnTestFailed() bool {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.connTestFailures++
	return r.parity && r.connTestFailures <= maxConnTestFailures
}

// ConnTestSucceeded resets the consecutive failure counter.
func (r *Collector) ConnTestSucceeded() {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.connTestFailures = 0
}

// Test validates connectivity and credentials against the Hyper-V host.
func (r *Collector) Test() (status int, err error) {
	if err = r.client.Connect(r.provider); err != nil {
		if errors.Is(err, driver.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		return
	}
	if _, err = r.client.driver.IsAlive(); err != nil {
		if errors.Is(err, driver.ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		return
	}
	if r.provider.IsHyperVCluster() {
		if err = r.validateClusterMembership(); err != nil {
			return
		}
		r.log.Info("Connected to Hyper-V Failover Cluster via WinRM/HTTPS.")
	} else {
		r.log.Info("Connected to Hyper-V host via WinRM/HTTPS.")
	}
	return
}

// validateClusterMembership verifies the connected host is a Failover Cluster member.
func (r *Collector) validateClusterMembership() error {
	_, err := r.client.driver.GetCluster()
	if err != nil {
		return fmt.Errorf(
			"managementType is 'cluster' but Get-Cluster failed on this host — "+
				"verify the host is a Failover Cluster member: %w", err)
	}
	return nil
}

// NO-OP
func (r *Collector) Version() (_, _, _, _ string, err error) {
	return
}

// Follow link
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	return fmt.Errorf("not implemented")
}

// Start the collector.
func (r *Collector) Start() error {
	ctx := Context{
		client: r.client,
		db:     r.db,
		log:    r.log,
	}
	ctx.ctx, r.cancel = context.WithCancel(context.Background())
	start := func() {
		defer func() {
			r.endWatch()
			r.log.Info("Stopped.")
		}()
		for {
			if ctx.canceled() {
				return
			}
			if err := r.run(&ctx); err != nil {
				r.log.Error(err, "Run failed.", "retry", RetryInterval)
				select {
				case <-ctx.ctx.Done():
					return
				case <-time.After(RetryInterval):
				}
			}
		}
	}

	go start()

	return nil
}

// Shutdown the collector.
func (r *Collector) Shutdown() {
	r.log.Info("Shutdown.")
	if r.cancel != nil {
		r.cancel()
	}
}

// Run the current phase.
func (r *Collector) run(ctx *Context) (err error) {
	r.log.V(1).Info("Run started.")
	r.startTime = time.Now()
	r.phase = Started

	// Connect directly to HyperV host via WinRM using Secret credentials
	err = r.client.Connect(r.provider)
	if err != nil {
		return
	}

	// Phase 1: Fast initial load with LightMode (skips expensive Get-VHD).
	r.client.LightMode = true
	err = r.load(ctx)
	r.client.LightMode = false
	if err != nil {
		return
	}

	// Phase 2: Targeted disk capacity enrichment — runs only Get-VHD per
	// node, reusing the VM cache from Phase 1.
	enrichMark := time.Now()
	enrichOK := true
	if err = r.client.EnrichDiskCapacity(); err != nil {
		r.log.Error(err, "Disk capacity enrichment failed, will retry in refresh")
		enrichOK = false
	} else {
		if err = r.updateDisksFromCache(ctx); err != nil {
			r.log.Error(err, "Failed to update disk capacity in DB")
			enrichOK = false
		} else {
			r.log.Info("Disk capacity enriched.", "duration", time.Since(enrichMark))
		}
	}
	r.stateMu.Lock()
	r.firstRefreshDone = enrichOK
	r.parity = true
	r.stateMu.Unlock()
	r.phase = Parity
	if r.provider.IsHyperVCluster() {
		r.log.Info("Initial inventory loaded (cluster mode).",
			"clusters", r.clusterCount(),
			"hosts", r.hostCount(),
			"vms", r.vmCount(),
			"networks", r.networkCount(),
			"storages", r.storageCount(),
			"disks", r.diskCount())
	} else {
		r.log.Info("Initial inventory loaded.",
			"vms", r.vmCount(),
			"networks", r.networkCount(),
			"storages", r.storageCount(),
			"disks", r.diskCount())
	}

	// Start periodic refresh — all subsequent cycles use the optimized
	// path for cluster mode (LightMode, VM-first).
	r.beginWatch()
	for {
		select {
		case <-time.After(RefreshInterval):
			err = r.refresh(ctx)
			if err != nil {
				r.log.Error(err, "Refresh failed.")
			}
		case <-ctx.ctx.Done():
			return nil
		}
	}
}

// adapters returns the full adapter list, prepending cluster adapters if in cluster mode.
func (r *Collector) adapters() []Adapter {
	if r.provider.IsHyperVCluster() {
		return append(clusterAdapterList, adapterList...)
	}
	return adapterList
}

// Load the inventory.
// In cluster mode, adapters are grouped into phases to maximize parallelism:
//
//	Phase 1 (serial):   ClusterAdapter — populates the shared cluster cache
//	Phase 2 (parallel): HostAdapter + NetworkAdapter + StorageAdapter
//	Phase 3 (serial):   DiskAdapter — needs cached networks from Phase 2
//	Phase 4 (serial):   VMAdapter — needs cached VMs from Phase 3
func (r *Collector) load(ctx *Context) (err error) {
	r.phase = Load
	r.client.InvalidateCycleCache()
	mark := time.Now()

	if r.provider.IsHyperVCluster() {
		err = r.loadClusterParallel(ctx)
	} else {
		err = r.loadSerial(ctx)
	}
	if err != nil {
		return
	}

	r.phase = Loaded
	r.log.Info(
		"Initial Parity.",
		"duration",
		time.Since(mark))
	return
}

// loadSerial runs adapters one by one (standalone host mode).
func (r *Collector) loadSerial(ctx *Context) error {
	for _, adapter := range r.adapters() {
		if ctx.canceled() {
			return nil
		}
		adapterMark := time.Now()
		if err := r.create(ctx, adapter); err != nil {
			return err
		}
		r.log.Info("Adapter loaded.",
			"adapter", fmt.Sprintf("%T", adapter),
			"duration", time.Since(adapterMark).Seconds())
	}
	return nil
}

// loadClusterParallel runs cluster-mode adapters in parallel phases.
func (r *Collector) loadClusterParallel(ctx *Context) error {
	// Pre-fetch local network switches in the background. This WinRM call
	// doesn't need the cluster cache, so it overlaps with Phase 0.
	r.client.StartNetworkPrefetch()

	// Phase 0: Pre-warm the cluster cache. This is a single fast WinRM call
	// that all subsequent phases depend on. By doing it here (not inside the
	// ClusterAdapter), we can start the VM prefetch immediately afterward —
	// overlapping it with the ClusterAdapter's DB insert and Phase 2.
	if _, err := r.client.PrewarmClusterCache(); err != nil {
		return err
	}

	// Start VM pre-fetch in the background immediately — the cluster cache is
	// ready so it knows which nodes to query. This overlaps with ALL remaining
	// phases (ClusterAdapter DB insert + Phase 2 adapters).
	r.client.StartVMPrefetch()

	// Phase 1: ClusterAdapter — reads the already-cached cluster data and
	// inserts it into the DB (fast, no WinRM call needed).
	phase1 := []Adapter{&ClusterAdapter{}}
	for _, a := range phase1 {
		if ctx.canceled() {
			return nil
		}
		adapterMark := time.Now()
		if err := r.create(ctx, a); err != nil {
			return err
		}
		r.log.Info("Adapter loaded.",
			"adapter", fmt.Sprintf("%T", a),
			"duration", time.Since(adapterMark).Seconds())
	}

	// Phase 2: HostAdapter, NetworkAdapter, StorageAdapter — all only need
	// the cluster cache from Phase 0. Fetch data in parallel, insert serially.
	phase2 := []Adapter{&HostAdapter{}, &NetworkAdapter{}, &StorageAdapter{}}
	if err := r.createParallel(ctx, phase2); err != nil {
		return err
	}

	// Phase 3+4: DiskAdapter consumes pre-fetched VM data + cached networks.
	phase34 := []Adapter{&DiskAdapter{}, &VMAdapter{}}
	for _, a := range phase34 {
		if ctx.canceled() {
			return nil
		}
		adapterMark := time.Now()
		if err := r.create(ctx, a); err != nil {
			return err
		}
		r.log.Info("Adapter loaded.",
			"adapter", fmt.Sprintf("%T", a),
			"duration", time.Since(adapterMark).Seconds())
	}
	return nil
}

// adapterResult holds the output of a parallel adapter.List() call.
type adapterResult struct {
	adapter Adapter
	itr     fb.Iterator
	err     error
	elapsed time.Duration
}

// createParallel fetches data from multiple adapters concurrently, then
// inserts each adapter's results into the DB serially.
func (r *Collector) createParallel(ctx *Context, adapters []Adapter) error {
	results := make([]adapterResult, len(adapters))
	var wg sync.WaitGroup
	wg.Add(len(adapters))
	for i, a := range adapters {
		go func(idx int, adapter Adapter) {
			defer wg.Done()
			start := time.Now()
			itr, err := adapter.List(ctx, r.provider)
			results[idx] = adapterResult{
				adapter: adapter,
				itr:     itr,
				err:     err,
				elapsed: time.Since(start),
			}
		}(i, a)
	}
	wg.Wait()

	for _, res := range results {
		if ctx.canceled() {
			return nil
		}
		if res.err != nil {
			return res.err
		}
		if err := r.insertFromIterator(ctx, res.itr); err != nil {
			return err
		}
		r.log.Info("Adapter loaded.",
			"adapter", fmt.Sprintf("%T", res.adapter),
			"duration", res.elapsed.Seconds())
	}
	return nil
}

// insertFromIterator writes all objects from an iterator into the DB.
func (r *Collector) insertFromIterator(ctx *Context, itr fb.Iterator) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.End()
	}()
	for {
		object, hasNext := itr.Next()
		if !hasNext {
			break
		}
		if ctx.canceled() {
			return nil
		}
		m := object.(libmodel.Model)
		if err := tx.Insert(m); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// List and create resources using the adapter.
func (r *Collector) create(ctx *Context, adapter Adapter) (err error) {
	itr, aErr := adapter.List(ctx, r.provider)
	if aErr != nil {
		err = aErr
		return
	}
	return r.insertFromIterator(ctx, itr)
}

// refresh dispatches to refreshSerial (standalone / first cycle) or
// refreshClusterOptimized (subsequent cluster cycles).
func (r *Collector) refresh(ctx *Context) error {
	if r.provider.IsHyperVCluster() && r.firstRefreshDone {
		return r.refreshClusterOptimized(ctx)
	}
	err := r.refreshSerial(ctx)
	if err == nil {
		r.firstRefreshDone = true
	}
	return err
}

// refreshSerial runs all adapters sequentially in non-LightMode.
func (r *Collector) refreshSerial(ctx *Context) (err error) {
	r.phase = Refresh
	r.client.InvalidateCycleCache()
	mark := time.Now()
	for _, adapter := range r.adapters() {
		if ctx.canceled() {
			return
		}
		if err = r.refreshAdapter(ctx, adapter); err != nil {
			return
		}
	}
	r.log.Info(
		"Refresh finished.",
		"duration",
		time.Since(mark))
	return
}

// refreshClusterOptimized runs VM+Disk in LightMode first (power state
// committed in seconds), then remaining adapters plus a concurrent
// Get-VHD enrichment for disk capacity and RCT. Every Nth cycle falls
// back to refreshSerial to pick up security, guest OS, and guest networks.
func (r *Collector) refreshClusterOptimized(ctx *Context) error {
	r.phase = Refresh
	r.client.InvalidateCycleCache()
	mark := time.Now()

	// Every Nth cycle, run a full non-LightMode refresh to pick up
	// disk capacity/RCT, security settings, guest OS, and guest networks.
	const fullRefreshInterval = 10
	r.refreshCount++
	fullCycle := r.refreshCount%fullRefreshInterval == 0
	if fullCycle {
		err := r.refreshSerial(ctx)
		r.log.Info("Refresh finished (full).", "duration", time.Since(mark))
		return err
	}

	if _, err := r.client.PrewarmClusterCache(); err != nil {
		return err
	}

	// Phase 1: VM + Disk in LightMode (power state ASAP).
	r.client.LightMode = true
	r.client.StartNetworkPrefetch()
	phase1 := []Adapter{&VMAdapter{}, &DiskAdapter{}}
	for _, a := range phase1 {
		if ctx.canceled() {
			r.client.LightMode = false
			return nil
		}
		if err := r.refreshAdapter(ctx, a); err != nil {
			r.client.LightMode = false
			return err
		}
	}
	r.client.LightMode = false

	// Phase 2: disk capacity/RCT enrichment runs concurrently with
	// remaining adapters. EnrichDiskCapacity uses parallel per-node
	// Get-VHD (~6s) while cluster/host/network/storage run alongside.
	enrichDone := make(chan error, 1)
	go func() {
		if err := r.client.EnrichDiskCapacity(); err != nil {
			r.log.Info("Disk enrichment failed", "error", err)
			enrichDone <- err
			return
		}
		enrichDone <- r.updateDisksFromCache(ctx)
	}()

	phase2 := []Adapter{&ClusterAdapter{}, &HostAdapter{}, &NetworkAdapter{}, &StorageAdapter{}}
	for _, a := range phase2 {
		if ctx.canceled() {
			<-enrichDone
			return nil
		}
		if err := r.refreshAdapter(ctx, a); err != nil {
			<-enrichDone
			return err
		}
	}
	if enrichErr := <-enrichDone; enrichErr != nil {
		r.log.Error(enrichErr, "Disk enrichment failed during optimized refresh")
	}

	r.log.Info(
		"Refresh finished (optimized).",
		"duration",
		time.Since(mark))
	return nil
}

// refreshAdapter runs DeleteUnexisting + GetUpdates for a single adapter.
func (r *Collector) refreshAdapter(ctx *Context, adapter Adapter) error {
	deletions, err := adapter.DeleteUnexisting(ctx)
	if err != nil {
		return err
	}
	if err = r.apply(deletions); err != nil {
		return err
	}
	updates, err := adapter.GetUpdates(ctx)
	if err != nil {
		return err
	}
	return r.apply(updates)
}

// updateDisksFromCache re-persists VMs and Disks from the client cache into
// the DB. Called after EnrichDiskCapacity to save the newly-filled capacity
// and RCT data without running a full refresh cycle.
func (r *Collector) updateDisksFromCache(ctx *Context) error {
	vms, err := r.client.ListVMs()
	if err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.End() }()
	for i := range vms {
		// Update the standalone Disk records.
		for _, d := range vms[i].Disks {
			m := &model.Disk{Base: model.Base{ID: d.ID}}
			if err := tx.Get(m); err != nil {
				if errors.Is(err, libmodel.NotFound) {
					continue
				}
				return fmt.Errorf("get disk %s: %w", d.ID, err)
			}
			if d.Capacity > 0 {
				m.Capacity = d.Capacity
				m.RCTEnabled = d.RCTEnabled
				if err := tx.Update(m); err != nil {
					return err
				}
			}
		}
		// Update the VM record's embedded disks (served by the REST API).
		vm := &model.VM{Base: model.Base{ID: vms[i].UUID}}
		if err := tx.Get(vm); err != nil {
			if errors.Is(err, libmodel.NotFound) {
				continue
			}
			return fmt.Errorf("get vm %s: %w", vms[i].UUID, err)
		}
		changed := false
		for j := range vm.Disks {
			for _, d := range vms[i].Disks {
				if vm.Disks[j].ID == d.ID && d.Capacity > 0 {
					vm.Disks[j].Capacity = d.Capacity
					vm.Disks[j].RCTEnabled = d.RCTEnabled
					changed = true
				}
			}
		}
		if changed {
			if err := tx.Update(vm); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// Apply the changeSet.
func (r *Collector) apply(changeSet []Updater) (err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()
	for _, updater := range changeSet {
		err = updater(tx)
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}

// clusterCount returns the number of clusters in the database.
func (r *Collector) clusterCount() int {
	count, err := r.db.Count(&model.Cluster{}, nil)
	if err != nil {
		r.log.Error(err, "Cluster count failed.")
	}
	return int(count)
}

// hostCount returns the number of hosts in the database.
func (r *Collector) hostCount() int {
	count, err := r.db.Count(&model.Host{}, nil)
	if err != nil {
		r.log.Error(err, "Host count failed.")
	}
	return int(count)
}

// vmCount returns the number of VMs in the database.
func (r *Collector) vmCount() int {
	count, _ := r.db.Count(&model.VM{}, nil)
	return int(count)
}

// networkCount returns the number of networks in the database.
func (r *Collector) networkCount() int {
	count, _ := r.db.Count(&model.Network{}, nil)
	return int(count)
}

// storageCount returns the number of storages in the database.
func (r *Collector) storageCount() int {
	count, _ := r.db.Count(&model.Storage{}, nil)
	return int(count)
}

// diskCount returns the number of disks in the database.
func (r *Collector) diskCount() int {
	count, _ := r.db.Count(&model.Disk{}, nil)
	return int(count)
}

// Add model watches.
func (r *Collector) beginWatch() {
	// Cluster watch — triggers VM revalidation when cluster state changes.
	if r.provider.IsHyperVCluster() {
		w, err := r.db.Watch(
			&model.Cluster{},
			&ClusterEventHandler{
				DB:  r.db,
				log: r.log,
			})
		if err != nil {
			r.log.Error(err, "Cluster watch failed.")
		} else {
			r.watches = append(r.watches, w)
		}

		w, err = r.db.Watch(
			&model.Host{},
			&HostEventHandler{
				DB:  r.db,
				log: r.log,
			})
		if err != nil {
			r.log.Error(err, "Host watch failed.")
		} else {
			r.watches = append(r.watches, w)
		}
	}

	w, err := r.db.Watch(
		&model.VM{},
		&VMEventHandler{
			Provider: r.provider,
			DB:       r.db,
			log:      r.log,
		})
	if err != nil {
		r.log.Error(err, "VM watch failed.")
		return
	}
	r.watches = append(r.watches, w)
}

// End watches.
func (r *Collector) endWatch() {
	for _, w := range r.watches {
		w.End()
	}
	r.watches = nil
}

// HyperVCredentials returns the HyperV/WinRM credentials from the secret.
func (r *Collector) HyperVCredentials() (username, password string) {
	return hvutil.HyperVCredentials(r.secret)
}

// SMBCredentials returns the SMB credentials from the secret.
func (r *Collector) SMBCredentials() (username, password string) {
	return hvutil.SMBCredentials(r.secret)
}

// SMBPath returns the local mount point where SMB is mounted in the pod.
func (r *Collector) SMBPath() string {
	return hvutil.SMBMountPath
}

// SMBUrl returns the SMB share URL from the secret.
func (r *Collector) SMBUrl() string {
	return hvutil.SMBUrl(r.secret)
}

// Context for collector operations.
type Context struct {
	client *Client
	db     libmodel.DB
	log    logging.LevelLogger
	ctx    context.Context
}

func (c *Context) canceled() bool {
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}
