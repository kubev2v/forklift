package toehold

import (
	"context"
	"fmt"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/kubev2v/forklift/pkg/toehold/version"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// loadToeholdSSH reads the provider's toehold SSH public key once. Returns the
// secret name (in the provider namespace) and the key material.
func (c *ToeholdContext) loadToeholdSSH(ctx context.Context) (secretName, publicKey, providerNS string, err error) {
	toehold := c.Toehold
	providerNS = toehold.Spec.Provider.Namespace
	if providerNS == "" {
		providerNS = toehold.Namespace
	}
	provider := &api.Provider{}
	err = c.Client.Get(ctx, client.ObjectKey{
		Namespace: providerNS,
		Name:      toehold.Spec.Provider.Name,
	}, provider)
	if err != nil {
		err = liberr.Wrap(err, "provider", toehold.Spec.Provider.Name)
		return
	}
	providerNS = provider.Namespace
	secretName = provider.Status.ToeholdSSHPublicSecret
	if secretName == "" {
		err = liberr.New(
			"provider has no toehold SSH public secret yet",
			"provider", provider.Name)
		return
	}
	secret := &core.Secret{}
	err = c.Client.Get(ctx, client.ObjectKey{Namespace: providerNS, Name: secretName}, secret)
	if err != nil {
		err = liberr.Wrap(err, "secret", secretName)
		return
	}
	key, ok := secret.Data["public-key"]
	if !ok || len(key) == 0 {
		err = liberr.New("toehold SSH public key secret is missing public-key data", "secret", secretName)
		return
	}
	publicKey = string(key)
	return
}

func (c *ToeholdContext) setOwner(obj meta.Object) error {
	if c.Scheme == nil {
		return liberr.New("controller scheme is not configured")
	}
	return controllerutil.SetControllerReference(c.Toehold, obj, c.Scheme)
}

func (c *ToeholdContext) ensureCredsSecret(ctx context.Context, pctx *providerContext, sshPublicKey string) error {
	toehold := c.Toehold
	name := toehold.Name + credsSecretSuffix
	ns := toehold.Namespace
	data := map[string]string{
		settings.VCenterURL:                 pctx.Provider.Spec.URL,
		settings.VCenterUser:                string(pctx.Secret.Data["user"]),
		settings.VCenterPassword:            string(pctx.Secret.Data["password"]),
		settings.VCenterInsecure:            fmt.Sprint(base.GetInsecureSkipVerifyFlag(pctx.Secret)),
		settings.VCenterThumbprint:          pctx.Provider.Status.Fingerprint,
		settings.CopyApplianceTemplateContentHash: version.DiskHash(toehold.Spec, sshPublicKey),
		settings.ToeholdBaseContainerImage:  toehold.Spec.BaseDisk.ContainerImage,
	}
	secret := &core.Secret{}
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, secret)
	if k8serr.IsNotFound(err) {
		secret = &core.Secret{
			ObjectMeta: meta.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Type:       core.SecretTypeOpaque,
			StringData: data,
		}
		if err = c.setOwner(secret); err != nil {
			return liberr.Wrap(err)
		}
		return liberr.Wrap(c.Client.Create(ctx, secret))
	}
	if err != nil {
		return liberr.Wrap(err)
	}
	secret.StringData = data
	if err = c.setOwner(secret); err != nil {
		return liberr.Wrap(err)
	}
	return liberr.Wrap(c.Client.Update(ctx, secret))
}

