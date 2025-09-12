package ocp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	export "kubevirt.io/api/export/v1alpha1"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ocpclient "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ensure KubeVirt types are known to client-go's global scheme
func init() {
	_ = cnv.AddToScheme(scheme.Scheme)
}

// Network types
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

type Builder struct {
	*plancontext.Context
	sourceClient client.Client
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

	links := vmExport.Status.Links
	if links.External != nil {
		object.Data = map[string]string{
			"ca.pem": links.External.Cert,
		}
	} else {
		return liberr.Wrap(fmt.Errorf("failed to get external link from VM-exports"))
	}

	return nil
}

// DataVolumes implements base.Builder
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
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
			if !errors.IsAlreadyExists(err) {
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

func (r *Builder) PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (name string, err error) {
	// The VM is built from configuration, we don't need the preference
	err = liberr.New("preferences are not used by this provider")
	return
}

// TemplateLabels implements base.Builder
func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	// The VM is build from configuration, we don't need the label
	err = liberr.New("templates are not used by this provider")
	return
}

// VirtualMachine implements base.Builder
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
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

func (r *Builder) mapDisks(sourceVm *cnv.VirtualMachine, targetVmSpec *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, vmRef ref.Ref) {
	diskMap := createDiskMap(sourceVm, vmRef)
	configMaps, secrets := r.createEnvMaps(sourceVm, vmRef)

	// Clear original disks and volumes, will be required for other mapped devices later
	targetVmSpec.Template.Spec.Domain.Devices.Disks = []cnv.Disk{}
	targetVmSpec.Template.Spec.Volumes = []cnv.Volume{}

	r.mapPVCsToTarget(targetVmSpec, persistentVolumeClaims, diskMap)
	r.mapConfigMapsToTarget(targetVmSpec, configMaps)
	r.mapSecretsToTarget(targetVmSpec, secrets)
	r.mapDeviceDisks(targetVmSpec, sourceVm, diskMap)
}

// FIXME: The map does not contain all possible disk configuration
// We should go through the missing and implement them or warn around them
func (r *Builder) isDiskInDiskMap(disk *cnv.Disk, diskMap map[string]*cnv.Disk) bool {
	for _, val := range diskMap {
		if disk.Name == val.Name {
			return true
		}
	}
	return false
}

func (r *Builder) mapDeviceDisks(targetVmSpec *cnv.VirtualMachineSpec, sourceVm *cnv.VirtualMachine, diskMap map[string]*cnv.Disk) {
	for _, disk := range sourceVm.Spec.Template.Spec.Domain.Devices.Disks {
		if r.isDiskInDiskMap(&disk, diskMap) {
			targetVmSpec.Template.Spec.Domain.Devices.Disks = append(targetVmSpec.Template.Spec.Domain.Devices.Disks, *disk.DeepCopy())
		}
	}
}

