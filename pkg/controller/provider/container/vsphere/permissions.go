package vsphere

import (
	"context"
	"fmt"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25/mo"
)

const PrivilegeCheckInterval = 10 * time.Minute

type privilegeGroup struct {
	Description string
	Privileges  []string
}

// MissingPrivileges groups missing privilege IDs by their functional description.
type MissingPrivileges struct {
	Group      string
	Privileges []string
}

type privilegeCheckResult struct {
	missing []MissingPrivileges
}

// requiredPrivileges defines the vSphere privileges MTV needs for migration,
// grouped by functional area. Derived from the govmomi operations in
// pkg/controller/plan/adapter/vsphere/client.go and
// pkg/controller/conversion/client.go.
var requiredPrivileges = []privilegeGroup{
	{
		Description: "Virtual Machine Snapshot Operations",
		Privileges: []string{
			"VirtualMachine.State.CreateSnapshot",
			"VirtualMachine.State.RemoveSnapshot",
		},
	},
	{
		Description: "Virtual Machine Power Management",
		Privileges: []string{
			"VirtualMachine.Interact.PowerOn",
			"VirtualMachine.Interact.PowerOff",
		},
	},
	{
		Description: "Virtual Machine Disk Access (VDDK)",
		Privileges: []string{
			"VirtualMachine.Provisioning.DiskRandomRead",
			"VirtualMachine.Provisioning.GetVmFiles",
			"VirtualMachine.Config.ChangeTracking",
		},
	},
	{
		Description: "Datastore Access",
		Privileges: []string{
			"Datastore.Browse",
			"Datastore.FileManagement",
		},
	},
}

// pruneToAvailablePrivileges intersects required privilege groups against the
// set of privileges the vCenter actually supports. This prevents false positives
// on older vCenter versions that lack certain privilege IDs.
func pruneToAvailablePrivileges(required []privilegeGroup, available map[string]bool) []privilegeGroup {
	var pruned []privilegeGroup
	for _, group := range required {
		var kept []string
		for _, priv := range group.Privileges {
			if available[priv] {
				kept = append(kept, priv)
			}
		}
		if len(kept) > 0 {
			pruned = append(pruned, privilegeGroup{
				Description: group.Description,
				Privileges:  kept,
			})
		}
	}
	return pruned
}

// flattenPrivileges extracts all privilege IDs from a slice of privilege groups
// into a single flat list, preserving order.
func flattenPrivileges(groups []privilegeGroup) []string {
	var flat []string
	for _, g := range groups {
		flat = append(flat, g.Privileges...)
	}
	return flat
}

// comparePrivileges checks which required privileges are missing given the
// granted booleans (aligned 1:1 with the flat privilege ID list).
func comparePrivileges(groups []privilegeGroup, granted map[string]bool) []MissingPrivileges {
	var result []MissingPrivileges
	for _, group := range groups {
		var missing []string
		for _, priv := range group.Privileges {
			if !granted[priv] {
				missing = append(missing, priv)
			}
		}
		if len(missing) > 0 {
			result = append(result, MissingPrivileges{
				Group:      group.Description,
				Privileges: missing,
			})
		}
	}
	return result
}

// FormatMissing produces a human-readable summary of missing privileges
// suitable for a condition Suggestion field.
func FormatMissing(missing []MissingPrivileges) string {
	var sb strings.Builder
	sb.WriteString("MISSING VSPHERE PRIVILEGES:\n\n")
	for _, m := range missing {
		fmt.Fprintf(&sb, "%s:\n", m.Group)
		for _, p := range m.Privileges {
			fmt.Fprintf(&sb, "  - %s\n", p)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("RESOLUTION:\n")
	sb.WriteString("Assign a vSphere role with these privileges to the service account\n")
	sb.WriteString("on the vCenter root folder with propagation enabled.\n")
	sb.WriteString("See: https://docs.redhat.com/en/documentation/migration_toolkit_for_virtualization/2.7/html-single/installing_and_using_the_migration_toolkit_for_virtualization/index#vmware-prerequisites_mtv\n")
	return sb.String()
}

// checkPermissionsWithClient checks whether the service account has the
// privileges required for migration using the provided govmomi client.
// Returns nil, nil if all privileges are granted.
func checkPermissionsWithClient(ctx context.Context, client *govmomi.Client) ([]MissingPrivileges, error) {
	authManager := object.NewAuthorizationManager(client.Client)

	var moAuthMgr mo.AuthorizationManager
	err := authManager.Properties(ctx, authManager.Reference(), []string{"privilegeList"}, &moAuthMgr)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	available := make(map[string]bool, len(moAuthMgr.PrivilegeList))
	for _, priv := range moAuthMgr.PrivilegeList {
		available[priv.PrivId] = true
	}

	pruned := pruneToAvailablePrivileges(requiredPrivileges, available)
	if len(pruned) == 0 {
		return nil, nil
	}

	flatPrivIDs := flattenPrivileges(pruned)

	sessionMgr := session.NewManager(client.Client)
	userSession, err := sessionMgr.UserSession(ctx)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	rootFolder := client.ServiceContent.RootFolder
	results, err := authManager.HasPrivilegeOnEntity(ctx, rootFolder, userSession.Key, flatPrivIDs)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	granted := make(map[string]bool, len(flatPrivIDs))
	for i, privID := range flatPrivIDs {
		if i < len(results) {
			granted[privID] = results[i]
		}
	}

	return comparePrivileges(pruned, granted), nil
}

// checkAndCachePermissions runs the privilege check using the collector's
// already-open client and caches the result. Called from getUpdates() after
// the client connects, before parity is achieved.
func (r *Collector) checkAndCachePermissions(ctx context.Context) {
	if r.provider.Spec.Settings[api.SDK] == api.ESXI {
		r.missingPrivsMu.Lock()
		r.missingPrivs = &privilegeCheckResult{}
		r.missingPrivsMu.Unlock()
		return
	}

	missing, err := checkPermissionsWithClient(ctx, r.client)
	if err != nil {
		r.log.Error(err, "Privilege check failed.")
		return
	}

	r.missingPrivsMu.Lock()
	r.missingPrivs = &privilegeCheckResult{missing: missing}
	r.missingPrivsMu.Unlock()
}
