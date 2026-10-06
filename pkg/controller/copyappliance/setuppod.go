package copyappliance

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// setupKeyDir is where the appliance secret is mounted in the setup pod.
const setupKeyDir = "/etc/forklift/appliance"

// setupPodSelector matches the appliance's setup pods. Appliance names are
// generated, so one is unique in its namespace.
func setupPodSelector(appliance *api.CopyAppliance) client.MatchingLabelsSelector {
	return client.MatchingLabelsSelector{
		Selector: labels.SelectorFromSet(
			map[string]string{api.LabelCopyApplianceSetup: appliance.Name}),
	}
}

// setupPod returns the appliance's setup pod, or nil when it has none.
func (r *DeployRunner) setupPod(ctx context.Context) (pod *core.Pod, err error) {
	appliance := r.context.Appliance
	podList := &core.PodList{}
	err = r.client.List(ctx, podList,
		client.InNamespace(appliance.Namespace),
		setupPodSelector(appliance))
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(podList.Items) > 0 {
		pod = &podList.Items[0]
	}
	return
}

// createSetupPod starts a pod that loads the image at pullSpec into the
// appliance's podman store under tag, and installs the supervisor that runs it.
// pullSpec is pinned to a digest by the caller.
//
// The pod reads the image with no credential and reaches no API server, so it
// gets no service account token.
func (r *DeployRunner) createSetupPod(ctx context.Context, pullSpec, tag string) (err error) {
	appliance := r.context.Appliance
	address, ok := appliance.Address()
	if !ok {
		err = liberr.New(
			"the appliance reports no address to reach it on",
			"appliance", appliance.Name)
		return
	}
	if Settings.SetupImage == "" {
		err = liberr.New(
			"no image is configured for the appliance setup pod",
			"setting", settings.CopyApplianceSetupImage)
		return
	}
	networks, err := setupNetworks(Settings.SetupTransferNetwork, Settings.Namespace)
	if err != nil {
		return
	}

	// Past the transfer timeout the pod is making no further progress, and its
	// login to the appliance has already been dropped by the deadline the
	// worker set on it. The margin covers the image pull and the exit.
	activeDeadline := int64(SSHFileTransferTimeout.Seconds()) + 600
	automount := false
	nonRoot := true
	allowPrivilegeEscalation := false

	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: appliance.Name + "-setup-",
			Namespace:    appliance.Namespace,
			Labels: map[string]string{
				api.LabelCopyApplianceSetup: appliance.Name,
			},
		},
		Spec: core.PodSpec{
			RestartPolicy:                core.RestartPolicyNever,
			ActiveDeadlineSeconds:        &activeDeadline,
			AutomountServiceAccountToken: &automount,
			SecurityContext: &core.PodSecurityContext{
				RunAsNonRoot:   &nonRoot,
				SeccompProfile: &core.SeccompProfile{Type: core.SeccompProfileTypeRuntimeDefault},
			},
			Volumes: []core.Volume{
				{
					Name: "appliance-secret",
					VolumeSource: core.VolumeSource{
						Secret: &core.SecretVolumeSource{
							SecretName: appliance.Spec.Secret.Name,
							// The client half of the TLS material stays in the
							// cluster; the appliance is never given it.
							Items: []core.KeyToPath{
								{Key: sshPrivateKeyData, Path: sshPrivateKeyData},
								{Key: announce.CACert, Path: announce.CACert},
								{Key: announce.ServerCert, Path: announce.ServerCert},
								{Key: announce.ServerKey, Path: announce.ServerKey},
							},
						},
					},
				},
			},
			Containers: []core.Container{
				{
					Name:                     "setup",
					Image:                    Settings.SetupImage,
					Command:                  []string{setupBinary},
					TerminationMessagePolicy: core.TerminationMessageFallbackToLogsOnError,
					SecurityContext: &core.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivilegeEscalation,
						Capabilities:             &core.Capabilities{Drop: []core.Capability{"ALL"}},
					},
					Env: []core.EnvVar{
						{Name: settings.CopyApplianceSSHUser, Value: Settings.SSHUser},
						{Name: settings.CopyApplianceSetupAppliance, Value: appliance.Name},
						{Name: settings.CopyApplianceSetupAddress, Value: address},
						{Name: settings.CopyApplianceSetupPullSpec, Value: pullSpec},
						{Name: settings.CopyApplianceSetupTag, Value: tag},
						{Name: settings.CopyApplianceSetupKeyDir, Value: setupKeyDir},
						{Name: settings.CopyApplianceSetupNbdSsl, Value: strconv.FormatBool(r.context.NbdSsl)},
					},
					VolumeMounts: []core.VolumeMount{
						{Name: "appliance-secret", MountPath: setupKeyDir, ReadOnly: true},
					},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse("100m"),
							core.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse("1"),
							core.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}
	// The controller reaches appliance VMs over its own transfer network when
	// one is configured, so the pod has to be on the same one.
	if networks != "" {
		pod.Annotations = map[string]string{base.AnnTransferNetwork: networks}
	}
	// Deleting the CopyAppliance takes the pod with it, so a pod is never left
	// logged into a VM that is being destroyed.
	err = controllerutil.SetControllerReference(appliance, pod, r.client.Scheme())
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = liberr.Wrap(r.client.Create(ctx, pod))
	return
}

// deleteSetupPods removes the appliance's setup pods.
func (r *TeardownRunner) deleteSetupPods(ctx context.Context) (err error) {
	appliance := r.context.Appliance
	err = liberr.Wrap(
		r.client.DeleteAllOf(ctx, &core.Pod{},
			client.InNamespace(appliance.Namespace),
			setupPodSelector(appliance)))
	return
}

// setupFailure reports what a failed setup pod left behind. The worker writes
// the reason to stderr, which TerminationMessageFallbackToLogsOnError turns
// into the container's termination message.
func setupFailure(pod *core.Pod) string {
	for _, status := range pod.Status.ContainerStatuses {
		if terminated := status.State.Terminated; terminated != nil {
			if terminated.Message != "" {
				return terminated.Message
			}
			if terminated.Reason != "" {
				return terminated.Reason
			}
		}
	}
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	return string(pod.Status.Phase)
}

// setupNetworks qualifies a Multus network annotation with the namespace it was
// written for. The value is the controller pod's own annotation, and an
// attachment named in it without a namespace would otherwise be looked for in
// the appliance's namespace, where the controller's attachment is not defined.
func setupNetworks(annotation, namespace string) (value string, err error) {
	annotation = strings.TrimSpace(annotation)
	if annotation == "" {
		return
	}
	if !strings.HasPrefix(annotation, "[") {
		entries := strings.Split(annotation, ",")
		for i, entry := range entries {
			entry = strings.TrimSpace(entry)
			if !strings.Contains(entry, "/") {
				entry = namespace + "/" + entry
			}
			entries[i] = entry
		}
		value = strings.Join(entries, ",")
		return
	}
	var elements []k8snet.NetworkSelectionElement
	err = json.Unmarshal([]byte(annotation), &elements)
	if err != nil {
		err = liberr.Wrap(err, "annotation", annotation)
		return
	}
	for i := range elements {
		if elements[i].Namespace == "" {
			elements[i].Namespace = namespace
		}
	}
	raw, err := json.Marshal(elements)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	value = string(raw)
	return
}
