package plan

import (
	"context"
	"errors"
	"fmt"
	"sort"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Condition type reported when the api clone method lacks host credentials.
const HostCredentialsNotReady = "HostCredentialsNotReady"

// validateHostCredentials checks that every ESXi host that could perform a
// copy-offload clone has a Ready Host CR carrying SDK credentials.
//
// This only applies to the "api" clone method. Unlike the VIB and SSH methods,
// it issues CopyVirtualDisk against the ESXi host itself -- the same call made
// through vCenter copies the data but does not offload to VAAI XCOPY -- so the
// populator must be able to log in to that host directly.
func (r *Reconciler) validateHostCredentials(ctx *plancontext.Context) error {
	plan := ctx.Plan

	sourceProvider := plan.Provider.Source
	if sourceProvider == nil || sourceProvider.Type() != api.VSphere {
		plan.Status.DeleteCondition(HostCredentialsNotReady)
		return nil
	}
	if !r.planUsesVSphereXcopyPopulator(plan) {
		plan.Status.DeleteCondition(HostCredentialsNotReady)
		return nil
	}
	if sourceProvider.Spec.Settings[api.ESXiCloneMethod] != api.ESXiCloneMethodAPI {
		plan.Status.DeleteCondition(HostCredentialsNotReady)
		return nil
	}

	hostIDs, err := r.hostsRequiringCredentials(ctx)
	if err != nil {
		return err
	}

	readyHosts, err := r.readyHostCRs(ctx)
	if err != nil {
		return err
	}

	var problems []string
	for _, hostID := range hostIDs {
		hostDef, found := readyHosts[hostID]
		if !found {
			problems = append(problems, fmt.Sprintf(
				"id:%s no Ready Host CR with credentials", hostID))
			continue
		}
		if hostDef.Spec.IpAddress == "" {
			problems = append(problems, fmt.Sprintf(
				"id:%s Host CR %q has an empty spec.ipAddress", hostID, hostDef.Name))
			continue
		}
		secret := &core.Secret{}
		sErr := ctx.Client.Get(context.TODO(), client.ObjectKey{
			Namespace: hostDef.Spec.Secret.Namespace,
			Name:      hostDef.Spec.Secret.Name,
		}, secret)
		if sErr != nil {
			problems = append(problems, fmt.Sprintf(
				"id:%s secret %q of Host CR %q cannot be read", hostID, hostDef.Spec.Secret.Name, hostDef.Name))
			continue
		}
		if len(secret.Data["user"]) == 0 || len(secret.Data["password"]) == 0 {
			problems = append(problems, fmt.Sprintf(
				"id:%s secret %q of Host CR %q is missing 'user' or 'password'", hostID, hostDef.Spec.Secret.Name, hostDef.Name))
		}
	}

	if len(problems) == 0 {
		plan.Status.DeleteCondition(HostCredentialsNotReady)
		return nil
	}

	sort.Strings(problems)
	plan.Status.SetCondition(libcnd.Condition{
		Type:     HostCredentialsNotReady,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message: fmt.Sprintf(
			"The 'esxiCloneMethod' setting on provider '%s' is '%s', which performs the copy through the ESXi host's own API and therefore needs credentials for each host that may run a copy. See the Items field for the hosts that are not ready.",
			sourceProvider.Name, api.ESXiCloneMethodAPI),
		Suggestion: fmt.Sprintf(
			"Create a Host CR in namespace '%s' for each listed ESXi host, with spec.ipAddress set to an address serving the host SDK and spec.secret referencing a secret with 'user' and 'password' keys.",
			sourceProvider.Namespace),
		Items: problems,
	})

	return nil
}

// hostsRequiringCredentials returns the IDs of every ESXi host that could be
// asked to perform a clone for this plan.
//
// When the storage map pins dedicated migration hosts, the builder picks one at
// random per disk, so every one of them needs credentials. Otherwise each VM is
// copied by the host it currently runs on.
func (r *Reconciler) hostsRequiringCredentials(ctx *plancontext.Context) ([]string, error) {
	plan := ctx.Plan
	ids := map[string]bool{}

	dedicated := false
	if plan.Map.Storage != nil {
		for _, mapping := range plan.Map.Storage.Spec.Map {
			if mapping.OffloadPlugin == nil || mapping.OffloadPlugin.VSphereXcopyPluginConfig == nil {
				continue
			}
			for _, hostID := range mapping.OffloadPlugin.VSphereXcopyPluginConfig.DedicatedMigrationHosts {
				if hostID != "" {
					ids[hostID] = true
					dedicated = true
				}
			}
		}
	}

	if !dedicated {
		for i := range plan.Spec.VMs {
			ref := &plan.Spec.VMs[i].Ref
			if ref.NotSet() {
				continue
			}
			v, pErr := ctx.Source.Inventory.VM(ref)
			if pErr != nil {
				// Missing or ambiguous VM refs are reported by validateVM; do not
				// duplicate those findings here.
				if errors.As(pErr, &web.NotFoundError{}) || errors.As(pErr, &web.RefNotUniqueError{}) {
					continue
				}
				return nil, liberr.Wrap(pErr)
			}
			if vm, ok := v.(*vsphere.VM); ok && vm.Host != "" {
				ids[vm.Host] = true
			}
		}
	}

	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// readyHostCRs returns the Ready Host CRs of the plan's source provider, keyed
// by inventory host ID.
func (r *Reconciler) readyHostCRs(ctx *plancontext.Context) (map[string]*api.Host, error) {
	list := &api.HostList{}
	err := r.List(
		context.TODO(),
		list,
		&client.ListOptions{Namespace: ctx.Plan.Provider.Source.Namespace},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	hosts := map[string]*api.Host{}
	for i := range list.Items {
		host := &list.Items[i]
		if !libref.Equals(&host.Spec.Provider, &ctx.Plan.Spec.Provider.Source) {
			continue
		}
		if !host.Status.HasCondition(libcnd.Ready) {
			continue
		}
		ref := host.Spec.Ref
		m := &vsphere.Host{}
		if pErr := ctx.Source.Inventory.Find(m, ref); pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				continue
			}
			return nil, liberr.Wrap(pErr)
		}
		hosts[m.ID] = host
	}

	return hosts, nil
}
