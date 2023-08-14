package ocp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strings"

	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	export "kubevirt.io/api/export/v1alpha1"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Network types
const (
	Pod    = "pod"
	Multus = "multus"
)

type Builder struct {
	*plancontext.Context
	macConflictsMap map[string]string
	sourceClient    client.Client
}

// ConfigMap implements base.Builder
func (r *Builder) ConfigMap(vmRef ref.Ref, secret *core.Secret, object *core.ConfigMap) error {
	vmExport := &export.VirtualMachineExport{}
	r.Log.Info("Fetching vmExport", "vmRef", vmRef)

	key := client.ObjectKey{
		Namespace: vmRef.Namespace,
		Name:      vmRef.Name,
	}
	err := r.sourceClient.Get(context.TODO(), key, vmExport)
	if err != nil {
		r.Log.Error(err, "Failed to get VM-export ConfigMap")
		return liberr.Wrap(err)
	}

	object.Data = map[string]string{
		"ca.pem": vmExport.Status.Links.External.Cert,
	}

	return nil
}

// DataVolumes implements base.Builder
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume) (dvs []cdi.DataVolume, err error) {
	vmExport := &export.VirtualMachineExport{}
	key := client.ObjectKey{
		Namespace: vmRef.Namespace,
		Name:      vmRef.Name,
	}

	err = r.sourceClient.Get(context.TODO(), key, vmExport)
	if err != nil {
		r.Log.Error(err, "Failed to get VM-export ConfigMap")
		return nil, liberr.Wrap(err)
	}

	// Build storage map
	storageMap := map[string]v1beta1.DestinationStorage{}
	for _, storage := range r.Map.Storage.Spec.Map {
		storageMap[storage.Source.Name] = storage.Destination
	}

	dataVolumes := []cdi.DataVolume{}
	for _, volume := range vmExport.Status.Links.External.Volumes {
		// Get PVC
		pvc := &core.PersistentVolumeClaim{}
		err = r.sourceClient.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: volume.Name}, pvc)
		if err != nil {
			return nil, liberr.Wrap(err)
		}

		size := pvc.Spec.Resources.Requests["storage"]
		dataVolume := dvTemplate.DeepCopy()
		dataVolume.Annotations[planbase.AnnDiskSource] = fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)

		url := getExportURL(volume.Formats)
		if url == "" {
			return nil, liberr.Wrap(fmt.Errorf("failed to get export URL, available formats: %v", volume.Formats))
		}
		storageClassName := storageMap[*pvc.Spec.StorageClassName].StorageClass
		dataVolume.Spec = *createDataVolumeSpec(size, storageClassName, url, configMap.Name, secret.Name)

		err = r.Destination.Client.Create(context.TODO(), dataVolume, &client.CreateOptions{})
		if err != nil {
			if !k8serr.IsAlreadyExists(err) {
				r.Log.Error(err, "Failed to create DataVolume")
				return nil, liberr.Wrap(err)
			}
		}

		dataVolumes = append(dataVolumes, *dataVolume)
	}

	return dataVolumes, nil
}

func getExportURL(virtualMachineExportVolumeFormat []export.VirtualMachineExportVolumeFormat) (url string) {
	for _, format := range virtualMachineExportVolumeFormat {
		if format.Format == export.KubeVirtGz || format.Format == export.ArchiveGz {
			return format.Url
		}
	}

	return ""
}

// PodEnvironment implements base.Builder
func (*Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error) {
	return nil, nil
}

// ResolveDataVolumeIdentifier implements base.Builder
func (*Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return dv.ObjectMeta.Annotations[planbase.AnnDiskSource]
}

// ResolvePersistentVolumeClaimIdentifier implements base.Builder
func (*Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return ""
}