func createDiskMap(sourceVm *cnv.VirtualMachine, vmRef ref.Ref) map[string]*cnv.Disk {
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

func (r *Builder) mapPVCsToTarget(targetVmSpec *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, diskMap map[string]*cnv.Disk) {
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

func (r *Builder) mapConfigMapsToTarget(targetVmSpec *cnv.VirtualMachineSpec, configMaps map[string]*envMap) {
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

		targetVmSpec.Template.Spec.Volumes = append(targetVmSpec.Template.Spec.Volumes, configMapVolume)
	}
}

func (r *Builder) mapSecretsToTarget(targetVmSpec *cnv.VirtualMachineSpec, secrets map[string]*envMap) {
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
			name, namespace := ocpclient.GetNetworkNameAndNamespace(network.Multus.NetworkName, &ref.Ref{Name: sourceVm.Name, Namespace: sourceVm.Namespace})
			pair, found := r.Map.Network.FindNetworkByNameAndNamespace(namespace, name)
			if !found {
				r.Log.Info("Network not found", "namespace", namespace, "name", name)
				continue
			}

			// Check if the network should be ignored
			if pair.Destination.Type == Ignored {
				r.Log.Info("Network is ignored", "namespace", namespace, "name", name)
				continue
			}

			// Check if the network is mapped to the pod network
			if pair.Destination.Type == Pod {
				targetNetwork.Pod = &cnv.PodNetwork{}
				continue
			}

			targetNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: fmt.Sprintf("%s/%s", pair.Destination.Namespace, pair.Destination.Name),
			}

		case network.Pod != nil:
			pair, found := r.Map.Network.FindNetworkByType(Pod)
			if !found {
				r.Log.Info("Network not found", "type", Pod)
				continue
			}

			// Check if the network should be ignored
			if pair.Destination.Type == Ignored {
				r.Log.Info("Network is ignored", "type", Pod)
				continue
			}

			// Check if the network is mapped to a multus network
			if pair.Destination.Type == Multus {
				targetNetwork.Multus = &cnv.MultusNetwork{
					NetworkName: fmt.Sprintf("%s/%s", pair.Destination.Namespace, pair.Destination.Name),
				}
				continue
			}

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

// findVMInManifestItems extracts the common logic for processing manifest items
// regardless of whether they come from metav1.List or core.List.
// Skips items that fail to decode and continues scanning. Returns (vm, nil) if found,
// or (nil, error) only if no VirtualMachine is found after scanning all items.
func (r *Builder) findVMInManifestItems(items []runtime.RawExtension, decode func([]byte, *schema.GroupVersionKind, runtime.Object) (runtime.Object, *schema.GroupVersionKind, error)) (*cnv.VirtualMachine, error) {
	for i, item := range items {
		decoded, gvk, err := decode(item.Raw, nil, nil)
		if err != nil {
			r.Log.V(1).Info("Skipping manifest item: decode failed", "index", i, "error", err)
			continue
		}

		r.Log.Info("Decoded manifest item", "index", i, "kind", gvk.Kind, "type", fmt.Sprintf("%T", decoded))

		if vm, ok := decoded.(*cnv.VirtualMachine); ok {
			r.Log.Info("Found vm in manifest", "vm.name", vm.Name, "vm.namespace", vm.Namespace)
			return vm, nil
		}
	}
	return nil, liberr.New("no VirtualMachine found in manifest items")
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
	var transport *http.Transport

	if caCert != "" {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(caCert)) {
			return nil, liberr.New("failed to parse CA certificate")
		}

		tlsConfig := &tls.Config{
			RootCAs: caCertPool,
		}

		transport = &http.Transport{TLSClientConfig: tlsConfig}

	} else {
		r.Log.Info("Certificate from VM export is empty, using system CA certificates")
		transport = &http.Transport{}
	}

	httpClient := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", vmManifestUrl, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to create http request")
	}

	req.Header.Set("Accept", "application/json")

	// Read token from secret
	if vme.Status.TokenSecretRef == nil {
		return nil, liberr.New("token secret reference is nil")
	}
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

	// Log the raw manifest content for debugging
	r.Log.Info("Retrieved manifest content", "size", len(body), "url", vmManifestUrl)
	r.Log.V(1).Info("Manifest body", "content", string(body))

	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode(body, nil, nil)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to decode vm manifest")
	}

	// Handle different manifest formats returned by VirtualMachineExport
	// We need to support multiple List types because different Kubernetes/OpenShift versions
	// and API configurations may return different serialization formats:
	//
	// *metav1.List: From k8s.io/apimachinery/pkg/apis/meta/v1 - used by older K8s versions
	//               or when the API server uses meta API serialization
	// *core.List:   From k8s.io/api/core/v1 - used by newer K8s versions or when the
	//               API server uses core API serialization
	//
	// Both types have identical structure (Items []runtime.RawExtension) but are different
	// Go types, so we must handle each explicitly for type safety.
	//
	// Note: We cannot use a single *v1.List case because metav1.List and core.List are
	// distinct types from different packages, even though they have the same structure.
	switch t := obj.(type) {
	case *cnv.VirtualMachine:
		return t, nil
	case *metav1.List:
		r.Log.Info("Found metav1.List in manifest", "itemCount", len(t.Items))
		vm, err := r.findVMInManifestItems(t.Items, decode)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to decode vm manifest")
		}
		return vm, nil
	case *core.List:
		r.Log.Info("Found core.List in manifest", "itemCount", len(t.Items))
		vm, err := r.findVMInManifestItems(t.Items, decode)
		if err != nil {
			return nil, liberr.Wrap(err, "failed to decode vm manifest")
		}
		return vm, nil
	default:
		r.Log.V(1).Info("Unexpected object type in manifest", "type", fmt.Sprintf("%T", obj))
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
			Resources: core.VolumeResourceRequirements{
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

func (r *Builder) SupportsVolumePopulators(vmRef ref.Ref) bool {
	return false
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcs []*core.PersistentVolumeClaim, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) PopulatorTransferredBytes(persistentVolumeClaim *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
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
