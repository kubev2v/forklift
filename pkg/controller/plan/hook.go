package plan

import (
	"context"
	"encoding/base64"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"gopkg.in/yaml.v2"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

//
// Hook runner.
type HookRunner struct {
	*plancontext.Context
	// VM.
	vm *planapi.VMStatus
	// Hook.
	hookRef *planapi.HookRef
	// Hook.
	hook *api.Hook
}

//
// Run.
func (r *HookRunner) Run(vm *planapi.VMStatus) (err error) {
	r.vm = vm
	step, found := vm.ActiveStep()
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

//
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
	} else {
		job = &list.Items[0]
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

//
// Build the Job.
func (r *HookRunner) job(mp *core.ConfigMap) (job *batch.Job, err error) {
	template := r.template(mp)
	job = &batch.Job{
		Spec: batch.JobSpec{Template: *template},
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

//
// Build pod template.
func (r *HookRunner) template(mp *core.ConfigMap) (template *core.PodTemplateSpec) {
	template = &core.PodTemplateSpec{
		Spec: core.PodSpec{
			RestartPolicy: "OnFailure",
			Containers: []core.Container{
				{
					Name:  "hook",
					Image: r.hook.Spec.Image,
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
	sa := r.hook.Spec.ServiceAccount
	if len(sa) > 0 {
		template.Spec.ServiceAccountName = sa
	}
	if len(r.hook.Spec.Playbook) > 0 {
		container := &template.Spec.Containers[0]
		container.Command = []string{
			"/bin/entrypoint",
			"ansible-runner",
			"-p",
			"/tmp/hook/playbook.yml",
			"run",
			"/tmp/runner",
		}
	}

	return
}

//
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
	} else {
		mp = &list.Items[0]
	}

	return
}

//
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

//
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

//
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

//
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

//
// Labels for created resources.
func (r *HookRunner) labels() map[string]string {
	return map[string]string{
		"plan":      string(r.Plan.UID),
		"migration": string(r.Plan.UID),
		"step":      r.vm.Phase,
		"vm":        r.vm.ID,
	}
}