// Secret implements base.Builder
func (r *Builder) Secret(vmRef ref.Ref, in *core.Secret, object *core.Secret) error {
	vmExport := &export.VirtualMachineExport{}
	err := r.sourceClient.Get(context.Background(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vmExport)
	if err != nil {
		r.Log.Error(err, "Failed to get VM-export Secret")
		return liberr.Wrap(err)
	}

	// Export pod is ready
	// Create config maps with CA on the destination
	// Read secret token
	if vmExport.Status.TokenSecretRef == nil {
		r.Log.Error(err, "Token secret ref is nil")
		return liberr.Wrap(err, "Token secret ref is nil")
	}
	tokenSecret := &core.Secret{}
	err = r.sourceClient.Get(context.Background(), client.ObjectKey{Namespace: vmRef.Namespace, Name: *vmExport.Status.TokenSecretRef}, tokenSecret)
	if err != nil {
		return liberr.Wrap(err)
	}

	// Create secret token header
	object.StringData = map[string]string{
		"token": fmt.Sprintf("x-kubevirt-export-token:%s", tokenSecret.Data["token"]),
	}

	return nil
}

// Tasks implements base.Builder
func (r *Builder) Tasks(vmRef ref.Ref) (list []*planapi.Task, err error) {
	vm := &cnv.VirtualMachine{}
	key := client.ObjectKey{
		Namespace: vmRef.Namespace,
		Name:      vmRef.Name,
	}

	err = r.sourceClient.Get(context.TODO(), key, vm)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	for _, vol := range vm.Spec.Template.Spec.Volumes {
		var size resource.Quantity
		var volName, volNamespace string
		switch {
		case vol.PersistentVolumeClaim != nil:
			pvc := &core.PersistentVolumeClaim{}
			err = r.sourceClient.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vol.PersistentVolumeClaim.ClaimName}, pvc)
			if err != nil {
				return nil, liberr.Wrap(err)
			}
			volName = pvc.Name
			volNamespace = pvc.Namespace
			size = pvc.Spec.Resources.Requests["storage"]
		case vol.DataVolume != nil:
			pvc := &core.PersistentVolumeClaim{}
			err = r.sourceClient.Get(context.TODO(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vol.DataVolume.Name}, pvc)
			if err != nil {
				return nil, liberr.Wrap(err)
			}
			volName = pvc.Name
			volNamespace = pvc.Namespace
			size = pvc.Spec.Resources.Requests["storage"]
		default:
			r.Log.Info("Unsupported volume type", "type", vol.VolumeSource)
			continue
		}

		mB := size.Value() / 1024 / 1024
		list = append(
			list,
			&planapi.Task{
				Name: fmt.Sprintf("%s/%s", volNamespace, volName),
				Progress: libitr.Progress{
					Total: mB,
				},
				Annotations: map[string]string{
					"unit": "MB",
				},
			})
	}

	return
}

// TemplateLabels implements base.Builder
func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	// The VM is build from configuration, we don't need the label
	err = liberr.New("templates are not used by this provider")
	return
}

