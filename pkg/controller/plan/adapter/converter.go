package adapter

import (
	"context"
	"fmt"

	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

		scratchPVC, err := c.ensureScratchPVC(pvc)
		if err != nil {
			return false, err
		}

		if scratchPVC == nil {
			c.Log.Info("Scratch PVC is not ready", "pvc", getScratchPVCName(pvc))
			return false, nil
		}

		switch scratchPVC.Status.Phase {
		case v1.ClaimBound:
			c.Log.Info("Scratch PVC bound", "pvc", scratchPVC.Name)
		case v1.ClaimPending:
			c.Log.Info("Scratch PVC pending", "pvc", scratchPVC.Name)
			return false, nil
		case v1.ClaimLost:
			c.Log.Info("Scratch PVC lost", "pvc", scratchPVC.Name)
			return false, liberr.New("scratch pvc lost")
		default:
			c.Log.Info("Scratch PVC status is unknown", "pvc", scratchPVC.Name, "status", scratchPVC.Status.Phase)
			return false, nil
		}

		convertJob, err := c.ensureJob(pvc, srcFormat(pvc), dstFormat)
		if err != nil {
			return false, err
		}

		if convertJob == nil {
			c.Log.Info("Convert job is not ready yet for pvc", "pvc", pvc.Name)
			return false, nil
		}

		c.Log.Info("Convert job status", "pvc", pvc.Name, "status", convertJob.Status)
		for _, condition := range convertJob.Status.Conditions {
			switch condition.Type {
			case batchv1.JobComplete:
				completed++
				c.Log.Info("Convert job completed", "pvc", pvc.Name)

				// Delete scrach PVC
				err = c.Destination.Client.Delete(context.Background(), scratchPVC, &client.DeleteOptions{})
				if err != nil {
					c.Log.Error(err, "Failed to delete scratch PVC", "pvc", scratchPVC.Name)
				}

				return true, nil

			case batchv1.JobFailed:
				if convertJob.Status.Failed >= 3 {
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

func (c *Converter) ensureJob(pvc *v1.PersistentVolumeClaim, srcFormat, dstFormat string) (*batchv1.Job, error) {
	jobName := getJobName(pvc, "convert")
	job := &batchv1.Job{}
	err := c.Destination.Client.Get(context.Background(), client.ObjectKey{Name: jobName, Namespace: pvc.Namespace}, job)
	if err != nil {
		if k8serr.IsNotFound(err) {
			c.Log.Info("Converting PVC format", "pvc", pvc.Name, "srcFormat", srcFormat, "dstFormat", dstFormat)
			job := createConvertJob(pvc, srcFormat, dstFormat, c.Labels)
			err = c.Destination.Client.Create(context.Background(), job, &client.CreateOptions{})
			if err != nil {
				return nil, err
			}
		}
		return nil, err
	}

	return job, err
}

func createConvertJob(pvc *v1.PersistentVolumeClaim, srcFormat, dstFormat string, labels map[string]string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: meta.ObjectMeta{
			Name:      fmt.Sprintf("convert-%s", pvc.Name),
			Namespace: pvc.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(3)),
			Completions:  ptr.To(int32(1)),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					SecurityContext: &v1.PodSecurityContext{
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
						FSGroup: ptr.To(int64(107)),
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
									ClaimName: getScratchPVCName(pvc),
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
		Image: base.Settings.VirtV2vImageCold,
		SecurityContext: &v1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			RunAsNonRoot:             ptr.To(true),
			RunAsUser:                ptr.To(int64(107)),
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

func (c *Converter) ensureScratchPVC(sourcePVC *v1.PersistentVolumeClaim) (*v1.PersistentVolumeClaim, error) {
	scratchPVC := &v1.PersistentVolumeClaim{}
	err := c.Destination.Client.Get(context.Background(), client.ObjectKey{Name: getScratchPVCName(sourcePVC), Namespace: sourcePVC.Namespace}, scratchPVC)
	if err != nil {
		if k8serr.IsNotFound(err) {
			scratchPVC := makeScratchPVC(sourcePVC)
			c.Log.Info("Scratch pvc doesn't exist, creating...", "pvc", sourcePVC.Name)
			err = c.Destination.Client.Create(context.Background(), scratchPVC, &client.CreateOptions{})
		}
		return nil, err
	}

	return scratchPVC, nil
}

func getScratchPVCName(pvc *v1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s-scratch", pvc.Name)
}

func makeScratchPVC(pvc *v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
	size := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	return &v1.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Name:      getScratchPVCName(pvc),
			Namespace: pvc.Namespace,
			Labels:    pvc.Labels,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: pvc.Spec.AccessModes,
			VolumeMode:  pvc.Spec.VolumeMode,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: size,
				},
			},
			StorageClassName: pvc.Spec.StorageClassName,
		},
	}
}

func getJobName(pvc *v1.PersistentVolumeClaim, jobName string) string {
	return fmt.Sprintf("%s-%s", jobName, pvc.Name)
}
