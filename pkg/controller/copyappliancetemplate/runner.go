package copyappliancetemplate

import (
	"context"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/copyappliancetemplate/version"
	templatevsphere "github.com/kubev2v/forklift/pkg/copyappliancetemplate/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libvsphere "github.com/kubev2v/forklift/pkg/lib/vsphere"
	vimtypes "github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Runner drives a CopyApplianceTemplate to a ready vCenter template. It holds no
// state of its own: every pass reads where it got to from the status and
// leaves the next stage behind — same shape as the copy-appliance runners.
// CopyApplianceTemplateContext is the template's cluster client and CR. Runners read and
// write status through the same object the reconciler persists — same shape as
// copyappliance.ApplianceContext (k8s/CR half; vSphere is opened per stage).
type CopyApplianceTemplateContext struct {
	Client                client.Client
	Scheme                *runtime.Scheme
	CopyApplianceTemplate *api.CopyApplianceTemplate
}

type Runner struct {
	context *CopyApplianceTemplateContext
}

// Run runs the current stage once. A finished stage sets Status.Stage to its
// successor; the next reconcile picks it up. done is true only when the
// Finished stage has been reached.
func (run *Runner) Run(ctx context.Context) (done bool, err error) {
	copyApplianceTemplate := run.context.CopyApplianceTemplate
	sshSecretName, sshPublicKey, sshProviderNS, err := run.context.loadCopyApplianceSSH(ctx)
	if err != nil {
		return false, err
	}
	copyApplianceTemplate.Status.Template.DiskHash = version.DiskHash(copyApplianceTemplate.Spec, sshPublicKey)
	copyApplianceTemplate.Status.Template.ConfigHash = version.ConfigHash(copyApplianceTemplate.Spec)
	copyApplianceTemplate.Status.Template.BaseContainerImage = copyApplianceTemplate.Spec.BaseDisk.ContainerImage
	if copyApplianceTemplate.Status.Stage == "" || copyApplianceTemplate.Status.Stage == api.StageEnsurePrerequisites {
		copyApplianceTemplate.Status.Stage = api.StageEnsureTemplate
	}
	copyApplianceTemplate.Status.Phase = api.CopyApplianceTemplatePhaseRunning

	log.Info("copy appliance template stage",
		"copyApplianceTemplate", copyApplianceTemplate.Name,
		"namespace", copyApplianceTemplate.Namespace,
		"stage", copyApplianceTemplate.Status.Stage,
		"phase", copyApplianceTemplate.Status.Phase,
	)

	switch copyApplianceTemplate.Status.Stage {
	case api.StageTemplateFinished:
		copyApplianceTemplate.Status.Phase = api.CopyApplianceTemplatePhaseSucceeded
		now := meta.Now()
		copyApplianceTemplate.Status.CompletionTime = &now
		return true, nil
	case api.StageEnsureTemplate:
		next, ensureErr := run.ensureTemplate(ctx, sshSecretName, sshPublicKey, sshProviderNS)
		if ensureErr != nil {
			return false, ensureErr
		}
		copyApplianceTemplate.Status.Stage = next
	case api.StageBuildAndUpload:
		built, buildErr := run.buildAndUpload(ctx, sshSecretName, sshPublicKey, sshProviderNS)
		if buildErr != nil {
			return false, buildErr
		}
		if built {
			copyApplianceTemplate.Status.Stage = api.StageTemplateFinished
		}
	default:
		return false, liberr.New(fmt.Sprintf("unknown copyApplianceTemplate stage %q", copyApplianceTemplate.Status.Stage))
	}
	return false, nil
}

func (run *Runner) ensureTemplate(ctx context.Context, sshSecretName, sshPublicKey, sshProviderNS string) (next api.CopyApplianceTemplateStage, err error) {
	pctx, err := run.context.providerContext(ctx)
	if err != nil {
		return
	}
	defer func() { _ = pctx.Client.Close(ctx) }()

	if _, err = run.context.ensureSSHPublicSecret(ctx, sshSecretName, sshPublicKey, sshProviderNS); err != nil {
		return
	}
	if err = run.context.ensureCredsSecret(ctx, pctx, sshPublicKey); err != nil {
		return
	}
	if err = pctx.Client.ValidateInventory(ctx, templatevsphere.InventoryPreflight{
		Folder:    run.context.CopyApplianceTemplate.Spec.Folder,
		Datastore: run.context.CopyApplianceTemplate.Spec.Datastore,
		Network:   run.context.CopyApplianceTemplate.Spec.Network,
	}); err != nil {
		return
	}

	diskHash := run.context.CopyApplianceTemplate.Status.Template.DiskHash
	configHash := run.context.CopyApplianceTemplate.Status.Template.ConfigHash

	ref, err := pctx.Client.FindVM(ctx, run.context.CopyApplianceTemplate.Spec.Folder, run.context.CopyApplianceTemplate.Spec.TemplateName, true)
	if err != nil {
		_ = pctx.Client.DestroyIfExists(ctx, run.context.CopyApplianceTemplate.Spec.Folder, run.context.CopyApplianceTemplate.Spec.TemplateName)
		return run.requireBuild(ctx, pctx)
	}
	anns, err := pctx.Client.GetAnnotationMap(ctx, ref.VM)
	if err != nil {
		return
	}
	storedDisk, storedConfig := anns[templatevsphere.DiskHashAnnotation], anns[templatevsphere.ConfigHashAnnotation]
	if storedDisk == "" || storedDisk != diskHash {
		_ = pctx.Client.DestroyIfExists(ctx, run.context.CopyApplianceTemplate.Spec.Folder, run.context.CopyApplianceTemplate.Spec.TemplateName)
		return run.requireBuild(ctx, pctx)
	}

	run.context.CopyApplianceTemplate.Status.Template.Moref = ref.Moref
	if storedConfig != configHash {
		spec := vimtypes.VirtualMachineConfigSpec{
			NumCPUs:  cpuCount(run.context.CopyApplianceTemplate),
			MemoryMB: int64(memoryMiB(run.context.CopyApplianceTemplate)),
		}
		task, reconfErr := ref.VM.Reconfigure(ctx, spec)
		if reconfErr != nil {
			return "", liberr.Wrap(reconfErr, "reconfigure template hardware")
		}
		if err = task.Wait(ctx); err != nil {
			return "", liberr.Wrap(err, "reconfigure template hardware task")
		}
		if err = pctx.Client.SetAnnotationMap(ctx, ref.VM, map[string]string{
			templatevsphere.DiskHashAnnotation:           diskHash,
			templatevsphere.ConfigHashAnnotation:         configHash,
			templatevsphere.BaseContainerImageAnnotation: run.context.CopyApplianceTemplate.Spec.BaseDisk.ContainerImage,
			templatevsphere.ImportedAtAnnotation:         run.context.CopyApplianceTemplate.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z"),
		}); err != nil {
			return "", fmt.Errorf("stamp template %q moref=%s: %w", ref.Name, ref.Moref, err)
		}
	}
	run.context.CopyApplianceTemplate.Status.SetCondition(libcnd.Condition{
		Type:     api.CopyApplianceTemplateUpToDate,
		Status:   libcnd.True,
		Category: libcnd.Advisory,
		Message:  "Reusing existing vCenter template.",
	})
	return api.StageTemplateFinished, nil
}

func (run *Runner) requireBuild(ctx context.Context, pctx *providerContext) (api.CopyApplianceTemplateStage, error) {
	if err := run.context.deleteBuildPod(ctx); err != nil {
		return "", err
	}
	if err := pctx.Client.ValidateInventory(ctx, templatevsphere.InventoryPreflight{
		Folder:       run.context.CopyApplianceTemplate.Spec.Folder,
		Datastore:    run.context.CopyApplianceTemplate.Spec.Datastore,
		Network:      run.context.CopyApplianceTemplate.Spec.Network,
		MinFreeBytes: templatevsphere.DefaultTemplateDatastoreFreeBytes,
	}); err != nil {
		return "", err
	}
	return api.StageBuildAndUpload, nil
}

// buildAndUpload returns true once the build pod has succeeded and the
// template is stamped; false means still waiting.
func (run *Runner) buildAndUpload(ctx context.Context, sshSecretName, sshPublicKey, sshProviderNS string) (done bool, err error) {
	pod, err := run.context.ensureBuildPod(ctx, sshSecretName, sshPublicKey, sshProviderNS)
	if err != nil {
		return false, err
	}
	run.context.CopyApplianceTemplate.Status.BuildPod = &core.ObjectReference{
		Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name,
	}
	if pod.Status.Phase == core.PodFailed {
		return false, liberr.New("copyApplianceTemplate build pod failed")
	}
	if pod.Status.Phase != core.PodSucceeded {
		run.context.CopyApplianceTemplate.Status.Message = "Waiting for copyApplianceTemplate build pod"
		return false, nil
	}
	pctx, err := run.context.providerContext(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = pctx.Client.Close(ctx) }()
	ref, err := pctx.Client.FindVM(ctx, run.context.CopyApplianceTemplate.Spec.Folder, run.context.CopyApplianceTemplate.Spec.TemplateName, true)
	if err != nil {
		return false, fmt.Errorf("find template %q after build: %w", run.context.CopyApplianceTemplate.Spec.TemplateName, err)
	}
	now := meta.Now()
	run.context.CopyApplianceTemplate.Status.Template.ImportedAt = &now
	run.context.CopyApplianceTemplate.Status.Template.Moref = ref.Moref
	// ImportOVF already stamps disk/config hashes before mark-as-template.
	return true, nil
}

type providerContext struct {
	Provider *api.Provider
	Secret   *core.Secret
	Client   *templatevsphere.Client
}

func (c *CopyApplianceTemplateContext) providerContext(ctx context.Context) (*providerContext, error) {
	copyApplianceTemplate := c.CopyApplianceTemplate
	provider := &api.Provider{}
	err := c.Client.Get(ctx, types.NamespacedName{
		Namespace: copyApplianceTemplate.Spec.Provider.Namespace,
		Name:      copyApplianceTemplate.Spec.Provider.Name,
	}, provider)
	if err != nil {
		return nil, err
	}
	secret := &core.Secret{}
	err = c.Client.Get(ctx, types.NamespacedName{
		Namespace: provider.Spec.Secret.Namespace,
		Name:      provider.Spec.Secret.Name,
	}, secret)
	if err != nil {
		return nil, err
	}
	gc, err := libvsphere.ConnectProvider(
		ctx,
		provider.Spec.URL,
		string(secret.Data["user"]),
		string(secret.Data["password"]),
		provider.Status.Fingerprint,
		secret,
	)
	if err != nil {
		return nil, err
	}
	vsClient, err := templatevsphere.NewClient(gc)
	if err != nil {
		return nil, err
	}
	return &providerContext{Provider: provider, Secret: secret, Client: vsClient}, nil
}