// VirtualMachine implements base.Builder
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim) error {
	vmExport := &export.VirtualMachineExport{}
	err := r.sourceClient.Get(context.Background(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vmExport)
	if err != nil {
		return liberr.Wrap(err)
	}

	sourceVm, err := r.getSourceVmFromDefinition(vmExport)
	if err != nil {
		return liberr.Wrap(err)
	}

	targetVmSpec := sourceVm.Spec.DeepCopy()
	object.Template = targetVmSpec.Template
	r.mapDisks(sourceVm, targetVmSpec, persistentVolumeClaims, vmRef)
	r.mapNetworks(sourceVm, targetVmSpec)

	return nil
}

func (r *Builder) mapDisks(sourceVm *cnv.VirtualMachine, targetVmSpec *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim, vmRef ref.Ref) {
	pvcMap := make(map[string]*core.PersistentVolumeClaim)
	for i := range persistentVolumeClaims {
		pvc := &persistentVolumeClaims[i]
		if source, ok := pvc.Annotations[planbase.AnnDiskSource]; ok {
			pvcMap[source] = pvc
		}
	}

	diskMap := createDiskMap(sourceVm, pvcMap, vmRef)
	configMaps, secrets := r.createEnvMaps(sourceVm, vmRef)

	// Clear original disks and volumes, will be required for other mapped devices later
	targetVmSpec.Template.Spec.Domain.Devices.Disks = []cnv.Disk{}
	targetVmSpec.Template.Spec.Volumes = []cnv.Volume{}

	r.mapPVCsToTarget(targetVmSpec, persistentVolumeClaims, diskMap)
	r.mapConfigMapsToTarget(targetVmSpec, configMaps, diskMap)
	r.mapSecretsToTarget(targetVmSpec, secrets, diskMap)
}

func createDiskMap(sourceVm *cnv.VirtualMachine, pvcMap map[string]*core.PersistentVolumeClaim, vmRef ref.Ref) map[string]*cnv.Disk {
	diskMap := make(map[string]*cnv.Disk)

	for _, disk := range sourceVm.Spec.Template.Spec.Domain.Devices.Disks {
		currentDisk := disk
		for _, vol := range sourceVm.Spec.Template.Spec.Volumes {
			if vol.Name != disk.Name {
				continue
			}

			var key string
			switch {
			case vol.PersistentVolumeClaim != nil:
				key = pvcSourceName(vmRef.Namespace, vol.PersistentVolumeClaim.ClaimName)
			case vol.DataVolume != nil:
				key = pvcSourceName(vmRef.Namespace, vol.DataVolume.Name)
			case vol.ConfigMap != nil:
				key = vol.ConfigMap.Name
			case vol.Secret != nil:
				key = vol.Secret.SecretName
			}

			diskMap[key] = &currentDisk
			break
		}
	}

	return diskMap
}

func (r *Builder) mapPVCsToTarget(targetVmSpec *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim, diskMap map[string]*cnv.Disk) {
	for _, volume := range persistentVolumeClaims {
		if disk, ok := diskMap[volume.Annotations[planbase.AnnDiskSource]]; ok {
			targetVolume := cnv.Volume{
				Name: disk.Name,
				VolumeSource: cnv.VolumeSource{
					PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
							ClaimName: volume.Name,
						},
					},
				},
			}
			targetVmSpec.Template.Spec.Volumes = append(targetVmSpec.Template.Spec.Volumes, targetVolume)
			targetVmSpec.Template.Spec.Domain.Devices.Disks = append(targetVmSpec.Template.Spec.Domain.Devices.Disks, *disk.DeepCopy())
		}
	}
}

type envMap struct {
	envResource interface{}
	volName     string
}

func (r *Builder) createEnvMaps(sourceVm *cnv.VirtualMachine, vmRef ref.Ref) (map[string]*envMap, map[string]*envMap) {
	configMaps := make(map[string]*envMap)
	secrets := make(map[string]*envMap)

	for _, envVol := range sourceVm.Spec.Template.Spec.Volumes {
		switch {
		case envVol.ConfigMap != nil:
			configMap := &core.ConfigMap{}
			err := r.sourceClient.Get(context.Background(), client.ObjectKey{Namespace: vmRef.Namespace, Name: envVol.ConfigMap.Name}, configMap)
			if err != nil {
				r.Log.Error(err, "Failed to get ConfigMap", "namespace", vmRef.Namespace, "name", envVol.ConfigMap.Name)
				continue
			}
			configMaps[envVol.ConfigMap.Name] = &envMap{
				envResource: configMap,
				volName:     envVol.Name,
			}

		case envVol.Secret != nil:
			secret := &core.Secret{}
			err := r.sourceClient.Get(context.Background(), client.ObjectKey{Namespace: vmRef.Namespace, Name: envVol.Secret.SecretName}, secret)
			if err != nil {
				r.Log.Error(err, "Failed to get Secret", "namespace", vmRef.Namespace, "name", envVol.Secret.SecretName)
				continue
			}
			secrets[envVol.Secret.SecretName] = &envMap{
				envResource: secret,
				volName:     envVol.Name,
			}
		}
	}

	return configMaps, secrets
}

