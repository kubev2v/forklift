package ova

import (
	"fmt"
	"strconv"
	"strings"

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
	LabelProviderServer = "ova-server"
	SubappOVAServer     = "ova-server"
	AppForklift         = "forklift"
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

func (r *Labeler) ProviderLabels(provider *api.Provider) map[string]string {
	return map[string]string{
		LabelApp:      AppForklift,
		LabelSubapp:   SubappOVAServer,
		LabelProvider: string(provider.UID),
	}
}

func (r *Labeler) ServerLabels(provider *api.Provider, server *api.OVAProviderServer) map[string]string {
	return map[string]string{
		LabelApp:            AppForklift,
		LabelSubapp:         SubappOVAServer,
		LabelProvider:       string(provider.UID),
		LabelProviderServer: string(server.UID),
	}
}

type Builder struct {
	OVAProviderServer *api.OVAProviderServer
	Labeler           Labeler
}

func (r *Builder) prefix(provider *api.Provider) string {
	return fmt.Sprintf("%s-", provider.Name)
}

func (r *Builder) ProviderServer(provider *api.Provider) (server *api.OVAProviderServer) {
	server = &api.OVAProviderServer{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Labels:       r.Labeler.ProviderLabels(provider),
			Namespace:    Settings.Namespace,
		},
		Spec: api.OVAProviderServerSpec{
			Provider: core.ObjectReference{
				Namespace: provider.Namespace,
				Name:      provider.Name,
			},
		},
	}
	return
}

func (r *Builder) PersistentVolume(provider *api.Provider) (pv *core.PersistentVolume) {
	segments := strings.Split(provider.Spec.URL, ":")
	pv = &core.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Labels:       r.Labeler.ServerLabels(provider, r.OVAProviderServer),
		},
		Spec: core.PersistentVolumeSpec{
			Capacity: core.ResourceList{
				core.ResourceStorage: resource.MustParse(PVSize),
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			PersistentVolumeSource: core.PersistentVolumeSource{
				NFS: &core.NFSVolumeSource{
					Server: segments[0],
					Path:   segments[1],
				},
			},
			PersistentVolumeReclaimPolicy: core.PersistentVolumeReclaimRetain,
		},
	}
	return
}

func (r *Builder) PersistentVolumeClaim(provider *api.Provider, pv *core.PersistentVolume) (pvc *core.PersistentVolumeClaim) {
	pvc = &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Labels:       r.Labeler.ServerLabels(provider, r.OVAProviderServer),
			Namespace:    Settings.Namespace,
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			VolumeName:       pv.Name,
			StorageClassName: &pv.Spec.StorageClassName,
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse(PVSize),
				},
			},
		},
	}
	return
}

// Deployment builds a deployment for an OVA provider server. OVA provider servers are now deployed in
// Forklift's namespace, so they will not have an owner reference to the parent Provider CR
// unless the Provider is created in Forklift's namespace.
// (Owner references cannot point to cross-namespace resources.)
func (r *Builder) Deployment(provider *api.Provider, pvc *core.PersistentVolumeClaim) (deployment *appsv1.Deployment) {
	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Namespace:    Settings.Namespace,
			Labels:       r.Labeler.ServerLabels(provider, r.OVAProviderServer),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: r.Labeler.ServerLabels(provider, r.OVAProviderServer),
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: r.Labeler.ServerLabels(provider, r.OVAProviderServer),
				},
				Spec: r.PodSpec(provider, pvc),
			},
		},
	}
	return
}

func (r *Builder) PodSpec(provider *api.Provider, pvc *core.PersistentVolumeClaim) (spec core.PodSpec) {
	spec = core.PodSpec{
		Containers: []core.Container{
			{
				Name:  MainContainer,
				Image: r.containerImage(),
				Ports: []core.ContainerPort{
					{ContainerPort: 8080, Protocol: core.ProtocolTCP},
				},
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceCPU:    resource.MustParse(Settings.OVA.Pod.Resources.CPU.Request),
						core.ResourceMemory: resource.MustParse(Settings.OVA.Pod.Resources.Memory.Request),
					},
					Limits: core.ResourceList{
						core.ResourceCPU:    resource.MustParse(Settings.OVA.Pod.Resources.CPU.Limit),
						core.ResourceMemory: resource.MustParse(Settings.OVA.Pod.Resources.Memory.Limit),
					},
				},
				SecurityContext: r.securityContext(),
				VolumeMounts: []core.VolumeMount{
					{
						Name:      NFSVolumeMountName,
						MountPath: NFSVolumeMountPath,
					},
				},
				Env: []core.EnvVar{
					{
						Name:  ProviderName,
						Value: provider.Name,
					},
					{
						Name:  ProviderNamespace,
						Value: provider.Namespace,
					},
					{
						Name:  CatalogPath,
						Value: r.nfsMountPath(),
					},
					{
						Name:  ApplianceEndpoints,
						Value: r.applianceEndpoints(provider),
					},
					{
						Name:  AuthRequired,
						Value: "true",
					},
				},
			},
		},
		Volumes: []core.Volume{
			{
				Name: NFSVolumeMountName,
				VolumeSource: core.VolumeSource{
					PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		},
	}
	return
}

func (r *Builder) Service(provider *api.Provider) (svc *core.Service) {
	svc = &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Namespace:    Settings.Namespace,
			Labels:       r.Labeler.ServerLabels(provider, r.OVAProviderServer),
		},
		Spec: core.ServiceSpec{
			Selector: r.Labeler.ServerLabels(provider, r.OVAProviderServer),
			Ports: []core.ServicePort{
				{
					Name:       "api-http",
					Protocol:   core.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromInt32(8080),
				},
			},
			Type: core.ServiceTypeClusterIP,
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

func (r *Builder) applianceEndpoints(provider *api.Provider) string {
	gateEnabled := Settings.Features.OVAApplianceManagement
	providerEnabled, _ := strconv.ParseBool(provider.Spec.Settings[SettingApplianceManagement])
	return strconv.FormatBool(gateEnabled && providerEnabled)
}

func (r *Builder) containerImage() string {
	return Settings.Providers.OVA.Pod.ContainerImage
}

func (r *Builder) nfsMountPath() string {
	return NFSVolumeMountPath
}
