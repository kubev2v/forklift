package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	coordinationclientv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	defaultLeaseDuration = 30 * time.Second
	leaseRenewInterval   = 3 * time.Second
	minLeaseDuration     = leaseRenewInterval + 2*time.Second
	leaseCleanupBound    = 10 * time.Second
	leaseRenewTimeout   = 5 * time.Second
)

// downloadLeaseLocker limits concurrent Nutanix HTTP downloads with the same
// slot-lease pattern used by the vSphere copy-offload populator.
type downloadLeaseLocker struct {
	namespace            string
	clientset            kubernetes.Interface
	leaseDuration        time.Duration
	retryInterval        time.Duration
	renewInterval        time.Duration
	maxConcurrentHolders int
}

func newDownloadLeaseLocker(slots int, namespace string, leaseDurationSec int) (*downloadLeaseLocker, error) {
	if slots < 1 {
		slots = 1
	}
	if namespace == "" {
		namespace = "openshift-mtv"
	}
	duration := defaultLeaseDuration
	if leaseDurationSec > 0 {
		if leaseDurationSec > math.MaxInt32 {
			return nil, fmt.Errorf("lease-duration-seconds %d exceeds int32 range", leaseDurationSec)
		}
		duration = time.Duration(leaseDurationSec) * time.Second
		if duration < minLeaseDuration {
			return nil, fmt.Errorf("lease-duration-seconds %d is below minimum %d",
				leaseDurationSec, int(minLeaseDuration.Seconds()))
		}
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config for download leases: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client for download leases: %w", err)
	}
	return &downloadLeaseLocker{
		namespace:            namespace,
		clientset:            cs,
		leaseDuration:        duration,
		retryInterval:        10 * time.Second,
		renewInterval:        leaseRenewInterval,
		maxConcurrentHolders: slots,
	}, nil
}

func (h *downloadLeaseLocker) withLock(ctx context.Context, leaseKey string, work func(context.Context) error) error {
	if leaseKey == "" {
		return work(ctx)
	}
	if errs := validation.IsDNS1123Label(leaseKey); len(errs) > 0 {
		return fmt.Errorf("lease key %q is not a valid DNS label: %v", leaseKey, errs)
	}

	holder, err := os.Hostname()
	if err != nil {
		holder = "nutanix-http-" + uuid.New().String()
	}
	log := klog.Background().WithName("nutanix-http").WithName("lease")
	leaseClient := h.clientset.CoordinationV1().Leases(h.namespace)

	testName := fmt.Sprintf("nutanix-http-%s-slot-0", leaseKey)
	if _, err = leaseClient.Get(ctx, testName, metav1.GetOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("lease API access failed for %s: %w", leaseKey, err)
	}

	for {
		if ctx.Err() != nil {
			return fmt.Errorf("context canceled while waiting for download lease: %w", ctx.Err())
		}
		for slot := 0; slot < h.maxConcurrentHolders; slot++ {
			lease, ok, err := h.tryAcquireSlot(ctx, leaseClient, holder, leaseKey, slot)
			if err != nil {
				return err
			}
			if ok {
				log.Info("download lease acquired", "slot", slot, "key", leaseKey)
				return h.execute(ctx, leaseClient, lease, leaseKey, slot, work)
			}
		}
		log.Info("all download lease slots taken, waiting", "slots", h.maxConcurrentHolders, "key", leaseKey)
		select {
		case <-time.After(h.retryInterval):
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for download lease: %w", ctx.Err())
		}
	}
}

