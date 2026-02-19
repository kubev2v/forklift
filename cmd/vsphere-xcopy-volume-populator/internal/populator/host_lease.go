package populator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	coordinationclientv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/klog/v2"
)

// HostLeaseLocker is a mechanism to use k8s lease object to perform
// critical sections exclusively. This is important to prevent heavy
// operations such as rescans from destabilizing the ESX and the copy process.
type HostLeaseLocker struct {
	// the namespace should be constant as we want to lock ESX operations across migration
	// plans. One option is to hardcode openshift-mtv, the other is to consider
	// a new secret value or a flag
	namespace string
	clientset kubernetes.Interface
	// leaseDuration is how long the lease is held (in seconds). Default: 10 seconds
	// Can be configured via HOST_LEASE_DURATION_SECONDS env var
	leaseDuration time.Duration
	// retryInterval is how long to wait before retrying to acquire a held lease. Default: 10 seconds
	retryInterval time.Duration
	// renewInterval is how often to renew the lease while work is running. Default: 3 seconds
	renewInterval time.Duration
	// maxConcurrentHolders is the maximum number of concurrent lease holders per host. Default: 2
	maxConcurrentHolders int
}

// NewHostLeaseLocker creates a new HostLeaseLocker with the given clientset
func NewHostLeaseLocker(clientset kubernetes.Interface) *HostLeaseLocker {
	h := HostLeaseLocker{
		clientset:            clientset,
		leaseDuration:        10 * time.Second,
		retryInterval:        10 * time.Second,
		renewInterval:        3 * time.Second,
		maxConcurrentHolders: 2,
		namespace:            "openshift-mtv",
	}

	if leaseNs := os.Getenv("HOST_LEASE_NAMESPACE"); leaseNs != "" {
		h.namespace = leaseNs
	}

	if durationStr := os.Getenv("HOST_LEASE_DURATION_SECONDS"); durationStr != "" {
		if duration, err := time.ParseDuration(durationStr + "s"); err == nil {
			h.leaseDuration = duration
		}
	}

	return &h
}

// WithLock acquires a distributed lock for a specific ESXi host using direct Lease API.
// It blocks until the lock is acquired or the context is canceled.
// The actual work (the critical section) is performed by the provided `work` function.
// The lease is automatically renewed while work is running and deleted when complete.
func (h *HostLeaseLocker) WithLock(ctx context.Context, hostID string, work func(ctx context.Context) error) error {
	if hostID == "" {
		return fmt.Errorf("hostID is empty, can't hold a lease without any identity")
	}

	if dnsValidationErrors := validation.IsDNS1123Label(hostID); len(dnsValidationErrors) > 0 {
		return fmt.Errorf("the hostID to use for the lease isn't a valid DNS name: %v", dnsValidationErrors)
	}

	// 1. Define a unique identity for this populator instance (the lock holder).
	lockHolderIdentity, err := os.Hostname()
	if err != nil {
		lockHolderIdentity = "populator-" + uuid.New().String()
	}
	log := klog.Background().WithName("copy-offload").WithName("lease")
	log.V(2).Info("populator identity", "identity", lockHolderIdentity)

	// 2. Get the lease client
	leaseClient := h.clientset.CoordinationV1().Leases(h.namespace)

	// 3. Pre-check: Verify we can access the Lease API before entering retry loop.
	// Try to get slot-0 as a test (it may or may not exist)
	testLeaseName := fmt.Sprintf("esxi-lock-%s-slot-0", hostID)
	_, err = leaseClient.Get(ctx, testLeaseName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		// API access error (not just "lease doesn't exist") - fail fast
		return fmt.Errorf("failed to access lease API for host %s (failing fast - not retrying): %w", hostID, err)
	}

	// 4. Try to acquire any available lease slot in a retry loop
	leaseDurationSec := int32(h.leaseDuration.Seconds())

	for {
		// Check if context is canceled
		if ctx.Err() != nil {
			return fmt.Errorf("context canceled while waiting for lock: %w", ctx.Err())
		}

		// Try each slot in order
		for slot := 0; slot < h.maxConcurrentHolders; slot++ {
			leaseName := fmt.Sprintf("esxi-lock-%s-slot-%d", hostID, slot)

			// Try to create the lease for this slot
			now := metav1.NewMicroTime(time.Now())
			lease := &coordinationv1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      leaseName,
					Namespace: h.namespace,
				},
				Spec: coordinationv1.LeaseSpec{
					HolderIdentity:       &lockHolderIdentity,
					LeaseDurationSeconds: &leaseDurationSec,
					AcquireTime:          &now,
					RenewTime:            &now,
				},
			}

			createdLease, err := leaseClient.Create(ctx, lease, metav1.CreateOptions{})
			if err == nil {
				// Successfully created the lease - we have a slot!
				log.Info("lease acquired", "slot", slot, "host", hostID)
				return h.executeWorkWithLease(ctx, leaseClient, createdLease, hostID, slot, work)
			}

			// If it's not an "already exists" error, it's an API error - fail fast
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create lease for host %s slot %d (API error - not retrying): %w", hostID, slot, err)
			}

			// Lease already exists - check if it's expired or still held
			existingLease, getErr := leaseClient.Get(ctx, leaseName, metav1.GetOptions{})
			if getErr != nil {
				if !apierrors.IsNotFound(getErr) {
					// Failed to get the existing lease - this is an API error
					return fmt.Errorf("failed to get existing lease for host %s slot %d (API error - not retrying): %w", hostID, slot, getErr)
				}
				// Lease was deleted between create and get - try this slot again
				log.V(2).Info("lease was deleted, retrying slot", "lease", leaseName)
				// Retry this slot immediately by continuing the loop
				slot--
				continue
			}

			// Check if the lease is expired
			if h.isLeaseExpired(existingLease) {
				// Lease is expired - try to take it over
				log.Info("lease expired, attempting takeover", "lease", leaseName, "slot", slot)
				existingLease.Spec.HolderIdentity = &lockHolderIdentity
				now := metav1.NewMicroTime(time.Now())
				existingLease.Spec.AcquireTime = &now
				existingLease.Spec.RenewTime = &now

				updatedLease, updateErr := leaseClient.Update(ctx, existingLease, metav1.UpdateOptions{})
				if updateErr == nil {
					// Successfully took over the expired lease
					log.Info("lease acquired (expired takeover)", "slot", slot, "host", hostID)
					return h.executeWorkWithLease(ctx, leaseClient, updatedLease, hostID, slot, work)
				}
				// Update failed (likely someone else took it or conflict) - try next slot
				log.V(2).Info("failed to take over expired lease", "slot", slot, "err", updateErr)
			} else {
				// Lease is held by someone else
				holder := "unknown"
				if existingLease.Spec.HolderIdentity != nil {
					holder = *existingLease.Spec.HolderIdentity
				}
				log.V(2).Info("lease slot held by another", "slot", slot, "host", hostID, "holder", holder)
			}
		}

		// All slots are taken - wait and retry
		log.Info("all lease slots taken, waiting before retry", "slots", h.maxConcurrentHolders, "host", hostID, "wait", h.retryInterval)

		select {
		case <-time.After(h.retryInterval):
			// Retry all slots
		case <-ctx.Done():
			return fmt.Errorf("context canceled while waiting for lock: %w", ctx.Err())
		}
	}
}

