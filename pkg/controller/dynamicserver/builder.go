package dynamicserver

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/labeler"
	"github.com/kubev2v/forklift/pkg/settings"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var Settings = &settings.Settings

const (
	MainContainer      = "main"
	NFSVolumeMountName = "nfs"
	NFSVolumeMountPath = "/ova"
	QEMUGroup          = 107
	PVSize             = "1Gi"
)

// Labels
const (
	LabelApp            = "app"
	LabelSubapp         = "subapp"
	LabelProvider       = "provider"
	LabelProviderServer = "dynamic-server"
	SubappOVAServer     = "dynamic-server"
	AppForklift         = "forklift"
	// Provider tracking labels
	LabelProviderName      = "forklift.konveyor.io/provider-name"
	LabelProviderNamespace = "forklift.konveyor.io/provider-namespace"
	LabelProviderUID       = "forklift.konveyor.io/provider-uid"
	LabelDynamicProvider   = "forklift.konveyor.io/dynamic-provider"
	LabelStorageName       = "forklift.konveyor.io/storage-name"
)

// Env vars
const (
	ProviderNamespace  = "PROVIDER_NAMESPACE"
	ProviderName       = "PROVIDER_NAME"
	CatalogPath        = "CATALOG_PATH"
	ApplianceEndpoints = "APPLIANCE_ENDPOINTS"
	AuthRequired       = "AUTH_REQUIRED"
)

const (
	SettingApplianceManagement = "applianceManagement"
)

type Labeler struct {
	labeler.Labeler
}

func (r *Labeler) ProviderTypeLabels(providerType string) map[string]string {
	return map[string]string{
		LabelApp:       AppForklift,
		LabelSubapp:    SubappOVAServer,
		"providerType": providerType,
	}
}

func (r *Labeler) ServerLabels(server *api.DynamicProviderServer, providerType string) map[string]string {
	labels := map[string]string{
		LabelApp:            AppForklift,
		LabelSubapp:         SubappOVAServer,
		"providerType":      providerType,
		LabelProviderServer: string(server.UID),
	}

	// Add provider tracking labels from DynamicProviderServer
	if server.Spec.ProviderRef.Name != "" {
		labels[LabelProviderName] = server.Spec.ProviderRef.Name
		labels[LabelProviderNamespace] = server.Spec.ProviderRef.Namespace
	}
	if server.Spec.DynamicProviderRef.Name != "" {
		labels[LabelDynamicProvider] = server.Spec.DynamicProviderRef.Name
	}
	// Add provider UID if available in server labels
	if serverProviderUID, ok := server.Labels[LabelProviderUID]; ok {
		labels[LabelProviderUID] = serverProviderUID
	}

	return labels
}

type Builder struct {
	Server       *api.DynamicProviderServer
	Provider     *api.Provider
	ProviderType string
	Labeler      Labeler
}

func (r *Builder) prefix(providerType string) string {
	return fmt.Sprintf("%s-", providerType)
}

// PersistentVolumeClaims creates PVCs for all storage volumes in the spec.
func (r *Builder) PersistentVolumeClaims() (pvcs []*core.PersistentVolumeClaim) {
	for _, storageSpec := range r.Server.Spec.Storages {
		accessMode := core.ReadWriteOnce
		if storageSpec.AccessMode != nil {
			accessMode = *storageSpec.AccessMode
		}

		volumeMode := core.PersistentVolumeFilesystem
		if storageSpec.VolumeMode != nil {
			volumeMode = *storageSpec.VolumeMode
		}

		// Get server labels and add storage-specific label to differentiate PVCs
		labels := r.Labeler.ServerLabels(r.Server, r.ProviderType)
		labels[LabelStorageName] = storageSpec.Name

		pvc := &core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: r.prefix(r.ProviderType) + storageSpec.Name + "-",
				Labels:       labels,
				Namespace:    Settings.Namespace,
			},
			Spec: core.PersistentVolumeClaimSpec{
				AccessModes: []core.PersistentVolumeAccessMode{
					accessMode,
				},
				VolumeMode: &volumeMode,
				Resources: core.VolumeResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(storageSpec.Size),
					},
				},
			},
		}

		if storageSpec.StorageClass != "" {
			pvc.Spec.StorageClassName = &storageSpec.StorageClass
		}

		pvcs = append(pvcs, pvc)
	}

	return
}

// Deployment builds a deployment for a dynamic provider server.
func (r *Builder) Deployment(pvcs []*core.PersistentVolumeClaim) (deployment *appsv1.Deployment) {
	replicas := int32(1)
	if r.Server.Spec.Replicas != nil {
		replicas = *r.Server.Spec.Replicas
	}

	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(r.ProviderType),
			Namespace:    Settings.Namespace,
			Labels:       r.Labeler.ServerLabels(r.Server, r.ProviderType),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: r.Labeler.ServerLabels(r.Server, r.ProviderType),
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: r.Labeler.ServerLabels(r.Server, r.ProviderType),
				},
				Spec: r.PodSpec(pvcs),
			},
		},
	}
	return
}

