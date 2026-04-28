package plan

import (
	"cmp"
	"context"
	"encoding/base64"
	"os"
	"path"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/aap"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"gopkg.in/yaml.v2"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Labels
const (
	// VM step label
	kStep = "step"
	// Hook ID label
	kHook = "hook"
)

// Resource label
const (
	ResourceHookConfig = "hook-config"
)

// defaultAAPJobPollSeconds is used when spec.aap.timeout and spec.deadline are 0 and ForkliftController aap_timeout is unset.
const defaultAAPJobPollSeconds int64 = 3600

// Hook runner.
type HookRunner struct {
	*plancontext.Context
	// VM.
	vm *planapi.VMStatus
	// Hook.
	hook *api.Hook
}

// Run.
func (r *HookRunner) Run(vm *planapi.VMStatus) (err error) {
	r.vm = vm
	step, found := vm.FindStep(vm.Phase)
	if !found {
		err = liberr.New("Step not found.")
		return
	}
	if ref, found := vm.FindHook(vm.Phase); found {
		if r.hook, found = r.FindHook(ref.Hook); !found {
			step.Error = &planapi.Error{
				Reasons: []string{"Hook not found."},
				Phase:   step.Phase,
			}
			return
		}
	} else {
		step.MarkedCompleted()
		return
	}

	// Check if this is an AAP job template hook
	if r.hook.Spec.AAP != nil {
		err = r.runAAPJob(step)
		return
	}

	// Standard local playbook execution
	job, err := r.ensureJob()
	if err != nil {
		return
	}
	step.MarkStarted()
	conditions := libcnd.Conditions{}
	for _, cnd := range job.Status.Conditions {
		conditions.SetCondition(libcnd.Condition{
			Type:    string(cnd.Type),
			Status:  string(cnd.Status),
			Reason:  cnd.Reason,
			Message: cnd.Message,
		})
	}
	if conditions.HasCondition("Failed") {
		step.AddError(conditions.FindCondition("Failed").Message)
		step.MarkCompleted()
	} else if int(job.Status.Failed) > Settings.Migration.HookRetry {
		step.AddError("Retry limit exceeded.")
		step.MarkCompleted()
	} else if job.Status.Succeeded > 0 {
		step.Progress.Completed = 1
		step.MarkCompleted()
	}

	return
}

// Ensure the job.
func (r *HookRunner) ensureJob() (job *batch.Job, err error) {
	mp, err := r.ensureConfigMap()
	if err != nil {
		return
	}
	list := batch.JobList{}
	err = r.Client.List(
		context.TODO(),
		&list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.labels()),
			Namespace:     r.Plan.Namespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		job, err = r.job(mp)
		if err != nil {
			return
		}
		err = r.Client.Create(context.TODO(), job)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created (hook) job.",
			"job",
			path.Join(
				job.Namespace,
				job.Name))
	} else {
		job = &list.Items[0]
		r.Log.V(1).Info(
			"Found (hook) job.",
			"job",
			path.Join(
				job.Namespace,
				job.Name))
	}
	err = k8sutil.SetOwnerReference(job, mp, scheme.Scheme)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.Client.Update(context.TODO(), mp)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

// Build the Job.
func (r *HookRunner) job(mp *core.ConfigMap) (job *batch.Job, err error) {
	template := r.template(mp)
	backOff := int32(1)
	job = &batch.Job{
		Spec: batch.JobSpec{
			Template:     *template,
			BackoffLimit: &backOff,
		},
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Plan.Namespace,
			GenerateName: strings.ToLower(
				strings.Join([]string{
					r.Plan.Name,
					r.vm.ID,
					r.vm.Phase},
					"-") + "-"),
			Labels: r.labels(),
		},
	}
	err = k8sutil.SetOwnerReference(r.Plan, job, scheme.Scheme)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

