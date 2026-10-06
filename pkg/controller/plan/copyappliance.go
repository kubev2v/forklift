package plan

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	appliancectrl "github.com/kubev2v/forklift/pkg/controller/copyappliance"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func copyApplianceTemplateForProvider(c client.Client, provider *api.Provider) (*api.CopyApplianceTemplate, error) {
	toehold := &api.CopyApplianceTemplate{}
	err := c.Get(context.TODO(), client.ObjectKey{
		Namespace: provider.Namespace,
		Name:      provider.CopyApplianceTemplateName(),
	}, toehold)
	if err != nil {
		return nil, liberr.Wrap(err, "copy appliance template", provider.CopyApplianceTemplateName())
	}
	return toehold, nil
}

func (r *Migration) ensureCopyAppliance(vm *plan.VMStatus) error {
	provider := r.Source.Provider
	if provider == nil {
		return liberr.New("source provider is not available")
	}
	// Skip inventory build when the appliance already exists (every reconcile).
	if found, err := r.getCopyAppliance(vm, true); err != nil || found != nil {
		return err
	}

	toehold, err := copyApplianceTemplateForProvider(r.Client, provider)
	if err != nil {
		return err
	}
	builder, err := appliancectrl.NewBuilder(provider)
	if err != nil {
		return err
	}
	appliance, err := builder.Appliance(toehold, vm.Ref, r.Migration.UID)
	if err != nil {
		return err
	}
	// Warm CBT leaves the source tip locked; attach the parent base VMDK.
	if r.Plan.IsWarm() {
		for i := range appliance.Spec.AttachDisks {
			appliance.Spec.AttachDisks[i].VMDKPath = appliancectrl.BaseVMDKPath(appliance.Spec.AttachDisks[i].VMDKPath)
		}
	}
	// Plan labels are for cleanupOrphanedResources, not appliance identity.
	appliance.Labels[convctx.LabelPlan] = string(r.Plan.UID)
	appliance.Labels[convctx.LabelPlanName] = r.Plan.Name
	appliance.Labels[convctx.LabelPlanNamespace] = r.Plan.Namespace
	if err = controllerutil.SetControllerReference(r.Migration, appliance, scheme.Scheme); err != nil {
		return liberr.Wrap(err)
	}
	_, err = r.copyApplianceEnsurer.Appliance(context.TODO(), appliance)
	return err
}

func (r *Migration) buildCopyApplianceAttachDisks(vm *plan.VMStatus) ([]api.AttachedDisk, error) {
	provider := r.Source.Provider
	if provider == nil {
		return nil, liberr.New("source provider is not available")
	}
	toehold, err := copyApplianceTemplateForProvider(r.Client, provider)
	if err != nil {
		return nil, err
	}
	builder, err := appliancectrl.NewBuilder(provider)
	if err != nil {
		return nil, err
	}
	appliance, err := builder.Appliance(toehold, vm.Ref, r.Migration.UID)
	if err != nil {
		return nil, err
	}
	disks := appliance.Spec.AttachDisks
	if r.Plan.IsWarm() {
		for i := range disks {
			disks[i].VMDKPath = appliancectrl.BaseVMDKPath(disks[i].VMDKPath)
		}
	}
	return disks, nil
}

// setCopyApplianceAttachDisks patches Spec.AttachDisks. Nil releases disks;
// a non-empty list re-exports them.
func (r *Migration) setCopyApplianceAttachDisks(vm *plan.VMStatus, disks []api.AttachedDisk) error {
	appliance, err := r.getCopyAppliance(vm, true)
	if err != nil {
		return err
	}
	if appliance == nil {
		return liberr.New("copy appliance is gone", "vm", vm.ID)
	}
	patch := client.MergeFrom(appliance.DeepCopy())
	appliance.Spec.AttachDisks = disks
	return r.Patch(context.TODO(), appliance, patch)
}

func (r *Migration) releaseCopyApplianceDisks(vm *plan.VMStatus) error {
	return r.setCopyApplianceAttachDisks(vm, nil)
}

func (r *Migration) refreshCopyApplianceDisks(vm *plan.VMStatus) error {
	disks, err := r.buildCopyApplianceAttachDisks(vm)
	if err != nil {
		return err
	}
	return r.setCopyApplianceAttachDisks(vm, disks)
}

func (r *Migration) waitForCopyAppliance(vm *plan.VMStatus) (bool, error) {
	appliance, err := r.getCopyAppliance(vm, true)
	if err != nil {
		return false, err
	}
	if appliance == nil {
		return false, liberr.New("copy appliance is gone", "vm", vm.ID)
	}
	if appliance.Status.Phase == api.PhaseDeployFailed {
		return false, liberr.New(appliancectrl.FailureReason(appliance))
	}
	if c := appliance.Status.FindCondition(libcnd.Ready); c != nil &&
		(c.Category == libcnd.Critical || c.Category == libcnd.Error) && c.Message != "" {
		return false, liberr.New(c.Message)
	}
	if appliance.Status.Phase != api.PhaseDeployCompleted ||
		!appliancectrl.IsDeployReady(appliance) {
		return false, nil
	}
	_, err = appliancectrl.ExportNbdConnections(appliance, r.Source.Provider.ToeholdNbdSsl())
	return err == nil, err
}

func (r *Migration) waitForCopyApplianceReleased(vm *plan.VMStatus) (bool, error) {
	appliance, err := r.getCopyAppliance(vm, true)
	if err != nil {
		return false, err
	}
	if appliance == nil {
		return false, liberr.New("copy appliance is gone", "vm", vm.ID)
	}
	if appliance.Status.Phase == api.PhaseDeployFailed {
		return false, liberr.New(appliancectrl.FailureReason(appliance))
	}
	if c := appliance.Status.FindCondition(libcnd.Ready); c != nil &&
		(c.Category == libcnd.Critical || c.Category == libcnd.Error) && c.Message != "" {
		return false, liberr.New(c.Message)
	}
	return len(appliance.Spec.AttachDisks) == 0 &&
		appliance.Status.Phase == api.PhaseReleased, nil
}

func (r *Migration) teardownCopyAppliance(vm *plan.VMStatus) (bool, error) {
	appliance, err := r.getCopyAppliance(vm, false)
	if err != nil {
		return false, err
	}
	if appliance == nil {
		return true, nil
	}
	if appliance.DeletionTimestamp == nil {
		err = r.Delete(context.TODO(), appliance)
		if err != nil && !k8serr.IsNotFound(err) {
			return false, liberr.Wrap(err)
		}
		return false, nil
	}
	switch appliance.Status.Phase {
	case api.PhaseTeardownCompleted, api.PhaseTeardownFailed:
		return true, nil
	default:
		return false, nil
	}
}

func (r *Migration) deleteCopyAppliance(vm *plan.VMStatus) error {
	appliance, err := r.getCopyAppliance(vm, true)
	if err != nil || appliance == nil {
		return err
	}
	return client.IgnoreNotFound(r.Delete(context.TODO(), appliance))
}

func (r *Migration) getCopyAppliance(vm *plan.VMStatus, liveOnly bool) (*api.CopyAppliance, error) {
	provider := r.Source.Provider
	if provider == nil {
		return nil, liberr.New("source provider is not available")
	}
	labels := r.copyApplianceEnsurer.Labeler.ApplianceLabels(provider, r.Migration.UID, vm.ID)
	return r.copyApplianceEnsurer.Find(context.TODO(), provider.Namespace, labels, liveOnly)
}