func (r *Builder) PodSpec(pvcs []*core.PersistentVolumeClaim) (spec core.PodSpec) {
	containerPort := int32(8080)
	if r.Server.Spec.Port != nil {
		containerPort = *r.Server.Spec.Port
	}

	container := core.Container{
		Name:  MainContainer,
		Image: r.Server.Spec.Image,
		Ports: []core.ContainerPort{
			{ContainerPort: containerPort, Protocol: core.ProtocolTCP},
		},
		SecurityContext: r.securityContext(),
		Env:             r.Server.Spec.Env,
	}

	// Add image pull policy if specified
	if r.Server.Spec.ImagePullPolicy != nil {
		container.ImagePullPolicy = *r.Server.Spec.ImagePullPolicy
	}

	// Add resources if specified
	if r.Server.Spec.Resources != nil {
		container.Resources = *r.Server.Spec.Resources
	}

	var volumeMounts []core.VolumeMount
	var volumes []core.Volume

	// Add volume mounts and volumes for storages (PVCs)
	for i, storageSpec := range r.Server.Spec.Storages {
		if i < len(pvcs) && pvcs[i] != nil {
			volumeName := fmt.Sprintf("storage-%s", storageSpec.Name)
			volumeMounts = append(volumeMounts, core.VolumeMount{
				Name:      volumeName,
				MountPath: storageSpec.MountPath,
			})
			volumes = append(volumes, core.Volume{
				Name: volumeName,
				VolumeSource: core.VolumeSource{
					PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcs[i].Name,
					},
				},
			})
		}
	}

	// Add volume mounts and volumes from existing sources (NFS, PVC, ConfigMap, etc.)
	// These are NOT created by the controller - they're embedded directly in the Pod spec
	for _, volume := range r.Server.Spec.Volumes {
		volumeMounts = append(volumeMounts, core.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.MountPath,
			SubPath:   volume.SubPath,
			ReadOnly:  volume.ReadOnly,
		})
		volumes = append(volumes, core.Volume{
			Name:         volume.Name,
			VolumeSource: volume.VolumeSource,
		})
	}

	// Mount provider credentials secret if it exists
	// Dynamic providers can optionally use secrets for authentication
	if r.Provider != nil && r.Provider.Spec.Secret.Name != "" {
		credentialsVolumeName := "provider-credentials"
		volumeMounts = append(volumeMounts, core.VolumeMount{
			Name:      credentialsVolumeName,
			MountPath: "/etc/forklift/credentials",
			ReadOnly:  true,
		})
		volumes = append(volumes, core.Volume{
			Name: credentialsVolumeName,
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					SecretName: r.Provider.Spec.Secret.Name,
				},
			},
		})

		// Add environment variable to inform the server where credentials are mounted
		container.Env = append(container.Env, core.EnvVar{
			Name:  "PROVIDER_CREDENTIALS_PATH",
			Value: "/etc/forklift/credentials",
		})
	}

	container.VolumeMounts = volumeMounts

	spec = core.PodSpec{
		Containers: []core.Container{container},
		Volumes:    volumes,
	}

	// Add security context if specified
	if r.Server.Spec.SecurityContext != nil {
		spec.SecurityContext = r.Server.Spec.SecurityContext
	}

	// Add image pull secrets if specified
	if len(r.Server.Spec.ImagePullSecrets) > 0 {
		spec.ImagePullSecrets = r.Server.Spec.ImagePullSecrets
	}

	// Add node selector if specified
	if r.Server.Spec.NodeSelector != nil {
		spec.NodeSelector = r.Server.Spec.NodeSelector
	}

	// Add affinity if specified
	if r.Server.Spec.Affinity != nil {
		spec.Affinity = r.Server.Spec.Affinity
	}

	// Add tolerations if specified
	if r.Server.Spec.Tolerations != nil {
		spec.Tolerations = r.Server.Spec.Tolerations
	}

	return
}

func (r *Builder) Service() (svc *core.Service) {
	servicePort := int32(8080)
	if r.Server.Spec.Port != nil {
		servicePort = *r.Server.Spec.Port
	}

	serviceType := core.ServiceTypeClusterIP
	if r.Server.Spec.ServiceType != nil {
		serviceType = *r.Server.Spec.ServiceType
	}

	svc = &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(r.ProviderType),
			Namespace:    Settings.Namespace,
			Labels:       r.Labeler.ServerLabels(r.Server, r.ProviderType),
		},
		Spec: core.ServiceSpec{
			Selector: r.Labeler.ServerLabels(r.Server, r.ProviderType),
			Ports: []core.ServicePort{
				{
					Name:       "api-http",
					Protocol:   core.ProtocolTCP,
					Port:       servicePort,
					TargetPort: intstr.FromInt32(servicePort),
				},
			},
			Type: serviceType,
		},
	}
	return
}

func (r *Builder) securityContext() (sc *core.SecurityContext) {
	sc = &core.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &core.Capabilities{
			Drop: []core.Capability{"ALL"},
		},
		RunAsGroup:   ptr.To(int64(QEMUGroup)),
		RunAsNonRoot: ptr.To(true),
		SeccompProfile: &core.SeccompProfile{
			Type: core.SeccompProfileTypeRuntimeDefault,
		},
	}
	return
}