func (r *Builder) mapConfigMapsToTarget(targetVmSpec *cnv.VirtualMachineSpec, configMaps map[string]*envMap, diskMap map[string]*cnv.Disk) {
	for _, configMap := range configMaps {
		// Create configmap on destination cluster
		sourceConfigMap := configMap.envResource.(*core.ConfigMap)
		targetConfigMap := &core.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        sourceConfigMap.Name,
				Namespace:   r.Plan.Spec.TargetNamespace,
				Labels:      sourceConfigMap.Labels,
				Annotations: sourceConfigMap.Annotations,
			},
			Data: sourceConfigMap.Data,
		}
		err := r.Destination.Client.Create(context.Background(), targetConfigMap)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				r.Log.Error(err, "Failed to create ConfigMap", "namespace", r.Plan.Spec.TargetNamespace, "name", targetConfigMap.Name)
				continue
			}
		}

		configMapVolume := cnv.Volume{
			Name: configMap.volName,
			VolumeSource: cnv.VolumeSource{
				ConfigMap: &cnv.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: targetConfigMap.Name,
					},
				},
			},
		}

		if disk, ok := diskMap[sourceConfigMap.Name]; ok {
			targetVmSpec.Template.Spec.Domain.Devices.Disks = append(targetVmSpec.Template.Spec.Domain.Devices.Disks, *disk.DeepCopy())
		} else {
			r.Log.Info("ConfigMap disk not found in diskMap, should never happen", "configMap", sourceConfigMap.Name)
		}

		targetVmSpec.Template.Spec.Volumes = append(targetVmSpec.Template.Spec.Volumes, configMapVolume)
	}
}

func (r *Builder) mapSecretsToTarget(targetVmSpec *cnv.VirtualMachineSpec, secrets map[string]*envMap, diskMap map[string]*cnv.Disk) {
	for _, secret := range secrets {
		// Create secret on destination cluster
		sourceSecret := secret.envResource.(*core.Secret)
		targetSecret := &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        sourceSecret.Name,
				Namespace:   r.Plan.Spec.TargetNamespace,
				Labels:      sourceSecret.Labels,
				Annotations: sourceSecret.Annotations,
			},
			Data: sourceSecret.Data,
		}
		err := r.Destination.Client.Create(context.Background(), targetSecret)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				r.Log.Error(err, "Failed to create Secret", "namespace", r.Plan.Spec.TargetNamespace, "name", targetSecret.Name)
				continue
			}
		}

		secretVolume := cnv.Volume{
			Name: secret.volName,
			VolumeSource: cnv.VolumeSource{
				Secret: &cnv.SecretVolumeSource{
					SecretName: targetSecret.Name,
				},
			},
		}

		if disk, ok := diskMap[sourceSecret.Name]; ok {
			targetVmSpec.Template.Spec.Domain.Devices.Disks = append(targetVmSpec.Template.Spec.Domain.Devices.Disks, *disk.DeepCopy())
		} else {
			r.Log.Info("Secret disk not found in diskMap, should never happen", "secret", sourceSecret.Name)
		}

		targetVmSpec.Template.Spec.Volumes = append(targetVmSpec.Template.Spec.Volumes, secretVolume)
	}
}

