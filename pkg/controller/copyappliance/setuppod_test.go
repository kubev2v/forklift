package copyappliance

import (
	"context"
	"slices"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/nbd-container/announce"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// setupDeployRunner is a deploy runner for an appliance with an address, backed
// by a cluster holding the given objects. The settings are the ones the setup
// pod is built from.
func setupDeployRunner(t *testing.T, objs ...runtime.Object) *DeployRunner {
	t.Helper()
	applied := testSettings()
	applied.SetupImage = "quay.io/kubev2v/forklift-controller:latest"
	withSettings(t, applied)
	withControllerNamespace(t, "openshift-mtv")

	appliance := testAppliance()
	appliance.Status.Addresses = []api.ApplianceAddress{
		{Network: "VM Network", MAC: "00:50:56:01:02:03", IP: "192.0.2.10"},
	}
	return &DeployRunner{
		context: &ApplianceContext{Appliance: appliance, Log: testLog()},
		client:  testClient(t, objs...),
	}
}

// withControllerNamespace sets the namespace the controller pod runs in, which
// is where the transfer network attachment it is annotated with is defined.
func withControllerNamespace(t *testing.T, namespace string) {
	t.Helper()
	previous := Settings.Namespace
	Settings.Namespace = namespace
	t.Cleanup(func() { Settings.Namespace = previous })
}

// aSetupPod is a setup pod for the appliance in the state the cluster would
// report it in.
func aSetupPod(appliance *api.CopyAppliance, name string, phase core.PodPhase) *core.Pod {
	return &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: appliance.Namespace,
			Labels:    map[string]string{api.LabelCopyApplianceSetup: appliance.Name},
		},
		Status: core.PodStatus{Phase: phase},
	}
}

// onlySetupPod is the one setup pod the runner created.
func onlySetupPod(t *testing.T, r *DeployRunner) *core.Pod {
	t.Helper()
	pod, err := r.setupPod(context.TODO())
	if err != nil {
		t.Fatalf("setupPod: %v", err)
	}
	if pod == nil {
		t.Fatal("no setup pod was created")
	}
	return pod
}

