package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ovaServerPrefix     = "ova-server"
	ovaImageVar         = "OVA_PROVIDER_SERVER_IMAGE"
	nfsVolumeNamePrefix = "nfs-volume"
	mountPath           = "/ova"
	pvSize              = "1Gi"
)

func (r Reconciler) CreateOVAServerDeployment(provider *api.Provider, ctx context.Context) {
	ownerReference := metav1.OwnerReference{
		APIVersion: "forklift.konveyor.io/v1beta1",
		Kind:       "Provider",
		Name:       provider.Name,
		UID:        provider.UID,
	}
	pvName := fmt.Sprintf("%s-pv-%s-%s", ovaServerPrefix, provider.Name, provider.Namespace)
	err := r.createPvForNfs(provider, ctx, ownerReference, pvName)
	if err != nil {
		r.Log.Error(err, "Failed to create PV for the OVA server")
		return
	}

	pvcName := fmt.Sprintf("%s-pvc-%s", ovaServerPrefix, provider.Name)
	err = r.createPvcForNfs(provider, ctx, ownerReference, pvName, pvcName)
	if err != nil {
		r.Log.Error(err, "Failed to create PVC for the OVA server")
		return
	}

	labels := map[string]string{"provider": provider.Name, "app": "forklift", "subapp": "ova-server"}
	err = r.createServerDeployment(provider, ctx, ownerReference, pvcName, labels)
	if err != nil {
		r.Log.Error(err, "Failed to create OVA server deployment")
		return
	}

	err = r.createServerService(provider, ctx, ownerReference, labels)
	if err != nil {
		r.Log.Error(err, "Failed to create OVA server service")
		return
	}
}

func (r *Reconciler) createPvForNfs(provider *api.Provider, ctx context.Context, ownerReference metav1.OwnerReference, pvName string) (err error) {
	splitted := strings.Split(provider.Spec.URL, ":")
	nfsServer := splitted[0]
	nfsPath := splitted[1]
	labels := map[string]string{"provider": provider.Name, "app": "forklift", "subapp": "ova-server"}

	pv := &core.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pvName,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
			Labels:          labels,
		},
		Spec: core.PersistentVolumeSpec{
			Capacity: core.ResourceList{
				core.ResourceStorage: resource.MustParse(pvSize),
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			PersistentVolumeSource: core.PersistentVolumeSource{
				NFS: &core.NFSVolumeSource{
					Path:   nfsPath,
					Server: nfsServer,
				},
			},
		},
	}
	err = r.Create(ctx, pv)
	if err != nil {
		return
	}
	return
}

func (r *Reconciler) createPvcForNfs(provider *api.Provider, ctx context.Context, ownerReference metav1.OwnerReference, pvName, pvcName string) (err error) {
	sc := ""
	labels := map[string]string{"providerName": provider.Name, "app": "forklift", "subapp": "ova-server"}
	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pvcName,
			Namespace:       provider.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
			Labels:          labels,
		},
		Spec: core.PersistentVolumeClaimSpec{
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse(pvSize),
				},
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			VolumeName:       pvName,
			StorageClassName: &sc,
		},
	}
	err = r.Create(ctx, pvc)
	if err != nil {
		return
	}
	return
}

func (r *Reconciler) createServerDeployment(provider *api.Provider, ctx context.Context, ownerReference metav1.OwnerReference, pvcName string, labels map[string]string) (err error) {
	deploymentName := fmt.Sprintf("%s-deployment-%s", ovaServerPrefix, provider.Name)
	annotations := make(map[string]string)
	var replicas int32 = 1

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            deploymentName,
			Namespace:       provider.Namespace,
			Annotations:     annotations,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":      "forklift",
					"provider": provider.Name,
					"subapp":   "ova-server",
				},
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: r.makeOvaProviderPodSpec(pvcName, provider.Name),
			},
		},
	}

	err = r.Create(ctx, deployment)
	if err != nil {
		return
	}
	return
}

func (r *Reconciler) createServerService(provider *api.Provider, ctx context.Context, ownerReference metav1.OwnerReference, labels map[string]string) (err error) {
	serviceName := fmt.Sprintf("ova-service-%s", provider.Name)
	service := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			Namespace:       provider.Namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: core.ServiceSpec{
			Selector: labels,
			Ports: []core.ServicePort{
				{
					Name:       "api-http",
					Protocol:   core.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: core.ServiceTypeClusterIP,
		},
	}

	err = r.Create(ctx, service)
	if err != nil {
		return
	}
	return
}

func (r *Reconciler) makeOvaProviderPodSpec(pvcName string, providerName string) core.PodSpec {
	imageName, ok := os.LookupEnv(ovaImageVar)
	if !ok {
		r.Log.Error(nil, "Failed to find OVA server image")
	}

	nfsVolumeName := fmt.Sprintf("%s-%s", nfsVolumeNamePrefix, providerName)
	ovaContainerName := fmt.Sprintf("%s-pod-%s", ovaServerPrefix, providerName)

	return core.PodSpec{
		Containers: []core.Container{
			{
				Name:  ovaContainerName,
				Ports: []core.ContainerPort{{ContainerPort: 8080, Protocol: core.ProtocolTCP}},
				Image: imageName,
				VolumeMounts: []core.VolumeMount{
					{
						Name:      nfsVolumeName,
						MountPath: mountPath,
					},
				},
			},
		},
		Volumes: []core.Volume{
			{
				Name: nfsVolumeName,
				VolumeSource: core.VolumeSource{
					PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			},
		},
	}
}
