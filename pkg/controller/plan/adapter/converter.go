package adapter

import (
	"context"
	"fmt"

	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type filterFn func(pvc *v1.PersistentVolumeClaim) bool
type srcFormatFn func(pvc *v1.PersistentVolumeClaim) string

type PVCConverter interface {
	ConvertPVCs(pvcs []*v1.PersistentVolumeClaim, srcFormat srcFormatFn, dstFormat string) (ready bool, err error)
}

type Converter struct {
	Destination *plancontext.Destination
	Log         logging.LevelLogger
	Labels      map[string]string
	FilterFn    filterFn
}

func NewConverter(destination *plancontext.Destination, logger logging.LevelLogger, labels map[string]string) *Converter {
	return &Converter{
		Destination: destination,
		Log:         logger,
		Labels:      labels,
	}
}

func (c *Converter) ConvertPVCs(pvcs []*v1.PersistentVolumeClaim, srcFormat srcFormatFn, dstFormat string) (ready bool, err error) {
	completed := 0
	for _, pvc := range pvcs {
		if c.FilterFn != nil && !c.FilterFn(pvc) {
			completed++
			continue
		}

		scratchDV, err := c.ensureScratchDV(pvc)
		if err != nil {
			return false, err
		}

		switch scratchDV.Status.Phase {
		case cdi.ImportScheduled, cdi.Pending:
			c.Log.Info("Scratch DV is not ready", "dv", scratchDV.Name, "status", scratchDV.Status.Phase)
			return false, nil
		case cdi.ImportInProgress:
			c.Log.Info("Scratch DV import in progress", "dv", scratchDV.Name)
			return false, nil
		case cdi.Succeeded:
			c.Log.Info("Scratch DV is ready", "dv", scratchDV.Name)
		default:
			c.Log.Info("Scratch DV is not ready", "dv", scratchDV.Name, "status", scratchDV.Status.Phase)
			return false, nil
		}

		convertJob, err := c.ensureJob(pvc, scratchDV, srcFormat(pvc), dstFormat)
		if err != nil {
			return false, err
		}

		c.Log.Info("Convert job status", "pvc", pvc.Name, "status", convertJob.Status)
		for _, condition := range convertJob.Status.Conditions {
			switch condition.Type {
			case batchv1.JobComplete:
				c.Log.Info("Convert job completed", "pvc", pvc.Name)
				c.deleteScratchDV(scratchDV)
				return true, nil

			case batchv1.JobFailed:
				if convertJob.Status.Failed >= 3 {
					c.deleteScratchDV(scratchDV)
					return true, liberr.New("convert job failed")
				}
			}
		}
	}

	if completed == len(pvcs) {
		return true, nil
	}

	return false, nil
}

func (c *Converter) deleteScratchDV(scratchDV *cdi.DataVolume) {
	err := c.Destination.Client.Delete(context.Background(), scratchDV)
	if err != nil {
		c.Log.Error(err, "Failed to delete scratch DV", "DV", scratchDV.Name)
	}
}

func (c *Converter) ensureJob(pvc *v1.PersistentVolumeClaim, dv *cdi.DataVolume, srcFormat, dstFormat string) (*batchv1.Job, error) {
	// Find existing job by label
	jobList := &batchv1.JobList{}
	label := client.MatchingLabels{planbase.AnnConversionSourcePVC: pvc.Name}
	err := c.Destination.Client.List(context.Background(), jobList, client.InNamespace(pvc.Namespace), label)
	if err != nil {
		return nil, err
	}

	if len(jobList.Items) == 1 {
		c.Log.Info("Found convert job", "job", jobList.Items[0].Name)
		return &jobList.Items[0], nil
	} else if len(jobList.Items) > 1 {
		return nil, liberr.New("multiple convert jobs found for pvc", "pvc", pvc.Name)
	}

	// Job doesn't exist, create it
	job := createConvertJob(pvc, dv, srcFormat, dstFormat, c.Labels)
	c.Log.Info("Creating convert job", "pvc", pvc.Name, "srcFormat", srcFormat, "dstFormat", dstFormat)
	err = c.Destination.Client.Create(context.Background(), job)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func createConvertJob(pvc *v1.PersistentVolumeClaim, dv *cdi.DataVolume, srcFormat, dstFormat string, labels map[string]string) *batchv1.Job {
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[planbase.AnnConversionSourcePVC] = pvc.Name
	return &batchv1.Job{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("convert-%s-", pvc.Name),
			Namespace:    pvc.Namespace,
			Labels:       labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(3)),
			Completions:  ptr.To(int32(1)),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					SecurityContext: &v1.PodSecurityContext{
						// Since we do not have RunAsUser and FSGroup, the pod will fail in the default namespace
						// as it would not be assigned these automatically
						RunAsNonRoot: ptr.To(true),
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						makeConversionContainer(pvc, srcFormat, dstFormat),
					},
					Volumes: []v1.Volume{
						{
							Name: "source",
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvc.Name,
								},
							},
						},
						{
							Name: "target",
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: dv.Name,
								},
							},
						},
					},
				},
			},
		},
	}
}

