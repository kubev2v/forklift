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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ovaServer              = "ova-server"
	ovaImageVar            = "OVA_PROVIDER_SERVER_IMAGE"
	nfsVolumeNamePrefix    = "nfs-volume"
	mountPath              = "/ova"
	pvSize                 = "1Gi"
	auditRestrictedLabel   = "pod-security.kubernetes.io/audit"
	enforceRestrictedLabel = "pod-security.kubernetes.io/enforce"
	qemuGroup              = 107
)

func (r Reconciler) CreateOVAServerDeployment(provider *api.Provider, ctx context.Context) {
	pvName := fmt.Sprintf("%s-pv-%s-%s", ovaServer, provider.Name, provider.Namespace)
	err := r.createPvForNfs(provider, ctx, pvName)
	if err != nil {
		r.Log.Error(err, "Failed to create PV for the OVA server")
		return
	}

	ownerReference := metav1.OwnerReference{
		APIVersion: "forklift.konveyor.io/v1beta1",
		Kind:       "Provider",
		Name:       provider.Name,
		UID:        provider.UID,
	}
	pvcName := fmt.Sprintf("%s-pvc-%s", ovaServer, provider.Name)
	err = r.createPvcForNfs(provider, ctx, ownerReference, pvName, pvcName)
	if err != nil {
		r.Log.Error(err, "Failed to create PVC for the OVA server")
		return
	}

	labels := map[string]string{"provider": provider.Name, "app": "forklift", "subapp": ovaServer}
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

func (r *Reconciler) createPvForNfs(provider *api.Provider, ctx context.Context, pvName string) (err error) {
	splitted := strings.Split(provider.Spec.URL, ":")
	nfsServer := splitted[0]
	nfsPath := splitted[1]
	labels := map[string]string{"provider": provider.Name, "app": "forklift", "subapp": ovaServer}

	pv := &core.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   pvName,
			Labels: labels,
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
	labels := map[string]string{"provider": provider.Name, "app": "forklift", "subapp": ovaServer}
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
	deploymentName := fmt.Sprintf("%s-deployment-%s", ovaServer, provider.Name)
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
				MatchLabels: labels,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: r.makeOvaProviderPodSpec(pvcName, provider.Name, provider.Namespace),
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

func (r *Reconciler) makeOvaProviderPodSpec(pvcName, providerName, providerNamespace string) core.PodSpec {
	imageName, ok := os.LookupEnv(ovaImageVar)
	if !ok {
		r.Log.Info("Failed to find OVA server image")
		return core.PodSpec{}
	}

	nfsVolumeName := fmt.Sprintf("%s-%s", nfsVolumeNamePrefix, providerName)
	ovaContainerName := fmt.Sprintf("%s-pod-%s", ovaServer, providerName)
	nonRoot := true
	user := int64(qemuGroup)
	allowPrivilegeEscalation := false

	securityContext := &core.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		Capabilities: &core.Capabilities{
			Drop: []core.Capability{"ALL"},
		},
	}

	restricted := r.isEnforcedRestrictionNamespace(providerNamespace)
	if restricted {
		seccompProfile := &core.SeccompProfile{
			Type: core.SeccompProfileTypeRuntimeDefault,
		}
		securityContext.RunAsUser = &user
		securityContext.RunAsNonRoot = &nonRoot
		securityContext.SeccompProfile = seccompProfile
	}

	container := core.Container{
		Name:  ovaContainerName,
		Image: imageName,
		Ports: []core.ContainerPort{{ContainerPort: 8080, Protocol: core.ProtocolTCP}},
		VolumeMounts: []core.VolumeMount{
			{
				Name:      nfsVolumeName,
				MountPath: mountPath,
			},
		},
		SecurityContext: securityContext,
	}

	volume := core.Volume{
		Name: nfsVolumeName,
		VolumeSource: core.VolumeSource{
			PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}

	podSpec := core.PodSpec{
		Containers: []core.Container{container},
		Volumes:    []core.Volume{volume},
	}
	return podSpec
}

func (r *Reconciler) isEnforcedRestrictionNamespace(namespaceName string) bool {
	ns := core.Namespace{}
	err := r.Get(context.TODO(), client.ObjectKey{Name: namespaceName}, &ns)
	if err != nil {
		r.Log.Error(err, "Error getting namespace for restriction check")
		return false
	}

	enforceLabel, enforceExists := ns.Labels[enforceRestrictedLabel]
	auditLabel, auditExists := ns.Labels[auditRestrictedLabel]

	return enforceExists && enforceLabel == "restricted" && !(auditExists && auditLabel == "restricted")
}
