package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	routev1 "github.com/openshift/api/route/v1"
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

// Env vars
const (
	ProviderNamespace  = "PROVIDER_NAMESPACE"
	ProviderName       = "PROVIDER_NAME"
	CatalogPath        = "CATALOG_PATH"
	ApplianceEndpoints = "APPLIANCE_ENDPOINTS"
	AuthRequired       = "AUTH_REQUIRED"
)

func (r Reconciler) CreateOVAServerDeployment(provider *api.Provider, ctx context.Context) (err error) {
	pvNamePrefix := fmt.Sprintf("%s-pv-%s-%s", ovaServer, provider.Name, provider.Namespace)
	pv, err := r.createPvForNfs(provider, ctx, pvNamePrefix)
	if err != nil {
		err = liberr.Wrap(err)
		r.Log.Error(err, "Failed to create PV for the OVA server")
		return
	}

	ownerReference := metav1.OwnerReference{
		APIVersion: "forklift.konveyor.io/v1beta1",
		Kind:       "Provider",
		Name:       provider.Name,
		UID:        provider.UID,
	}

	pvcNamePrefix := fmt.Sprintf("%s-pvc-%s", ovaServer, provider.Name)
	pvc, err := r.createPvcForNfs(provider, ctx, ownerReference, pv.Name, pvcNamePrefix)
	if err != nil {
		err = liberr.Wrap(err)
		r.Log.Error(err, "Failed to create PVC for the OVA server")
		return
	}

	labels := map[string]string{"provider": provider.Name, "app": "forklift", "subapp": ovaServer}
	err = r.createServerDeployment(provider, ctx, ownerReference, pvc.Name, labels)
	if err != nil {
		err = liberr.Wrap(err)
		r.Log.Error(err, "Failed to create OVA server deployment")
		return
	}

	err = r.createServerService(provider, ctx, ownerReference, labels)
	if err != nil {
		err = liberr.Wrap(err)
		r.Log.Error(err, "Failed to create OVA server service")
		return
	}

	if Settings.OpenShift {
		err = r.createServerRoute(provider, ctx, ownerReference, labels)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(err, "Failed to create OVA server route")
			return
		}
	}
	return
}

func (r *Reconciler) createPvForNfs(provider *api.Provider, ctx context.Context, pvNamePrefix string) (pv *core.PersistentVolume, err error) {
	splitted := strings.Split(provider.Spec.URL, ":")
	if len(splitted) < 2 {
		err = fmt.Errorf("invalid provider URL format: %s", provider.Spec.URL)

		err = liberr.Wrap(err)
		r.Log.Error(err, "Failed to parse NFS server and path from provider URL")
		return nil, err
	}
	nfsServer := splitted[0]
	nfsPath := splitted[1]
	labels := map[string]string{"provider": provider.Name, "app": "forklift", "subapp": ovaServer}

	pv = &core.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: pvNamePrefix,
			Labels:       labels,
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
			PersistentVolumeReclaimPolicy: core.PersistentVolumeReclaimDelete,
		},
	}
	err = r.Create(ctx, pv)
	return
}

func (r *Reconciler) createPvcForNfs(provider *api.Provider, ctx context.Context, ownerReference metav1.OwnerReference, pvName, pvcNamePrefix string) (pvc *core.PersistentVolumeClaim, err error) {
	sc := ""
	labels := map[string]string{"provider": provider.Name, "app": "forklift", "subapp": ovaServer}
	pvc = &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    pvcNamePrefix,
			Namespace:       provider.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
			Labels:          labels,
		},
		Spec: core.PersistentVolumeClaimSpec{
			Resources: core.VolumeResourceRequirements{
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
	return
}

func (r *Reconciler) createServerRoute(provider *api.Provider, ctx context.Context, ownerReference metav1.OwnerReference, labels map[string]string) (err error) {
	routeName := fmt.Sprintf("ova-route-%s", provider.Name)
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:            routeName,
			Namespace:       provider.Namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: routev1.RouteSpec{
			Path: "/appliances",
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: fmt.Sprintf("ova-service-%s", provider.Name),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
	}

	err = r.Create(ctx, route)
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
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceCPU:    resource.MustParse(Settings.OvaContainerRequestsCpu),
				core.ResourceMemory: resource.MustParse(Settings.OvaContainerRequestsMemory),
			},
			Limits: core.ResourceList{
				core.ResourceCPU:    resource.MustParse(Settings.OvaContainerLimitsCpu),
				core.ResourceMemory: resource.MustParse(Settings.OvaContainerRequestsMemory),
			},
		},
		SecurityContext: securityContext,
		Env: []core.EnvVar{
			{
				Name:  ProviderName,
				Value: providerName,
			},
			{
				Name:  ProviderNamespace,
				Value: providerNamespace,
			},
			{
				Name:  CatalogPath,
				Value: mountPath,
			},
			{
				Name:  ApplianceEndpoints,
				Value: "true",
			},
			{
				Name:  AuthRequired,
				Value: "true",
			},
		},
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
