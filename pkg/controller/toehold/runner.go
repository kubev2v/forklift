package toehold

import (
	"context"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libvsphere "github.com/kubev2v/forklift/pkg/lib/vsphere"
	"github.com/kubev2v/forklift/pkg/toehold/version"
	toeholdvsphere "github.com/kubev2v/forklift/pkg/toehold/vsphere"
	vimtypes "github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Runner drives a ToeholdTemplate to a ready vCenter template. It holds no
// state of its own: every pass reads where it got to from the status and
// leaves the next stage behind — same shape as the copy-appliance runners.
// ToeholdContext is the template's cluster client and CR. Runners read and
// write status through the same object the reconciler persists — same shape as
// copyappliance.ApplianceContext (k8s/CR half; vSphere is opened per stage).
type ToeholdContext struct {
	Client  client.Client
	Scheme  *runtime.Scheme
	Toehold *api.ToeholdTemplate
}

type Runner struct {
	context *ToeholdContext
}

// Run runs the current stage once. A finished stage sets Status.Stage to its
// successor; the next reconcile picks it up. done is true only when the
// Finished stage has been reached.
func (run *Runner) Run(ctx context.Context) (done bool, err error) {
	toehold := run.context.Toehold
	sshSecretName, sshPublicKey, sshProviderNS, err := run.context.loadToeholdSSH(ctx)
	if err != nil {
		return false, err
	}
	toehold.Status.Template.DiskHash = version.DiskHash(toehold.Spec, sshPublicKey)
	toehold.Status.Template.ConfigHash = version.ConfigHash(toehold.Spec)
	toehold.Status.Template.BaseContainerImage = toehold.Spec.BaseDisk.ContainerImage
	if toehold.Status.Stage == "" || toehold.Status.Stage == api.StageEnsurePrerequisites {
		toehold.Status.Stage = api.StageEnsureTemplate
	}
	toehold.Status.Phase = api.ToeholdTemplatePhaseRunning

	log.Info("toehold template stage",
		"toeholdTemplate", toehold.Name,
		"namespace", toehold.Namespace,
		"stage", toehold.Status.Stage,
		"phase", toehold.Status.Phase,
	)

	switch toehold.Status.Stage {
	case api.StageToeholdFinished:
		toehold.Status.Phase = api.ToeholdTemplatePhaseSucceeded
		now := meta.Now()
		toehold.Status.CompletionTime = &now
		return true, nil
	case api.StageEnsureTemplate:
		next, ensureErr := run.ensureTemplate(ctx, sshSecretName, sshPublicKey, sshProviderNS)
		if ensureErr != nil {
			return false, ensureErr
		}
		toehold.Status.Stage = next
	case api.StageBuildAndUpload:
		built, buildErr := run.buildAndUpload(ctx, sshSecretName, sshPublicKey, sshProviderNS)
		if buildErr != nil {
			return false, buildErr
		}
		if built {
			toehold.Status.Stage = api.StageToeholdFinished
		}
	default:
		return false, liberr.New(fmt.Sprintf("unknown toehold stage %q", toehold.Status.Stage))
	}
	return false, nil
}

func (run *Runner) ensureTemplate(ctx context.Context, sshSecretName, sshPublicKey, sshProviderNS string) (next api.ToeholdTemplateStage, err error) {
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
	if err = pctx.Client.ValidateInventory(ctx, toeholdvsphere.InventoryPreflight{
		Folder:    run.context.Toehold.Spec.Folder,
		Datastore: run.context.Toehold.Spec.Datastore,
		Network:   run.context.Toehold.Spec.Network,
	}); err != nil {
		return
	}

	diskHash := run.context.Toehold.Status.Template.DiskHash
	configHash := run.context.Toehold.Status.Template.ConfigHash

	ref, err := pctx.Client.FindVM(ctx, run.context.Toehold.Spec.Folder, run.context.Toehold.Spec.TemplateName, true)
	if err != nil {
		_ = pctx.Client.DestroyIfExists(ctx, run.context.Toehold.Spec.Folder, run.context.Toehold.Spec.TemplateName)
		return run.requireBuild(ctx, pctx)
	}
	anns, err := pctx.Client.GetAnnotationMap(ctx, ref.VM)
	if err != nil {
		return
	}
	storedDisk, storedConfig := anns[toeholdvsphere.DiskHashAnnotation], anns[toeholdvsphere.ConfigHashAnnotation]
	if storedDisk == "" || storedDisk != diskHash {
		_ = pctx.Client.DestroyIfExists(ctx, run.context.Toehold.Spec.Folder, run.context.Toehold.Spec.TemplateName)
		return run.requireBuild(ctx, pctx)
	}

	run.context.Toehold.Status.Template.Moref = ref.Moref
	if storedConfig != configHash {
		spec := vimtypes.VirtualMachineConfigSpec{
			NumCPUs:  cpuCount(run.context.Toehold),
			MemoryMB: int64(memoryMiB(run.context.Toehold)),
		}
		task, reconfErr := ref.VM.Reconfigure(ctx, spec)
		if reconfErr != nil {
			return "", liberr.Wrap(reconfErr, "reconfigure template hardware")
		}
		if err = task.Wait(ctx); err != nil {
			return "", liberr.Wrap(err, "reconfigure template hardware task")
		}
		if err = pctx.Client.SetAnnotationMap(ctx, ref.VM, map[string]string{
			toeholdvsphere.DiskHashAnnotation:           diskHash,
			toeholdvsphere.ConfigHashAnnotation:         configHash,
			toeholdvsphere.BaseContainerImageAnnotation: run.context.Toehold.Spec.BaseDisk.ContainerImage,
			toeholdvsphere.ImportedAtAnnotation:         run.context.Toehold.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z"),
		}); err != nil {
			return "", fmt.Errorf("stamp template %q moref=%s: %w", ref.Name, ref.Moref, err)
		}
	}
	run.context.Toehold.Status.SetCondition(libcnd.Condition{
		Type:     api.ToeholdTemplateUpToDate,
		Status:   libcnd.True,
		Category: libcnd.Advisory,
		Message:  "Reusing existing vCenter template.",
	})
	return api.StageToeholdFinished, nil
}

func (run *Runner) requireBuild(ctx context.Context, pctx *providerContext) (api.ToeholdTemplateStage, error) {
	if err := run.context.deleteBuildPod(ctx); err != nil {
		return "", err
	}
	if err := pctx.Client.ValidateInventory(ctx, toeholdvsphere.InventoryPreflight{
		Folder:       run.context.Toehold.Spec.Folder,
		Datastore:    run.context.Toehold.Spec.Datastore,
		Network:      run.context.Toehold.Spec.Network,
		MinFreeBytes: toeholdvsphere.DefaultTemplateDatastoreFreeBytes,
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
	run.context.Toehold.Status.BuildPod = &core.ObjectReference{
		Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name,
	}
	if pod.Status.Phase == core.PodFailed {
		return false, liberr.New("toehold build pod failed")
	}
	if pod.Status.Phase != core.PodSucceeded {
		run.context.Toehold.Status.Message = "Waiting for toehold build pod"
		return false, nil
	}
	pctx, err := run.context.providerContext(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = pctx.Client.Close(ctx) }()
	ref, err := pctx.Client.FindVM(ctx, run.context.Toehold.Spec.Folder, run.context.Toehold.Spec.TemplateName, true)
	if err != nil {
		return false, fmt.Errorf("find template %q after build: %w", run.context.Toehold.Spec.TemplateName, err)
	}
	now := meta.Now()
	run.context.Toehold.Status.Template.ImportedAt = &now
	run.context.Toehold.Status.Template.Moref = ref.Moref
	// ImportOVF already stamps disk/config hashes before mark-as-template.
	return true, nil
}

type providerContext struct {
	Provider *api.Provider
	Secret   *core.Secret
	Client   *toeholdvsphere.Client
}

func (c *ToeholdContext) providerContext(ctx context.Context) (*providerContext, error) {
	toehold := c.Toehold
	provider := &api.Provider{}
	err := c.Client.Get(ctx, types.NamespacedName{
		Namespace: toehold.Spec.Provider.Namespace,
		Name:      toehold.Spec.Provider.Name,
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
	vsClient, err := toeholdvsphere.NewClient(gc)
	if err != nil {
		return nil, err
	}
	return &providerContext{Provider: provider, Secret: secret, Client: vsClient}, nil
}
