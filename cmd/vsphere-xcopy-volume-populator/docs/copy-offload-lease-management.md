# vSphere XCOPY Volume Populator: Host Lease Management

To prevent overloading ESXi hosts during concurrent migrations, the vsphere-xcopy-volume-populator uses a distributed lease mechanism based on Kubernetes Lease objects. This ensures that heavy operations like storage rescans are serialized per ESXi host.

### How It Works

- **Lease-based Locking**: Before performing operations that could destabilize an ESXi host (such as rescanning storage), the populator acquires a lease specific to that host.
- **Concurrent Slots**: Multiple populators can work on the same host concurrently (default: 2 slots per host).
- **Automatic Renewal**: Leases are automatically renewed while work is in progress.
- **Auto-expiration**: Leases expire automatically after the configured duration to prevent deadlocks.

### Configuration

Host lease behavior can be configured via the ForkliftController CR:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: ForkliftController
metadata:
  name: forklift-controller
  namespace: openshift-mtv
spec:
  controller_host_lease_namespace: "openshift-mtv"        # Namespace for lease objects (default: openshift-mtv)
  controller_host_lease_duration_seconds: 10              # Lease duration in seconds (default: 10)
```

These settings are propagated to the populator pods via environment variables and control:
- **HOST_LEASE_NAMESPACE**: The namespace where lease objects are created (should be consistent across all migrations)
- **HOST_LEASE_DURATION_SECONDS**: How long a lease is held before auto-expiring

### Default Behavior

If not configured, the system uses these defaults:
- Namespace: `openshift-mtv` (same namespace as the migration infrastructure)
- Duration: `10 seconds` (balances responsiveness with operation duration)
- Max concurrent holders: `2` per ESXi host (hardcoded)

### Monitoring Leases

You can view active leases to see which populators are working on which hosts:

```bash
# View all ESXi host leases
oc get leases -n openshift-mtv | grep esxi-lock

# Get details of a specific lease
oc get lease esxi-lock-host-1234-slot-0 -n openshift-mtv -o yaml
```

Each lease shows:
- **holderIdentity**: The populator pod holding the lease
- **renewTime**: Last time the lease was renewed
- **leaseDurationSeconds**: How long until auto-expiration