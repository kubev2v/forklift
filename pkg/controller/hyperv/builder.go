package hyperv

import (
	"fmt"
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/util"
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
	SMBVolumeMountName = "smb"
	SMBVolumeMountPath = "/hyperv"
	QEMUGroup          = 107
	PVSize             = "1Gi"
	SMBCSIDriver       = "smb.csi.k8s.io"
)

// Labels
const (
	LabelApp            = "app"
	LabelSubapp         = "subapp"
	LabelProvider       = "provider"
	LabelProviderServer = "hyperv-server"
	SubappHyperVServer  = "hyperv-server"
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
	ApplianceManagementEnabled = "ApplianceManagementEnabled"
)

type Labeler struct {
	labeler.Labeler
}

func (r *Labeler) ProviderLabels(provider *api.Provider) map[string]string {
	return map[string]string{
		LabelApp:      AppForklift,
		LabelSubapp:   SubappHyperVServer,
		LabelProvider: string(provider.UID),
	}
}

func (r *Labeler) ServerLabels(provider *api.Provider, server *api.HyperVProviderServer) map[string]string {
	return map[string]string{
		LabelApp:            AppForklift,
		LabelSubapp:         SubappHyperVServer,
		LabelProvider:       string(provider.UID),
		LabelProviderServer: string(server.UID),
	}
}

type Builder struct {
	HyperVProviderServer *api.HyperVProviderServer
	Labeler              Labeler
}

func (r *Builder) prefix(provider *api.Provider) string {
	return fmt.Sprintf("%s-", provider.Name)
}

func (r *Builder) ProviderServer(provider *api.Provider) (server *api.HyperVProviderServer) {
	server = &api.HyperVProviderServer{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Labels:       r.Labeler.ProviderLabels(provider),
			Namespace:    Settings.Namespace,
		},
		Spec: api.HyperVProviderServerSpec{
			Provider: core.ObjectReference{
				Namespace: provider.Namespace,
				Name:      provider.Name,
			},
		},
	}
	return
}

// PersistentVolume builds a static PV for SMB CSI driver.
func (r *Builder) PersistentVolume(provider *api.Provider, secret *core.Secret) (pv *core.PersistentVolume) {

	smbSource := util.ParseSMBSource(provider.Spec.URL)
	secretName := secret.Name
	secretNamespace := secret.Namespace

	pv = &core.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Labels:       r.Labeler.ServerLabels(provider, r.HyperVProviderServer),
		},
		Spec: core.PersistentVolumeSpec{
			Capacity: core.ResourceList{
				core.ResourceStorage: resource.MustParse(PVSize),
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			PersistentVolumeSource: core.PersistentVolumeSource{
				CSI: &core.CSIPersistentVolumeSource{
					Driver:       SMBCSIDriver,
					VolumeHandle: string(provider.UID),
					VolumeAttributes: map[string]string{
						"source": smbSource,
					},
					NodeStageSecretRef: &core.SecretReference{
						Name:      secretName,
						Namespace: secretNamespace,
					},
				},
			},
			PersistentVolumeReclaimPolicy: core.PersistentVolumeReclaimRetain,
		},
	}
	return
}

// PersistentVolumeClaim builds a PVC that binds to the static PV.
func (r *Builder) PersistentVolumeClaim(provider *api.Provider, pv *core.PersistentVolume) (pvc *core.PersistentVolumeClaim) {
	emptyStorageClass := ""
	pvc = &core.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Labels:       r.Labeler.ServerLabels(provider, r.HyperVProviderServer),
			Namespace:    Settings.Namespace,
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			VolumeName:       pv.Name,
			StorageClassName: &emptyStorageClass,
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse(PVSize),
				},
			},
		},
	}
	return
}

// Deployment builds a deployment for a HyperV provider server. HyperV provider servers are deployed in
// Forklift's namespace, so they will not have an owner reference to the parent Provider CR
// unless the Provider is created in Forklift's namespace.
// (Owner references cannot point to cross-namespace resources.)
func (r *Builder) Deployment(provider *api.Provider, pvc *core.PersistentVolumeClaim) (deployment *appsv1.Deployment) {
	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: r.prefix(provider),
			Namespace:    Settings.Namespace,
			Labels:       r.Labeler.ServerLabels(provider, r.HyperVProviderServer),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: r.Labeler.ServerLabels(provider, r.HyperVProviderServer),
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: r.Labeler.ServerLabels(provider, r.HyperVProviderServer),
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
						core.ResourceCPU:    resource.MustParse(Settings.Providers.HyperV.Resources.CPU.Request),
						core.ResourceMemory: resource.MustParse(Settings.Providers.HyperV.Resources.Memory.Request),
					},
					Limits: core.ResourceList{
						core.ResourceCPU:    resource.MustParse(Settings.Providers.HyperV.Resources.CPU.Limit),
						core.ResourceMemory: resource.MustParse(Settings.Providers.HyperV.Resources.Memory.Limit),
					},
				},
				SecurityContext: r.securityContext(),
				VolumeMounts: []core.VolumeMount{
					{
						Name:      SMBVolumeMountName,
						MountPath: SMBVolumeMountPath,
						ReadOnly:  true,
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
						Value: r.smbMountPath(),
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
				Name: SMBVolumeMountName,
				VolumeSource: core.VolumeSource{
					PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
						ReadOnly:  true,
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
			Labels:       r.Labeler.ServerLabels(provider, r.HyperVProviderServer),
		},
		Spec: core.ServiceSpec{
			Selector: r.Labeler.ServerLabels(provider, r.HyperVProviderServer),
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
	gateEnabled := Settings.Features.OVFApplianceManagement
	providerEnabled, _ := strconv.ParseBool(provider.Spec.Settings[SettingApplianceManagement])
	return strconv.FormatBool(gateEnabled && providerEnabled)
}

func (r *Builder) containerImage() string {
	return Settings.Providers.HyperV.ContainerImage
}

func (r *Builder) smbMountPath() string {
	return SMBVolumeMountPath
}