// isLeaseExpired checks if a lease has expired
func (h *HostLeaseLocker) isLeaseExpired(lease *coordinationv1.Lease) bool {
	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return false
	}
	expiryTime := lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
	return time.Now().After(expiryTime)
}

// executeWorkWithLease executes the work while holding the lease and renewing it periodically
func (h *HostLeaseLocker) executeWorkWithLease(
	ctx context.Context,
	leaseClient coordinationclientv1.LeaseInterface,
	lease *coordinationv1.Lease,
	hostID string,
	slot int,
	work func(context.Context) error,
) error {
	log := klog.Background().WithName("copy-offload").WithName("lease")
	log.Info("lease acquired", "slot", slot, "host", hostID)

	// Create a context for the work that we can cancel if renewal fails
	workCtx, workCancel := context.WithCancel(ctx)
	defer workCancel()

	// Create a context for the renewal goroutine
	renewCtx, renewCancel := context.WithCancel(ctx)
	defer renewCancel()

	// Channel to signal work completion
	workDone := make(chan struct{})
	renewalErrors := make(chan error, 1)

	// Start lease renewal goroutine
	go func() {
		ticker := time.NewTicker(h.renewInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Renew the lease
				now := metav1.NewMicroTime(time.Now())
				lease.Spec.RenewTime = &now

				updatedLease, err := leaseClient.Update(renewCtx, lease, metav1.UpdateOptions{})
				if err != nil {
					log.Error(err, "failed to renew lease", "slot", slot, "host", hostID)

					// Cancel the work context immediately - we've lost the lock!
					workCancel()

					select {
					case renewalErrors <- fmt.Errorf("failed to renew lease, work cancelled: %w", err):
					default:
					}
					return
				}
				lease = updatedLease
				log.V(2).Info("lease renewed", "slot", slot, "host", hostID)

			case <-renewCtx.Done():
				// Work completed or context canceled
				return
			case <-workDone:
				// Work completed
				return
			}
		}
	}()

	// Execute the work
	workErr := work(workCtx)
	close(workDone)
	renewCancel() // Stop the renewal goroutine

	// Check if there was a renewal error
	select {
	case renewErr := <-renewalErrors:
		if workErr == nil {
			workErr = renewErr
		}
		// Add context to help debugging
		if errors.Is(workCtx.Err(), context.Canceled) {
			log.Info("work cancelled due to lease renewal failure", "slot", slot, "host", hostID)
		}
	default:
	}

	log.Info("lease released", "slot", slot, "host", hostID)

	// Note: We intentionally do NOT delete the lease explicitly.
	// The lease will auto-expire after leaseDuration (10s), at which point
	// other pods can acquire it. This is simpler and more reliable than
	// explicit deletion, which can fail silently. The 10-second delay is
	// acceptable given typical operation durations (30-300s per disk).
	log.V(2).Info("lease will auto-expire", "slot", slot, "host", hostID, "duration", h.leaseDuration)

	return workErr
}
