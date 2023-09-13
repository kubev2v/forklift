package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
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

func (r *Reconciler) CreateOVAServerDeployment(provider *api.Provider, ctx context.Context) {

	ownerReference := metav1.OwnerReference{
		APIVersion: "forklift.konveyor.io/v1beta1",
		Kind:       "Provider",
		Name:       provider.Name,
		UID:        provider.UID,
	}

	pvName := fmt.Sprintf("%s-pv-%s", ovaServerPrefix, provider.Name)
	splitted := strings.Split(provider.Spec.URL, ":")

	if len(splitted) != 2 {
		r.Log.Error(nil, "NFS server path doesn't contains :")
	}
	nfsServer := splitted[0]
	nfsPath := splitted[1]

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pvName,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				v1.ResourceStorage: resource.MustParse("1Gi"),
			},
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadOnlyMany,
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				NFS: &v1.NFSVolumeSource{
					Path:   nfsPath,
					Server: nfsServer,
				},
			},
		},
	}
	err := r.Create(ctx, pv)
	if err != nil {
		r.Log.Error(err, "Failed to create OVA server PV")
		return
	}

	pvcName := fmt.Sprintf("%s-pvc-%s", ovaServerPrefix, provider.Name)
	sc := ""
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pvcName,
			Namespace:       provider.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadOnlyMany,
			},
			VolumeName:       pvName,
			StorageClassName: &sc,
		},
	}
	err = r.Create(ctx, pvc)
	if err != nil {
		r.Log.Error(err, "Failed to create OVA server PVC")
		return
	}

	deploymentName := fmt.Sprintf("%s-deployment-%s", ovaServerPrefix, provider.Name)
	annotations := make(map[string]string)
	labels := map[string]string{"providerName": provider.Name, "app": "forklift"}
	var replicas int32 = 1

	//OVA server deployment
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
					"app": "forklift",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"providerName": provider.Name,
						"app":          "forklift",
					},
				},
				Spec: r.makeOvaProviderPodSpec(pvcName, string(provider.Name)),
			},
		},
	}

	err = r.Create(ctx, deployment)
	if err != nil {
		r.Log.Error(err, "Failed to create OVA server deployment")
		return
	}

	// OVA Server Service
	serviceName := fmt.Sprintf("ova-service-%s", provider.Name)
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceName,
			Namespace:       provider.Namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"providerName": provider.Name,
				"app":          "forklift",
			},
			Ports: []v1.ServicePort{
				{
					Name:       "api-http",
					Protocol:   v1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Type: v1.ServiceTypeClusterIP,
		},
	}

	err = r.Create(ctx, service)
	if err != nil {
		r.Log.Error(err, "Failed to create OVA server service")
		return
	}
}

func (r *Reconciler) makeOvaProviderPodSpec(pvcName string, providerName string) v1.PodSpec {

	imageName, ok := os.LookupEnv(ovaImageVar)
	if !ok {
		r.Log.Error(nil, "Failed to find OVA server image")
	}

	nfsVolumeName := fmt.Sprintf("%s-%s", nfsVolumeNamePrefix, providerName)

	ovaContainerName := fmt.Sprintf("%s-pod-%s", ovaServerPrefix, providerName)

	return v1.PodSpec{

		Containers: []v1.Container{
			{
				Name:  ovaContainerName,
				Ports: []v1.ContainerPort{{ContainerPort: 8080, Protocol: v1.ProtocolTCP}},
				Image: imageName,
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      nfsVolumeName,
						MountPath: "/ova",
					},
				},
			},
		},
		Volumes: []v1.Volume{
			{
				Name: nfsVolumeName,
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			},
		},
	}
}