// The pod is the whole of the appliance setup, so every field it needs has to
// be on it before it starts: it has no way to reach the API server afterwards.
func TestCreateSetupPod(t *testing.T) {
	const pullSpec = "quay.io/kubev2v/nbd-container@sha256:abc"
	const tag = "nbd-container:abc"

	t.Run("the pod carries everything the setup needs", func(t *testing.T) {
		r := setupDeployRunner(t)

		if err := r.createSetupPod(context.TODO(), pullSpec, tag); err != nil {
			t.Fatalf("createSetupPod: %v", err)
		}
		pod := onlySetupPod(t, r)
		appliance := r.context.Appliance

		if pod.Namespace != appliance.Namespace {
			t.Errorf("namespace = %q, want the appliance's %q", pod.Namespace, appliance.Namespace)
		}
		if pod.Labels[api.LabelCopyApplianceSetup] != appliance.Name {
			t.Errorf("label = %q, want %q", pod.Labels[api.LabelCopyApplianceSetup], appliance.Name)
		}
		if pod.Spec.RestartPolicy != core.RestartPolicyNever {
			t.Errorf("restart policy = %q, want %q", pod.Spec.RestartPolicy, core.RestartPolicyNever)
		}
		// Without an owner reference a deleted CopyAppliance leaves the pod
		// behind, still logged into a VM that is being destroyed.
		if !meta.IsControlledBy(pod, appliance) {
			t.Errorf("owner references = %+v, want the appliance controlling", pod.OwnerReferences)
		}
		// The pod reads a public image and reaches no API server.
		if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
			t.Errorf("automountServiceAccountToken = %v, want false",
				pod.Spec.AutomountServiceAccountToken)
		}
		if pod.Spec.ServiceAccountName != "" {
			t.Errorf("serviceAccountName = %q, want the namespace default",
				pod.Spec.ServiceAccountName)
		}
		container := pod.Spec.Containers[0]
		if container.Image != Settings.SetupImage {
			t.Errorf("image = %q, want %q", container.Image, Settings.SetupImage)
		}
		// The controller image's own entry point is the controller.
		if !slices.Equal(container.Command, []string{setupBinary}) {
			t.Errorf("command = %v, want %v", container.Command, []string{setupBinary})
		}
		if container.TerminationMessagePolicy != core.TerminationMessageFallbackToLogsOnError {
			t.Errorf("termination message policy = %q, want the fallback to logs",
				container.TerminationMessagePolicy)
		}
		env := map[string]string{}
		for _, variable := range container.Env {
			env[variable.Name] = variable.Value
		}
		for name, want := range map[string]string{
			settings.CopyApplianceSetupAppliance: appliance.Name,
			settings.CopyApplianceSetupAddress:   "192.0.2.10",
			settings.CopyApplianceSetupPullSpec:  pullSpec,
			settings.CopyApplianceSetupTag:       tag,
			settings.CopyApplianceSetupKeyDir:    setupKeyDir,
			settings.CopyApplianceSSHUser:        Settings.SSHUser,
		} {
			if env[name] != want {
				t.Errorf("%s = %q, want %q", name, env[name], want)
			}
		}
		if len(container.VolumeMounts) != 1 || container.VolumeMounts[0].MountPath != setupKeyDir {
			t.Errorf("volume mounts = %+v, want the secret at %q", container.VolumeMounts, setupKeyDir)
		}

		// The client half of the TLS material lets its holder read the
		// appliance's exports. It stays in the cluster.
		secret := pod.Spec.Volumes[0].Secret
		if secret.SecretName != appliance.Spec.Secret.Name {
			t.Errorf("secret = %q, want %q", secret.SecretName, appliance.Spec.Secret.Name)
		}
		var keys []string
		for _, item := range secret.Items {
			keys = append(keys, item.Key)
		}
		want := []string{sshPrivateKeyData, announce.CACert, announce.ServerCert, announce.ServerKey}
		slices.Sort(keys)
		slices.Sort(want)
		if !slices.Equal(keys, want) {
			t.Errorf("projected keys = %v, want %v", keys, want)
		}
	})

	// The controller reaches appliance VMs over the transfer network when one
	// is configured. A pod on the default network cannot reach them at all.
	t.Run("the transfer network is carried onto the pod", func(t *testing.T) {
		r := setupDeployRunner(t)
		applied := Settings.CopyAppliance
		applied.SetupTransferNetwork = "transfer"
		withSettings(t, applied)

		if err := r.createSetupPod(context.TODO(), pullSpec, tag); err != nil {
			t.Fatalf("createSetupPod: %v", err)
		}

		pod := onlySetupPod(t, r)
		want := Settings.Namespace + "/transfer"
		if got := pod.Annotations[base.AnnTransferNetwork]; got != want {
			t.Errorf("%s = %q, want %q", base.AnnTransferNetwork, got, want)
		}
	})

	t.Run("no transfer network leaves the annotation off", func(t *testing.T) {
		r := setupDeployRunner(t)

		if err := r.createSetupPod(context.TODO(), pullSpec, tag); err != nil {
			t.Fatalf("createSetupPod: %v", err)
		}

		pod := onlySetupPod(t, r)
		if _, ok := pod.Annotations[base.AnnTransferNetwork]; ok {
			t.Errorf("annotations = %v, want no transfer network", pod.Annotations)
		}
	})

	// Each of these would otherwise surface as a pod that cannot start or
	// cannot connect, several minutes later and with the reason in a pod the
	// operator has to go and find.
	t.Run("an appliance reporting no address fails naming it", func(t *testing.T) {
		r := setupDeployRunner(t)
		r.context.Appliance.Status.Addresses = nil

		err := r.createSetupPod(context.TODO(), pullSpec, tag)
		if err == nil {
			t.Fatal("createSetupPod succeeded for an appliance with no address")
		}
		if !errorMentions(t, err, r.context.Appliance.Name) {
			t.Errorf("error = %q, want it to name the appliance", err)
		}
	})

	t.Run("no configured setup image fails naming the setting", func(t *testing.T) {
		r := setupDeployRunner(t)
		applied := Settings.CopyAppliance
		applied.SetupImage = ""
		withSettings(t, applied)

		err := r.createSetupPod(context.TODO(), pullSpec, tag)
		if err == nil {
			t.Fatal("createSetupPod succeeded with no setup image")
		}
		if !errorMentions(t, err, settings.CopyApplianceSetupImage) {
			t.Errorf("error = %q, want it to name %q", err, settings.CopyApplianceSetupImage)
		}
	})
}

func TestSetupPod(t *testing.T) {
	t.Run("the appliance's pod is found", func(t *testing.T) {
		appliance := testAppliance()
		r := setupDeployRunner(t, aSetupPod(appliance, "appliance-setup-a", core.PodRunning))

		pod, err := r.setupPod(context.TODO())
		if err != nil {
			t.Fatalf("setupPod: %v", err)
		}
		if pod == nil || pod.Name != "appliance-setup-a" {
			t.Errorf("pod = %+v, want appliance-setup-a", pod)
		}
	})

	// The pods are labelled with the appliance name, and one appliance's pod
	// must not be read as another's.
	t.Run("another appliance's pod is not found", func(t *testing.T) {
		other := testAppliance()
		other.Name = "other-appliance"
		r := setupDeployRunner(t, aSetupPod(other, "other-setup-a", core.PodRunning))

		pod, err := r.setupPod(context.TODO())
		if err != nil {
			t.Fatalf("setupPod: %v", err)
		}
		if pod != nil {
			t.Errorf("pod = %+v, want none", pod)
		}
	})
}