// Build pod template.
func (r *HookRunner) template(mp *core.ConfigMap) (template *core.PodTemplateSpec) {
	template = &core.PodTemplateSpec{
		Spec: core.PodSpec{
			RestartPolicy: core.RestartPolicyNever,
			Containers: []core.Container{
				{
					Name:  "hook",
					Image: r.hook.Spec.Image,
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(Settings.Migration.HooksContainerRequestsCpu),
							core.ResourceMemory: resource.MustParse(Settings.Migration.HooksContainerRequestsMemory),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(Settings.Migration.HooksContainerLimitsCpu),
							core.ResourceMemory: resource.MustParse(Settings.Migration.HooksContainerLimitsMemory),
						},
					},
					VolumeMounts: []core.VolumeMount{
						{
							Name:      "hook",
							MountPath: "/tmp/hook",
						},
					},
				},
			},
			Volumes: []core.Volume{
				{
					Name: "hook",
					VolumeSource: core.VolumeSource{
						ConfigMap: &core.ConfigMapVolumeSource{
							LocalObjectReference: core.LocalObjectReference{
								Name: mp.Name,
							},
						},
					},
				},
			},
		},
	}
	deadline := r.hook.Spec.Deadline
	if deadline > 0 {
		template.Spec.ActiveDeadlineSeconds = &deadline
	}
	// Hook SA > plan SA > global controller SA > namespace default (empty).
	if sa := cmp.Or(r.hook.Spec.ServiceAccount, r.Context.Plan.Spec.ServiceAccount, Settings.Migration.ServiceAccount); sa != "" {
		template.Spec.ServiceAccountName = sa
	}
	if len(r.hook.Spec.Playbook) > 0 {
		container := &template.Spec.Containers[0]
		container.Command = []string{
			"/bin/entrypoint",
			"ansible-runner",
			"run",
			"/tmp/runner",
			"-p",
			"/tmp/hook/playbook.yml",
		}
	}

	return
}

// Ensure the ConfigMap.
func (r *HookRunner) ensureConfigMap() (mp *core.ConfigMap, err error) {
	list := core.ConfigMapList{}
	err = r.Client.List(
		context.TODO(),
		&list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.labels()),
			Namespace:     r.Plan.Namespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) == 0 {
		mp, err = r.configMap()
		if err != nil {
			return
		}
		err = r.Client.Create(context.TODO(), mp)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created (hook) configMap.",
			"map",
			path.Join(
				mp.Namespace,
				mp.Name))
	} else {
		mp = &list.Items[0]
		r.Log.V(1).Info(
			"Found (hook) configMap.",
			"map",
			path.Join(
				mp.Namespace,
				mp.Name))
	}

	return
}

// Job ConfigMap for volume mounts.
func (r *HookRunner) configMap() (mp *core.ConfigMap, err error) {
	workload, err := r.workload()
	if err != nil {
		return
	}
	playbook, err := r.playbook()
	if err != nil {
		return
	}
	plan, err := r.plan()
	if err != nil {
		return
	}
	mp = &core.ConfigMap{
		ObjectMeta: meta.ObjectMeta{
			Labels:    r.labels(),
			Namespace: r.Plan.Namespace,
			GenerateName: strings.ToLower(
				strings.Join([]string{
					r.Plan.Name,
					r.vm.ID,
					r.vm.Phase},
					"-")) + "-",
		},
		Data: map[string]string{
			"workload.yml": workload,
			"playbook.yml": playbook,
			"plan.yml":     plan,
		},
	}

	return
}

// Workload
func (r *HookRunner) workload() (workload string, err error) {
	inventory := r.Source.Inventory
	object, err := inventory.Workload(&r.vm.Ref)
	if err != nil {
		return
	}
	b, err := yaml.Marshal(object)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	workload = string(b)
	return
}