func makeConversionContainer(pvc *v1.PersistentVolumeClaim, srcFormat, dstFormat string) v1.Container {
	var volumeMode v1.PersistentVolumeMode
	if pvc.Spec.VolumeMode == nil {
		volumeMode = v1.PersistentVolumeFilesystem
	} else {
		volumeMode = *pvc.Spec.VolumeMode
	}
	rawBlock := volumeMode == v1.PersistentVolumeBlock
	var srcPath, dstPath string
	if rawBlock {
		srcPath = "/dev/block"
		dstPath = "/dev/target"
	} else {
		srcPath = "/mnt/disk.img"
		dstPath = "/output/disk.img"
	}

	container := v1.Container{
		Name:  "convert",
		Image: base.Settings.VirtV2vImage,
		SecurityContext: &v1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &v1.Capabilities{
				Drop: []v1.Capability{"ALL"},
			},
		},
		Command: []string{"/usr/local/bin/image-converter"},
		Args: []string{
			"-src-path", srcPath,
			"-dst-path", dstPath,
			"-src-format", srcFormat,
			"-dst-format", dstFormat,
			"-volume-mode", string(volumeMode),
		},
	}

	// Determine source path based on volumeMode
	if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == v1.PersistentVolumeBlock {
		container.VolumeDevices = []v1.VolumeDevice{
			{
				Name:       "source",
				DevicePath: "/dev/block",
			},
			{
				Name:       "target",
				DevicePath: "/dev/target",
			},
		}
	} else {
		container.VolumeMounts = []v1.VolumeMount{
			{
				Name:      "source",
				MountPath: "/mnt/",
			},
			{
				Name:      "target",
				MountPath: "/output/",
			},
		}
	}

	return container
}

func (c *Converter) ensureScratchDV(sourcePVC *v1.PersistentVolumeClaim) (*cdi.DataVolume, error) {
	dvList := &cdi.DataVolumeList{}
	label := client.MatchingLabels{planbase.AnnConversionSourcePVC: sourcePVC.Name}
	err := c.Destination.Client.List(context.Background(), dvList, client.InNamespace(sourcePVC.Namespace), label)
	if err != nil {
		return nil, err
	}

	if len(dvList.Items) == 1 {
		c.Log.Info("Found DV", "dv", dvList.Items[0].Name)
		return &dvList.Items[0], nil
	} else if len(dvList.Items) > 1 {
		return nil, liberr.New("multiple scratch DVs found for pvc", "pvc", sourcePVC.Name)
	}

	// DV doesn't exist, create it
	scratchDV := makeScratchDV(sourcePVC)
	c.Log.Info("DV doesn't exist, creating", "dv", scratchDV.Name)
	err = c.Destination.Client.Create(context.Background(), scratchDV)
	if err != nil {
		return nil, err
	}

	return scratchDV, nil
}

func makeScratchDV(pvc *v1.PersistentVolumeClaim) *cdi.DataVolume {
	size := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	annotations := make(map[string]string)
	annotations[planbase.AnnBindImmediate] = "true"
	annotations["migration"] = pvc.Annotations["migration"]
	annotations["vmID"] = pvc.Annotations["vmID"]

	migration := pvc.Labels["migration"]
	labels := map[string]string{
		"migration":                     migration,
		planbase.AnnConversionSourcePVC: pvc.Name,
	}

	return &cdi.DataVolume{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("scratch-dv-%s-", pvc.Name),
			Namespace:    pvc.Namespace,
			Annotations:  annotations,
			Labels:       labels,
		},
		Spec: cdi.DataVolumeSpec{
			Source: &cdi.DataVolumeSource{
				Blank: &cdi.DataVolumeBlankImage{},
			},
			Storage: &cdi.StorageSpec{
				VolumeMode:       pvc.Spec.VolumeMode,
				AccessModes:      pvc.Spec.AccessModes,
				StorageClassName: pvc.Spec.StorageClassName,
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: size,
					},
				},
			},
		},
	}
}

func getJobName(pvc *v1.PersistentVolumeClaim, jobName string) string {
	return fmt.Sprintf("%s-%s", jobName, pvc.Name)
}