func (r *Builder) mapNetworks(sourceVm *cnv.VirtualMachine, targetVmSpec *cnv.VirtualMachineSpec) {
	var networks []cnv.Network
	var interfaces []cnv.Interface

	// Map networks to NICs
	interfacesMap := make(map[string]*cnv.Interface)
	for _, ifc := range sourceVm.Spec.Template.Spec.Domain.Devices.Interfaces {
		currentInterface := ifc
		networkName := ifc.Name
		interfacesMap[networkName] = &currentInterface
	}

	var kInterface *cnv.Interface

	for _, network := range sourceVm.Spec.Template.Spec.Networks {
		targetNetwork := cnv.Network{Name: network.Name}

		kInterface = interfacesMap[network.Name]
		kInterface.Name = network.Name

		switch {
		case network.Multus != nil:
			namespace := strings.Split(network.Multus.NetworkName, "/")[0]
			name := strings.Split(network.Multus.NetworkName, "/")[1]
			pair, found := r.Map.Network.FindNetworkByNameAndNamespace(namespace, name)
			if !found {
				r.Log.Info("Network not found", "namespace", namespace, "name", name)
				continue
			}
			targetNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: fmt.Sprintf("%s/%s", pair.Destination.Namespace, pair.Destination.Name),
			}

		case network.Pod != nil:
			targetNetwork.Pod = &cnv.PodNetwork{}
		default:
			r.Log.Error(nil, "Unknown network type")
			continue
		}

		networks = append(networks, targetNetwork)
		interfaces = append(interfaces, *kInterface)
	}

	targetVmSpec.Template.Spec.Networks = networks
	targetVmSpec.Template.Spec.Domain.Devices.Interfaces = interfaces
}

func (r *Builder) getSourceVmFromDefinition(vme *export.VirtualMachineExport) (*cnv.VirtualMachine, error) {
	var vmManifestUrl string
	for _, manifest := range vme.Status.Links.External.Manifests {
		if manifest.Type == export.AllManifests {
			vmManifestUrl = manifest.Url
			break
		}
	}

	caCert := vme.Status.Links.External.Cert
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	httpClient := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", vmManifestUrl, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create http request")
	}

	req.Header.Set("Accept", "application/json")

	// Read token from secret
	token := &core.Secret{}
	key := client.ObjectKey{Namespace: vme.Namespace, Name: *vme.Status.TokenSecretRef}
	err = r.sourceClient.Get(context.Background(), key, token)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to get token secret")
	}

	req.Header.Set("x-kubevirt-export-token", string(token.Data["token"]))

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to get vm manifest")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, liberr.New("failed to get vm manifest", "status", resp.StatusCode)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to read vm manifest body")
	}

	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode(body, nil, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to decode vm manifest")
	}

	switch t := obj.(type) {
	case *v1.List:
		for _, item := range t.Items {
			decoded, _, err := decode(item.Raw, nil, nil)
			if err != nil {
				return nil, liberr.Wrap(err, "failed to decode vm manifest")
			}

			switch vm := decoded.(type) {
			case *cnv.VirtualMachine:
				r.Log.Info("Found vm in manifest", "vm", vm)
				return vm, nil
			default:
				continue
			}
		}
	}

	return nil, liberr.New("failed to find vm in manifest")
}

func createDataVolumeSpec(size resource.Quantity, storageClassName, url, configMap, secret string) *cdi.DataVolumeSpec {
	return &cdi.DataVolumeSpec{
		Source: &cdi.DataVolumeSource{
			HTTP: &cdi.DataVolumeSourceHTTP{
				URL:                url,
				CertConfigMap:      configMap,
				SecretExtraHeaders: []string{secret},
			},
		},
		Storage: &cdi.StorageSpec{
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: size,
				},
			},
			StorageClassName: &storageClassName,
		},
	}
}

func pvcSourceName(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func (r *Builder) SupportsVolumePopulators() bool {
	return false
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcNames []string, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) PopulatorTransferredBytes(persistentVolumeClaim *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []core.PersistentVolumeClaim) (err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

// Build LUN PVs.
func (r *Builder) LunPersistentVolumes(vmRef ref.Ref) (pvs []core.PersistentVolume, err error) {
	// do nothing
	return
}

// Build LUN PVCs.
func (r *Builder) LunPersistentVolumeClaims(vmRef ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	// do nothing
	return
}