func (c *ToeholdContext) ensureBuildPod(ctx context.Context, sshSecretName, sshPublicKey, sshProviderNS string) (*core.Pod, error) {
	toehold := c.Toehold
	podList := &core.PodList{}
	if err := c.Client.List(ctx, podList, &client.ListOptions{
		Namespace:     toehold.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{api.LabelToehold: toehold.Name}),
	}); err != nil {
		return nil, liberr.Wrap(err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		switch pod.Status.Phase {
		case core.PodFailed:
			if err := c.Client.Delete(ctx, pod); err != nil && !k8serr.IsNotFound(err) {
				return nil, liberr.Wrap(err)
			}
			continue
		case core.PodPending, core.PodRunning, core.PodSucceeded:
			if toehold.UID != "" && toehold.Namespace == pod.Namespace && !meta.IsControlledBy(pod, toehold) {
				if err := c.setOwner(pod); err != nil {
					return nil, liberr.Wrap(err)
				}
				if err := c.Client.Update(ctx, pod); err != nil {
					return nil, liberr.Wrap(err)
				}
			}
			return pod, nil
		}
	}

	localSSHSecret, err := c.ensureSSHPublicSecret(ctx, sshSecretName, sshPublicKey, sshProviderNS)
	if err != nil {
		return nil, err
	}

	builderImage := Settings.BuilderImage
	if toehold.Spec.BuilderImage != "" {
		builderImage = toehold.Spec.BuilderImage
	}
	activeDeadline := int64(7200)
	nodeSelector := map[string]string{"node-role.kubernetes.io/worker": ""}
	for k, v := range toehold.Spec.NodeSelector {
		nodeSelector[k] = v
	}
	nodeSelector["kubevirt.io/schedulable"] = "true"

	workDir := core.EmptyDirVolumeSource{}
	if giB := toehold.Spec.BaseDisk.WorkGiB; giB > 0 {
		workDir.SizeLimit = resource.NewQuantity(giB*1024*1024*1024, resource.BinarySI)
	}

	cpus := version.DefaultCPU
	if toehold.Spec.Resources.CPU > 0 {
		cpus = toehold.Spec.Resources.CPU
	}
	memMiB := version.DefaultMemoryMiB
	if toehold.Spec.Resources.MemoryMiB > 0 {
		memMiB = toehold.Spec.Resources.MemoryMiB
	}
	nonRoot := true
	allowPrivilegeEscalation := false
	user, fsGroup := qemuUser, qemuGroup
	seccompProfile := core.SeccompProfile{Type: core.SeccompProfileTypeRuntimeDefault}
	if settings.Settings.OpenShift {
		unshare := "profiles/unshare.json"
		seccompProfile = core.SeccompProfile{
			Type:             core.SeccompProfileTypeLocalhost,
			LocalhostProfile: &unshare,
		}
	}

	// Same security model as virt-v2v conversion pods (non-root + unshare, KVM device).
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: toehold.Name + "-build-",
			Namespace:    toehold.Namespace,
			Labels: map[string]string{
				api.LabelToehold: toehold.Name,
			},
		},
		Spec: core.PodSpec{
			RestartPolicy:         core.RestartPolicyNever,
			NodeSelector:          nodeSelector,
			ActiveDeadlineSeconds: &activeDeadline,
			SecurityContext: &core.PodSecurityContext{
				FSGroup:        &fsGroup,
				RunAsUser:      &user,
				RunAsNonRoot:   &nonRoot,
				SeccompProfile: &seccompProfile,
			},
			Volumes: []core.Volume{
				{
					Name: "base-disk",
					VolumeSource: core.VolumeSource{
						Image: &core.ImageVolumeSource{
							Reference:  toehold.Spec.BaseDisk.ContainerImage,
							PullPolicy: core.PullIfNotPresent,
						},
					},
				},
				{
					Name:         "work",
					VolumeSource: core.VolumeSource{EmptyDir: &workDir},
				},
				{
					Name: "ssh-public-key",
					VolumeSource: core.VolumeSource{
						Secret: &core.SecretVolumeSource{
							SecretName: localSSHSecret,
							Items: []core.KeyToPath{
								{Key: "public-key", Path: "public-key"},
							},
						},
					},
				},
			},
			Containers: []core.Container{
				{
					Name:            "build",
					Image:           builderImage,
					ImagePullPolicy: core.PullAlways,
					SecurityContext: &core.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivilegeEscalation,
						Capabilities:             &core.Capabilities{Drop: []core.Capability{"ALL"}},
					},
					EnvFrom: []core.EnvFromSource{{SecretRef: &core.SecretEnvSource{
						LocalObjectReference: core.LocalObjectReference{Name: toehold.Name + credsSecretSuffix},
					}}},
					Env: []core.EnvVar{
						{Name: settings.CopyApplianceTemplateName, Value: toehold.Spec.TemplateName},
						{Name: settings.ToeholdDatastore, Value: toehold.Spec.Datastore},
						{Name: settings.ToeholdFolder, Value: toehold.Spec.Folder},
						{Name: settings.ToeholdNetwork, Value: toehold.Spec.Network},
						{Name: settings.ToeholdBuildPodCPUs, Value: fmt.Sprint(cpus)},
						{Name: settings.ToeholdBuildPodMemoryMiB, Value: fmt.Sprint(memMiB)},
						{Name: settings.CopyApplianceTemplateConfigHash, Value: version.ConfigHash(toehold.Spec)},
						{Name: "TOEHOLD_SSH_PUBLIC_KEY_FILE", Value: "/etc/toehold/ssh/public-key"},
					},
					VolumeMounts: []core.VolumeMount{
						{Name: "base-disk", MountPath: "/disk", ReadOnly: true},
						{Name: "work", MountPath: "/work"},
						{Name: "ssh-public-key", MountPath: "/etc/toehold/ssh", ReadOnly: true},
					},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse("500m"),
							core.ResourceMemory: resource.MustParse("2Gi"),
							core.ResourceName("devices.kubevirt.io/kvm"): resource.MustParse("1"),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse("4"),
							core.ResourceMemory: resource.MustParse("8Gi"),
							core.ResourceName("devices.kubevirt.io/kvm"): resource.MustParse("1"),
						},
					},
				},
			},
		},
	}
	if toehold.Spec.BaseDisk.ImagePullSecret != nil {
		pod.Spec.ImagePullSecrets = []core.LocalObjectReference{*toehold.Spec.BaseDisk.ImagePullSecret}
	}

	if toehold.Spec.TransferNetwork != nil {
		ns := toehold.Spec.TransferNetwork.Namespace
		if ns == "" {
			ns = toehold.Namespace
		}
		nad := &k8snet.NetworkAttachmentDefinition{}
		err = c.Client.Get(ctx, client.ObjectKey{
			Namespace: ns,
			Name:      toehold.Spec.TransferNetwork.Name,
		}, nad)
		if err != nil {
			return nil, liberr.Wrap(err,
				"transferNetwork", toehold.Spec.TransferNetwork.Name,
				"namespace", ns)
		}
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		if err = base.ApplyTransferNetworkAnnotations(nad, pod.Annotations); err != nil {
			return nil, err
		}
	}

	if err = c.setOwner(pod); err != nil {
		return nil, liberr.Wrap(err)
	}
	if err = c.Client.Create(ctx, pod); err != nil {
		return nil, liberr.Wrap(err)
	}
	return pod, nil
}