// Decode playbook.
func (r *HookRunner) playbook() (playbook string, err error) {
	encoded := r.hook.Spec.Playbook
	if len(encoded) == 0 {
		return
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	playbook = string(b)
	return
}

// Plan (yaml).
func (r *HookRunner) plan() (plan string, err error) {
	b, err := yaml.Marshal(r.Plan.Spec)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	plan = string(b)
	return
}

// Labels for created resources.
func (r *HookRunner) labels() map[string]string {
	return map[string]string{
		kPlan:      string(r.Plan.UID),
		kMigration: string(r.Migration.UID),
		kVM:        r.vm.ID,
		kStep:      r.vm.Phase,
		kHook:      string(r.hook.UID),
		kResource:  ResourceHookConfig,
	}
}

// aapJobExtraVars builds extra_vars for the AAP job template (extend here when adding new migration context).
func (r *HookRunner) aapJobExtraVars() map[string]string {
	m := map[string]string{
		"vm_id":           r.vm.ID,
		"vm_name":         r.vm.Name,
		"plan_name":       r.Plan.Name,
		"plan_namespace":  r.Plan.Namespace,
		"migration_phase": r.vm.Phase,
	}
	if id := r.vm.Ref.ID; id != "" {
		m["vm_source_id"] = id
	}
	return m
}

// Run AAP job template remotely.
func (r *HookRunner) runAAPJob(step *planapi.Step) (err error) {
	aapConfig := r.hook.Spec.AAP
	m := Settings.Migration

	if aapConfig == nil {
		step.AddError("Hook AAP configuration is missing")
		step.MarkCompleted()
		return
	}

	useHook := strings.TrimSpace(aapConfig.URL) != "" && aapConfig.TokenSecret != nil &&
		strings.TrimSpace(aapConfig.TokenSecret.Name) != ""
	useCluster := strings.TrimSpace(m.AAPURL) != "" && strings.TrimSpace(m.AAPTokenSecretName) != ""
	if !useHook && !useCluster {
		step.AddError("AAP is not configured: set ForkliftController aap_url and aap_token_secret_name, or spec.aap.url and spec.aap.tokenSecret")
		step.MarkCompleted()
		return
	}

	var aapURL string
	var token string
	if useHook {
		aapURL = strings.TrimSpace(aapConfig.URL)
		var tokErr error
		token, tokErr = aap.GetTokenFromSecret(
			context.TODO(),
			r.Client,
			r.Plan.Namespace,
			aapConfig.TokenSecret,
		)
		err = tokErr
	} else {
		// Centralized token Secret lives in the forklift-controller deployment namespace (POD_NAMESPACE).
		aapURL = strings.TrimSpace(m.AAPURL)
		ns := strings.TrimSpace(os.Getenv("POD_NAMESPACE"))
		if ns == "" {
			step.AddError("POD_NAMESPACE is not set; cannot load AAP token Secret")
			step.MarkCompleted()
			return
		}
		var tokErr error
		token, tokErr = aap.GetTokenFromSecretName(
			context.TODO(),
			r.Client,
			ns,
			m.AAPTokenSecretName,
		)
		err = tokErr
	}
	if err != nil {
		step.AddError(err.Error())
		step.MarkCompleted()
		return
	}

	httpTimeout := 30 * time.Second
	if m.AAPTimeoutSeconds > 0 {
		httpTimeout = time.Duration(m.AAPTimeoutSeconds) * time.Second
	}
	aapClient := aap.NewClient(aapURL, token, httpTimeout)

	extraVars := r.aapJobExtraVars()

	r.Log.Info(
		"Launching AAP job template",
		"aap.jobTemplateId", aapConfig.JobTemplateID,
		"aap.url", aapURL,
		"vm.name", r.vm.Name,
		"vm.id", r.vm.ID,
	)

	// Launch the job
	jobID, err := aapClient.LaunchJob(context.TODO(), aapConfig.JobTemplateID, extraVars)
	if err != nil {
		step.AddError(err.Error())
		step.MarkCompleted()
		return
	}

	step.MarkStarted()
	r.Log.Info(
		"AAP job launched successfully",
		"jobId", jobID,
		"jobTemplateId", aapConfig.JobTemplateID,
	)

	// Poll until the AAP job completes. spec.aap.timeout (if set) takes precedence for wall-clock behavior,
	// then spec.deadline, then ForkliftController aap_timeout, then default 3600.
	var pollTimeout time.Duration
	unlimited := false
	switch {
	case aapConfig.Timeout < 0:
		unlimited = true
	case aapConfig.Timeout > 0:
		pollTimeout = time.Duration(aapConfig.Timeout) * time.Second
	case r.hook.Spec.Deadline < 0:
		unlimited = true
	case r.hook.Spec.Deadline > 0:
		pollTimeout = time.Duration(r.hook.Spec.Deadline) * time.Second
	default:
		if m.AAPTimeoutSeconds > 0 {
			pollTimeout = time.Duration(m.AAPTimeoutSeconds) * time.Second
		} else {
			pollTimeout = time.Duration(defaultAAPJobPollSeconds) * time.Second
		}
	}

	// Wait for job completion
	err = aapClient.WaitForJobCompletion(
		context.TODO(),
		jobID,
		pollTimeout,
		unlimited,
	)

	if err != nil {
		r.Log.Error(err, "AAP job failed", "jobId", jobID)
		step.AddError(err.Error())
		step.MarkCompleted()
		return
	}

	r.Log.Info(
		"AAP job completed successfully",
		"jobId", jobID,
	)

	step.Progress.Completed = 1
	step.MarkCompleted()

	return
}
