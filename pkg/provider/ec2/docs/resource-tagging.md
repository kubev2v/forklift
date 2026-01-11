# AWS Resource Tagging

The EC2 provider uses AWS tags to track migration resources. This approach makes migrations resilient to controller restarts and enables easy cleanup of orphaned resources.

## Why Tags?

Instead of storing resource state in Kubernetes, the EC2 provider uses AWS as the source of truth:

| Benefit | Description |
|---------|-------------|
| **Resilience** | Controller restarts don't lose track of AWS resources |
| **Idempotency** | Operations can be safely retried |
| **Visibility** | Resources are discoverable in AWS Console |
| **Cleanup** | Failed migrations can be manually cleaned by tag |

## Snapshot Tags

When the provider creates EBS snapshots:

| Tag Key | Example Value | Purpose |
|---------|---------------|---------|
| `forklift.konveyor.io/vmID` | `i-0abc123def456` | Links to source instance |
| `forklift.konveyor.io/vm-name` | `my-web-server` | Human-readable name |
| `forklift.konveyor.io/volume` | `vol-0def456abc` | Source volume ID |

## Volume Tags

When the provider creates new EBS volumes from snapshots:

| Tag Key | Example Value | Purpose |
|---------|---------------|---------|
| `forklift.konveyor.io/vmID` | `i-0abc123def456` | Links to source instance |
| `forklift.konveyor.io/vm-name` | `my-web-server` | Human-readable name |
| `forklift.konveyor.io/original-volume` | `vol-0def456abc` | Maps to source volume |
| `forklift.konveyor.io/snapshot` | `snap-0ghi789jkl` | Snapshot used to create |

## How Tags Are Used

**During migration:**
1. Create snapshots → tag with vmID and source volume
2. Query snapshots by vmID tag to check completion
3. Create volumes from snapshots → tag with vmID and metadata
4. Query volumes by vmID tag to verify availability

**For recovery:**
- If controller restarts mid-migration, it queries AWS by vmID tag
- Existing resources are discovered and migration continues

## Finding Tagged Resources

In AWS Console or CLI, filter by tag key `forklift.konveyor.io/vmID`:

```bash
# Find snapshots for a specific VM
aws ec2 describe-snapshots \
  --filters "Name=tag:forklift.konveyor.io/vmID,Values=i-0abc123def456"

# Find created volumes for a specific VM
aws ec2 describe-volumes \
  --filters "Name=tag:forklift.konveyor.io/vmID,Values=i-0abc123def456"
```

## Cleanup

After successful migration, snapshots are automatically deleted. Volumes are retained and managed through Kubernetes PVC lifecycle.