// A teardown that stopped on a delete would leave an undeletable CopyAppliance
// and read locks on the source disks, so the pods go but nothing here may fail.
func TestDeleteSetupPods(t *testing.T) {
	appliance := testAppliance()
	other := testAppliance()
	other.Name = "other-appliance"
	r := &TeardownRunner{
		context: &ApplianceContext{Appliance: appliance, Log: testLog()},
		client: testClient(t,
			aSetupPod(appliance, "appliance-setup-a", core.PodRunning),
			aSetupPod(appliance, "appliance-setup-b", core.PodFailed),
			aSetupPod(other, "other-setup-a", core.PodRunning)),
	}

	if err := r.deleteSetupPods(context.TODO()); err != nil {
		t.Fatalf("deleteSetupPods: %v", err)
	}

	podList := &core.PodList{}
	if err := r.client.List(context.TODO(), podList); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(podList.Items) != 1 || podList.Items[0].Name != "other-setup-a" {
		t.Errorf("pods left = %+v, want only the other appliance's", podList.Items)
	}

	// Nothing is left to delete on the next teardown pass.
	if err := r.deleteSetupPods(context.TODO()); err != nil {
		t.Errorf("deleteSetupPods with nothing to delete: %v", err)
	}
}

// What the worker wrote to stderr is the only account of why the deploy failed,
// and the fallbacks are what is left when the pod did not get that far.
func TestSetupFailure(t *testing.T) {
	terminated := func(state core.ContainerState) *core.Pod {
		return &core.Pod{Status: core.PodStatus{
			Phase:             core.PodFailed,
			ContainerStatuses: []core.ContainerStatus{{State: state}},
		}}
	}
	tests := []struct {
		name string
		pod  *core.Pod
		want string
	}{
		{
			"the termination message",
			terminated(core.ContainerState{
				Terminated: &core.ContainerStateTerminated{
					Reason: "Error", Message: "the appliance is not answering on SSH"},
			}),
			"the appliance is not answering on SSH",
		},
		{
			"the termination reason when there is no message",
			terminated(core.ContainerState{
				Terminated: &core.ContainerStateTerminated{Reason: "OOMKilled"},
			}),
			"OOMKilled",
		},
		{
			"the pod's own message when no container ran",
			&core.Pod{Status: core.PodStatus{
				Phase:   core.PodFailed,
				Message: "Pod was active on the node longer than the specified deadline",
			}},
			"Pod was active on the node longer than the specified deadline",
		},
		{
			"the phase when the pod recorded nothing at all",
			&core.Pod{Status: core.PodStatus{Phase: core.PodFailed}},
			string(core.PodFailed),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := setupFailure(tc.pod); got != tc.want {
				t.Errorf("setupFailure = %q, want %q", got, tc.want)
			}
		})
	}
}

// The annotation is the controller pod's own, read from the controller's
// namespace. An unqualified attachment name in it resolves in the pod's
// namespace, which for a setup pod is the provider's, where the controller's
// attachment is not defined.
func TestSetupNetworks(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		want       string
	}{
		{"no annotation", "", ""},
		{"a bare name is qualified", "transfer", "openshift-mtv/transfer"},
		{"a qualified name is left alone", "other/transfer", "other/transfer"},
		{
			"a list is qualified entry by entry",
			"transfer, other/second",
			"openshift-mtv/transfer,other/second",
		},
		{
			"a JSON element is qualified",
			`[{"name":"transfer"}]`,
			`[{"name":"transfer","namespace":"openshift-mtv"}]`,
		},
		{
			"a JSON element naming a namespace is left alone",
			`[{"name":"transfer","namespace":"other"}]`,
			`[{"name":"transfer","namespace":"other"}]`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := setupNetworks(tc.annotation, "openshift-mtv")
			if err != nil {
				t.Fatalf("setupNetworks: %v", err)
			}
			if got != tc.want {
				t.Errorf("setupNetworks = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("a malformed list fails naming the annotation", func(t *testing.T) {
		_, err := setupNetworks(`[{"name":`, "openshift-mtv")
		if err == nil {
			t.Fatal("setupNetworks succeeded on a malformed annotation")
		}
		if !errorMentions(t, err, `[{"name":`) {
			t.Errorf("error = %q, want it to name the annotation", err)
		}
	})
}
