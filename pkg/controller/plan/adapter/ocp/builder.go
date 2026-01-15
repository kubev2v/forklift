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
	"k8s.io/apimachinery/pkg/types"
	export "kubevirt.io/api/export/v1alpha1"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	ocpclient "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

	// For Skipped exports (e.g., ContainerDisk-only VMs), no cert to provide
	if vmExport.Status != nil && vmExport.Status.Phase == export.Skipped {
		r.Log.Info("VMExport is skipped, no ConfigMap cert needed", "vmRef", vmRef)
		return nil
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
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *v1.Secret, configMap *v1.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *v1.ConfigMap) (dvs []cdi.DataVolume, err error) {
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

	// For Skipped exports (e.g., ContainerDisk-only VMs), no volumes to transfer
	if vmExport.Status != nil && vmExport.Status.Phase == export.Skipped {
		r.Log.Info("VMExport is skipped, no DataVolumes to create", "vmRef", vmRef)
		return []cdi.DataVolume{}, nil
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
		// The dvTemplate contains GenerateName which will create a PVC with different name than the original PVC
		dataVolume.GenerateName = ""
		dataVolume.Name = pvc.Name
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

	// For Skipped exports (e.g., ContainerDisk-only VMs), no token secret
	if vmExport.Status != nil && vmExport.Status.Phase == export.Skipped {
		r.Log.Info("VMExport is skipped, no Secret token needed", "vmRef", vmRef)
		return nil
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
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*v1.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
	sourceVm, err := r.getSourceVmFromDefinition(vmRef)
	if err != nil {
		return liberr.Wrap(err)
	}

	targetVmSpec := sourceVm.Spec.DeepCopy()
	object.Template = targetVmSpec.Template
	// Preserve DataVolumeTemplates from source VM to maintain user workflows
	// that may expect the VM's DataVolume to be present
	object.DataVolumeTemplates = targetVmSpec.DataVolumeTemplates

	// Sanitize DataVolumeTemplates to prevent conflicts with Forklift-created DataVolumes:
	// 1. Match template names to the PVC names (Forklift creates DataVolumes with Name = pvc.Name)
	// 2. Clear spec.source to prevent KubeVirt from trying to create new DataVolumes with invalid sources
	// 3. Ensure namespace is set correctly (will be set by KubeVirt to match VM namespace)
	// 4. Update volume references to match renamed DataVolumeTemplates
	r.sanitizeDataVolumeTemplates(vmRef, object.DataVolumeTemplates, object)

	r.mapNetworks(sourceVm, targetVmSpec)

	return nil
}

// sanitizeDataVolumeTemplates ensures DataVolumeTemplates are compatible with Forklift-created DataVolumes.
// Forklift creates DataVolumes with Name = pvc.Name, so templates must match these names.
// We also clear spec.source to prevent KubeVirt from trying to create new DataVolumes with invalid source URLs.
// Additionally, we update volume references to match any renamed DataVolumeTemplates.
func (r *Builder) sanitizeDataVolumeTemplates(vmRef ref.Ref, templates []cnv.DataVolumeTemplateSpec, object *cnv.VirtualMachineSpec) {
	// Build a map of volume name -> PVC name from VM volumes
	// This allows us to match DataVolumeTemplates to the PVCs that Forklift will create DataVolumes for
	volumeToPVCName := make(map[string]string)
	sourceVm, err := r.getSourceVmFromDefinition(vmRef)
	if err != nil {
		r.Log.Error(err, "Failed to get source VM for DataVolumeTemplate sanitization")
		return
	}

	for _, vol := range sourceVm.Spec.Template.Spec.Volumes {
		var pvcName string
		switch {
		case vol.PersistentVolumeClaim != nil:
			// Volume references PVC directly
			pvcName = vol.PersistentVolumeClaim.ClaimName
		case vol.DataVolume != nil:
			// Volume references DataVolume - need to find the PVC it creates
			// In OCP, DataVolumes typically create PVCs with the same name as the DataVolume
			// But we need to check the actual PVC name from the source
			dv := &cdi.DataVolume{}
			err := r.sourceClient.Get(context.TODO(), client.ObjectKey{
				Namespace: vmRef.Namespace,
				Name:      vol.DataVolume.Name,
			}, dv)
			if err != nil {
				r.Log.V(1).Info("Could not find source DataVolume, using DataVolume name as PVC name",
					"dataVolume", vol.DataVolume.Name, "error", err)
				pvcName = vol.DataVolume.Name
			} else {
				// Use the actual PVC name that the DataVolume created
				if dv.Status.ClaimName != "" {
					pvcName = dv.Status.ClaimName
				} else {
					// Fallback: DataVolume name typically matches PVC name
					pvcName = vol.DataVolume.Name
				}
			}
		default:
			continue
		}
		if pvcName != "" {
			volumeToPVCName[vol.Name] = pvcName
		}
	}

	// Track template name changes so we can update volume references
	templateNameMap := make(map[string]string) // old name -> new name

	// Sanitize each template
	for i := range templates {
		template := &templates[i]
		oldName := template.Name

		// Find the corresponding PVC name by matching template to VM volumes
		// DataVolumeTemplates are referenced by volumes via vol.DataVolume.Name matching template name
		var targetPVCName string
		for volName, pvcName := range volumeToPVCName {
			// Check if any volume references this template by name
			for _, vol := range sourceVm.Spec.Template.Spec.Volumes {
				if vol.Name == volName && vol.DataVolume != nil && vol.DataVolume.Name == template.Name {
					targetPVCName = pvcName
					break
				}
			}
			if targetPVCName != "" {
				break
			}
		}

		// If we found a matching PVC name, update the template name to match
		// Forklift creates DataVolumes with Name = pvc.Name, so template must match
		if targetPVCName != "" && template.Name != targetPVCName {
			template.Name = targetPVCName
			template.GenerateName = ""
			templateNameMap[oldName] = targetPVCName
			r.Log.V(1).Info("Updated DataVolumeTemplate name to match Forklift-created DataVolume",
				"oldName", oldName, "newName", template.Name, "pvcName", targetPVCName)
		} else {
			// If we can't find a match, clear GenerateName to use explicit Name
			// and log a warning
			if template.GenerateName != "" {
				r.Log.Info("DataVolumeTemplate has GenerateName but no matching volume found, "+
					"template may not match Forklift-created DataVolume",
					"generateName", template.GenerateName, "templateName", template.Name)
				template.GenerateName = ""
			}
		}

		// Clear spec.source to prevent KubeVirt from trying to create new DataVolumes
		// with source URLs/namespaces that don't exist on the destination cluster.
		// Forklift has already created the DataVolumes, so templates should not have source specs.
		template.Spec.Source = nil

		// Ensure namespace is empty - KubeVirt will set it to match the VM namespace
		template.Namespace = ""
	}

	// Update volume references to match renamed DataVolumeTemplates
	// Volumes that reference DataVolumes by name need to be updated if the template name changed
	if len(templateNameMap) > 0 && object.Template != nil {
		for i := range object.Template.Spec.Volumes {
			vol := &object.Template.Spec.Volumes[i]
			if vol.DataVolume != nil {
				if newName, renamed := templateNameMap[vol.DataVolume.Name]; renamed {
					oldDVName := vol.DataVolume.Name
					vol.DataVolume.Name = newName
					r.Log.V(1).Info("Updated volume DataVolume reference to match renamed template",
						"volumeName", vol.Name, "oldDVName", oldDVName, "newDVName", newName)
				}
			}
		}
	}
}

// ConfigMaps builds CRs for each of the ConfigMaps that the source VM depends upon.
// Migration labels are set to track when they were first created, but since these may be
// used by more than one VM they are not labeled with the VM id.
func (r *Builder) ConfigMaps(vmRef ref.Ref) (list []core.ConfigMap, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	sources := []types.NamespacedName{}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		switch {
		case vol.ConfigMap != nil:
			key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.ConfigMap.Name}
			sources = append(sources, key)
		case vol.Sysprep != nil:
			if vol.Sysprep.ConfigMap != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.Sysprep.ConfigMap.Name}
				sources = append(sources, key)
			}
		default:
			continue
		}
	}
	for _, key := range sources {
		source := &core.ConfigMap{}
		err = r.sourceClient.Get(context.Background(), key, source)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		target := core.ConfigMap{}
		target.Name = source.Name
		target.Namespace = r.Plan.Spec.TargetNamespace
		target.Data = source.Data
		target.BinaryData = source.BinaryData
		target.Immutable = source.Immutable
		target.SetLabels(source.GetLabels())
		r.Labeler.SetLabels(&target, r.Labeler.MigrationLabels())
		target.SetAnnotations(source.GetAnnotations())
		r.Labeler.SetAnnotation(&target, planbase.AnnSource, key.String())
		list = append(list, target)
	}
	return
}