func (c *ToeholdContext) deleteBuildPod(ctx context.Context) error {
	toehold := c.Toehold
	podList := &core.PodList{}
	err := c.Client.List(ctx, podList, &client.ListOptions{
		Namespace:     toehold.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{api.LabelToehold: toehold.Name}),
	})
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range podList.Items {
		if err = c.Client.Delete(ctx, &podList.Items[i]); err != nil && !k8serr.IsNotFound(err) {
			return liberr.Wrap(err)
		}
	}
	return nil
}

// ensureSSHPublicSecret copies the provider's toehold SSH public key into the
// build namespace when needed. Pods can only mount secrets from their own namespace.
// secretName/publicKey/providerNS come from loadToeholdSSH.
func (c *ToeholdContext) ensureSSHPublicSecret(ctx context.Context, secretName, publicKey, providerNS string) (string, error) {
	targetNS := c.Toehold.Namespace
	if targetNS == providerNS {
		return secretName, nil
	}

	target := &core.Secret{}
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: targetNS, Name: secretName}, target)
	if k8serr.IsNotFound(err) {
		target = &core.Secret{
			ObjectMeta: meta.ObjectMeta{
				Name:      secretName,
				Namespace: targetNS,
			},
			Type: core.SecretTypeOpaque,
			Data: map[string][]byte{
				"public-key": []byte(publicKey),
			},
		}
		if err = c.setOwner(target); err != nil {
			return "", liberr.Wrap(err)
		}
		return secretName, liberr.Wrap(c.Client.Create(ctx, target))
	}
	if err != nil {
		return "", liberr.Wrap(err)
	}
	target.Data = map[string][]byte{
		"public-key": []byte(publicKey),
	}
	if err = c.setOwner(target); err != nil {
		return "", liberr.Wrap(err)
	}
	return secretName, liberr.Wrap(c.Client.Update(ctx, target))
}

func cpuCount(toehold *api.CopyApplianceTemplate) int32 {
	if toehold.Spec.Resources.CPU > 0 {
		return toehold.Spec.Resources.CPU
	}
	return version.DefaultCPU
}

func memoryMiB(toehold *api.CopyApplianceTemplate) int32 {
	if toehold.Spec.Resources.MemoryMiB > 0 {
		return toehold.Spec.Resources.MemoryMiB
	}
	return version.DefaultMemoryMiB
}