// tryAcquireSlot attempts to Create or take over one lease slot.
// ok is false when the slot is held by someone else.
func (h *downloadLeaseLocker) tryAcquireSlot(
	ctx context.Context,
	leaseClient coordinationclientv1.LeaseInterface,
	holder, leaseKey string,
	slot int,
) (lease *coordinationv1.Lease, ok bool, err error) {
	leaseDurationSec := int32(h.leaseDuration.Seconds())
	leaseName := fmt.Sprintf("nutanix-http-%s-slot-%d", leaseKey, slot)
	now := metav1.NewMicroTime(time.Now())
	lease = &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: h.namespace,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &holder,
			LeaseDurationSeconds: &leaseDurationSec,
			AcquireTime:          &now,
			RenewTime:            &now,
		},
	}
	created, err := leaseClient.Create(ctx, lease, metav1.CreateOptions{})
	if err == nil {
		return created, true, nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return nil, false, fmt.Errorf("create lease %s: %w", leaseName, err)
	}
	existing, getErr := leaseClient.Get(ctx, leaseName, metav1.GetOptions{})
	if getErr != nil {
		// Lost the race with a delete; try the next slot (or the
		// outer retry) instead of tight-looping on this index.
		if apierrors.IsNotFound(getErr) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get lease %s: %w", leaseName, getErr)
	}
	if !h.expired(existing) {
		return nil, false, nil
	}
	existing.Spec.HolderIdentity = &holder
	now = metav1.NewMicroTime(time.Now())
	existing.Spec.AcquireTime = &now
	existing.Spec.RenewTime = &now
	updated, updateErr := leaseClient.Update(ctx, existing, metav1.UpdateOptions{})
	if updateErr != nil {
		if apierrors.IsConflict(updateErr) || apierrors.IsNotFound(updateErr) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("take over lease %s: %w", leaseName, updateErr)
	}
	return updated, true, nil
}

func (h *downloadLeaseLocker) expired(lease *coordinationv1.Lease) bool {
	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return false
	}
	return time.Now().After(lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second))
}

func (h *downloadLeaseLocker) execute(
	ctx context.Context,
	leaseClient coordinationclientv1.LeaseInterface,
	lease *coordinationv1.Lease,
	leaseKey string,
	slot int,
	work func(context.Context) error,
) error {
	log := klog.Background().WithName("nutanix-http").WithName("lease")
	workCtx, workCancel := context.WithCancel(ctx)
	defer workCancel()
	renewCtx, renewCancel := context.WithCancel(ctx)
	defer renewCancel()

	workDone := make(chan struct{})
	renewalErrors := make(chan error, 1)
	var renewWG sync.WaitGroup
	renewWG.Add(1)
	go func() {
		defer renewWG.Done()
		ticker := time.NewTicker(h.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				now := metav1.NewMicroTime(time.Now())
				lease.Spec.RenewTime = &now
				rctx, rcancel := context.WithTimeout(renewCtx, leaseRenewTimeout)
				updated, err := leaseClient.Update(rctx, lease, metav1.UpdateOptions{})
				rcancel()
				if err != nil {
					workCancel()
					select {
					case renewalErrors <- fmt.Errorf("failed to renew download lease: %w", err):
					default:
					}
					return
				}
				lease = updated
			case <-renewCtx.Done():
				return
			case <-workDone:
				return
			}
		}
	}()

	workErr := work(workCtx)
	close(workDone)
	renewCancel()
	renewWG.Wait()
	select {
	case renewErr := <-renewalErrors:
		if workErr == nil {
			workErr = renewErr
		}
		if errors.Is(workCtx.Err(), context.Canceled) {
			log.Info("download cancelled after lease renewal failure", "slot", slot, "key", leaseKey)
		}
	default:
	}
	// Delete so the next waiter can Create immediately instead of waiting
	// for LeaseDurationSeconds after we stop renewing. Preconditions avoid
	// deleting a slot another worker has taken over.
	uid := lease.UID
	rv := lease.ResourceVersion
	deleteCtx, deleteCancel := context.WithTimeout(context.WithoutCancel(ctx), leaseCleanupBound)
	defer deleteCancel()
	delErr := leaseClient.Delete(deleteCtx, lease.Name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &rv},
	})
	if delErr != nil && !apierrors.IsNotFound(delErr) && !apierrors.IsConflict(delErr) {
		log.Error(delErr, "failed to delete download lease", "slot", slot, "key", leaseKey)
	}
	log.Info("download lease released", "slot", slot, "key", leaseKey)
	return workErr
}
