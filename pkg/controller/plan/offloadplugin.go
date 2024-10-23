package plan

import (
	"context"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

const retry int = 5

// OffloadPluginRunner
type OffloadPluginRunner struct {
	*plancontext.Context
	// VM.
	vm       *planapi.VMStatus
	kubevirt KubeVirt
}

// Run.
func (r *OffloadPluginRunner) Run(vm *planapi.VMStatus) (*batch.Job, error) {
	r.vm = vm
	return r.ensureJob()
}

// Ensure the job.
func (r *OffloadPluginRunner) ensureJob() (job *batch.Job, err error) {
	list := batch.JobList{}
	err = r.Destination.Client.List(
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
		job, err = r.job()
		if err != nil {
			return
		}
		err = r.Destination.Client.Create(context.TODO(), job)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created (offload plugin) job.",
			"job",
			path.Join(
				job.Namespace,
				job.Name))
	} else {
		job = &list.Items[0]
		r.Log.V(1).Info(
			"Found (offload plugin) job.",
			"job",
			path.Join(
				job.Namespace,
				job.Name))
	}

	return
}

// Build the Job.
func (r *OffloadPluginRunner) job() (job *batch.Job, err error) {
	secret, err := r.kubevirt.ensureSecret(r.vm.Ref, r.kubevirt.secretDataSetterForCDI(r.vm.Ref), r.labels())
	template := r.template(secret)
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
					r.vm.ID,
					"offloadplugin"},
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

// FIXME: This is just a tmp before we settle on the design in the end we could have multiple maps with multiple images
// we might even have multiple jobs with multiple offload plugins... depends on the mapping and design
func (r *OffloadPluginRunner) getOffloadPluginFromStorageMap() *api.OffloadPlugin {
	for _, storageMap := range r.Context.Plan.Map.Storage.Spec.Map {
		if storageMap.Destination.OffloadPlugin != nil {
			return storageMap.Destination.OffloadPlugin
		}
	}
	return nil
}

// Build pod template.
func (r *OffloadPluginRunner) template(secret *core.Secret) (template *core.PodTemplateSpec) {
	offloadPlugin := r.getOffloadPluginFromStorageMap()
	volumes, mounts := r.getVolumesAndMounts(secret)
	template = &core.PodTemplateSpec{
		Spec: core.PodSpec{
			RestartPolicy: core.RestartPolicyNever,
			Containers: []core.Container{
				{
					Name:         "offloadplugin",
					Image:        offloadPlugin.Image,
					Env:          r.getEnvironments(offloadPlugin),
					VolumeMounts: mounts,
				},
			},
			Volumes: volumes,
		},
	}

	return
}

// Labels for created resources.
func (r *OffloadPluginRunner) labels() map[string]string {
	return map[string]string{
		kPlan:      string(r.Plan.UID),
		kMigration: string(r.Migration.UID),
		kVM:        r.vm.ID,
		kStep:      r.vm.Phase,
	}
}

func (r *OffloadPluginRunner) getEnvironments(offloadPlugin *api.OffloadPlugin) (environments []core.EnvVar) {
	environments = append(environments,
		core.EnvVar{
			Name:  "HOST",
			Value: r.Context.Source.Provider.Spec.URL,
		},
		core.EnvVar{
			Name:  "PLAN_NAME",
			Value: r.Context.Plan.Name,
		},
		core.EnvVar{
			Name:  "NAMESPACE",
			Value: r.Context.Plan.Namespace,
		},
	)
	for key, val := range offloadPlugin.Vars {
		environments = append(environments,
			core.EnvVar{
				Name:  key,
				Value: val,
			})
	}
	return environments
}

func (r *OffloadPluginRunner) getVolumesAndMounts(secret *core.Secret) (volumes []core.Volume, mounts []core.VolumeMount) {
	var secretVolumeName = "secret-volume"
	volumes = append(volumes,
		core.Volume{
			Name: secretVolumeName,
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					SecretName: secret.Name,
				},
			},
		},
	)
	mounts = append(mounts,
		core.VolumeMount{
			Name:      secretVolumeName,
			MountPath: "/etc/secret",
			ReadOnly:  true,
		})
	return volumes, mounts
}