// Secrets builds CRs for each of the Secrets that the source VM depends upon.
// Migration labels are set to track when they were first created, but since these may be
// used by more than one VM they are not labeled with the VM id.
func (r *Builder) Secrets(vmRef ref.Ref) (list []core.Secret, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	sources := []types.NamespacedName{}
	for _, cred := range virtualMachine.Object.Spec.Template.Spec.AccessCredentials {
		switch {
		case cred.SSHPublicKey != nil:
			if cred.SSHPublicKey.Source.Secret != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: cred.SSHPublicKey.Source.Secret.SecretName}
				sources = append(sources, key)
			}
		case cred.UserPassword != nil:
			if cred.UserPassword.Source.Secret != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: cred.UserPassword.Source.Secret.SecretName}
				sources = append(sources, key)
			}
		}
	}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		switch {
		case vol.Secret != nil:
			key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.Secret.SecretName}
			sources = append(sources, key)
		case vol.CloudInitNoCloud != nil:
			if vol.CloudInitNoCloud.UserDataSecretRef != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.CloudInitNoCloud.UserDataSecretRef.Name}
				sources = append(sources, key)
			}
			if vol.CloudInitNoCloud.NetworkDataSecretRef != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.CloudInitNoCloud.NetworkDataSecretRef.Name}
				sources = append(sources, key)
			}
		case vol.CloudInitConfigDrive != nil:
			if vol.CloudInitConfigDrive.UserDataSecretRef != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.CloudInitConfigDrive.UserDataSecretRef.Name}
				sources = append(sources, key)
			}
			if vol.CloudInitConfigDrive.NetworkDataSecretRef != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.CloudInitConfigDrive.NetworkDataSecretRef.Name}
				sources = append(sources, key)
			}
		case vol.Sysprep != nil:
			if vol.Sysprep.Secret != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.Sysprep.Secret.Name}
				sources = append(sources, key)
			}
		case vol.ContainerDisk != nil:
			if vol.ContainerDisk.ImagePullSecret != "" {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.ContainerDisk.ImagePullSecret}
				sources = append(sources, key)
			}
		default:
			continue
		}
	}
	for _, key := range sources {
		source := &core.Secret{}
		err = r.sourceClient.Get(context.Background(), key, source)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		target := core.Secret{}
		target.Name = source.Name
		target.Namespace = r.Plan.Spec.TargetNamespace
		target.Data = source.Data
		target.Immutable = source.Immutable
		target.SetLabels(source.GetLabels())
		r.Labeler.SetLabels(&target, r.Labeler.MigrationLabels())
		target.SetAnnotations(source.GetAnnotations())
		r.Labeler.SetAnnotation(&target, planbase.AnnSource, key.String())
		list = append(list, target)
	}
	return
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

func (r *Builder) getSourceVmFromDefinition(vmRef ref.Ref) (*cnv.VirtualMachine, error) {
	vme := &export.VirtualMachineExport{}
	if err := r.sourceClient.Get(context.Background(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vme); err != nil {
		return nil, liberr.Wrap(err)
	}

	// For Skipped exports (e.g., ContainerDisk-only VMs), read VM directly from source
	if vme.Status != nil && vme.Status.Phase == export.Skipped {
		sourceVm := &cnv.VirtualMachine{}
		if err := r.sourceClient.Get(context.Background(), client.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, sourceVm); err != nil {
			return nil, liberr.Wrap(err, "failed to get source VM for skipped export")
		}
		r.Log.Info("Retrieved VM directly from source for skipped export", "vm", sourceVm.Name)
		return sourceVm, nil
	}

	// Fetch VM manifest from export server
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
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: size,
				},
			},
			StorageClassName: &storageClassName,
		},
	}
}

func (r *Builder) SupportsVolumePopulators() bool {
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

// ConversionPodConfig returns provider-specific configuration for the virt-v2v conversion pod.
// OCP provider does not require any special configuration.
func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}
