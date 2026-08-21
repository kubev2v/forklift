package nutanix

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/url"
	"path"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	providerbase "github.com/kubev2v/forklift/pkg/controller/base"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	libutil "github.com/kubev2v/forklift/pkg/lib/util"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Firmware boot types (model.VM.BootType).
const (
	bootTypeLegacy     = "LEGACY"
	bootTypeUEFI       = "UEFI"
	bootTypeSecureBoot = "SECURE_BOOT"
)

// Disk adapter types (model.Disk.AdapterType).
const (
	adapterSCSI = "SCSI"
	adapterSATA = "SATA"
	adapterIDE  = "IDE"
)

// Labels / secret keys for Prism Central download-cookie Secrets referenced
// via DataVolumeSourceHTTP.SecretExtraHeaders.
//
// The cookie Secret is created in DataVolumes() before the DataVolume exists
// in the API, so it cannot receive a DataVolume owner reference until later.
// Owner references are applied in two places (both idempotent):
//   - AdoptDownloadCookieSecretOwner, called from kubevirt.EnsureDataVolumes
//   - RefreshImportCredentials, merged into the cookie Secret update on auth failure
const (
	labelDownloadCookie = "forklift.konveyor.io/nutanix-download-cookie"
	cookieHeaderKey     = "cookie"
)

type Builder struct {
	*plancontext.Context
}

// Secret builds the DataVolume credential secret from the provider's
// connection secret. CDI's HTTP importer expects accessKeyId/secretKey
// (Basic Auth username/password), which Nutanix's Image Service also uses.
func (r *Builder) Secret(_ ref.Ref, in, object *core.Secret) error {
	object.StringData = map[string]string{
		"accessKeyId": string(in.Data["user"]),
		"secretKey":   string(in.Data["password"]),
	}
	return nil
}

// ConfigMap carries a CA certificate for CDI's HTTP importer to trust when
// downloading catalog images. CDI's DataVolumeSourceHTTP has no
// insecureSkipVerify field (unlike e.g. ImageIO), so when the provider is
// configured with insecureSkipVerify instead of a real CA, the only way to
// honor that is to fetch the server's own certificate once and trust it
// explicitly here -- the same fallback oVirt uses for older CNV versions.
func (r *Builder) ConfigMap(_ ref.Ref, in *core.Secret, object *core.ConfigMap) error {
	if cacert, found := libutil.GetCACert(in); found && len(cacert) > 0 {
		setCDICACerts(object, cacert)
		return nil
	}
	if !providerbase.GetInsecureSkipVerifyFlag(in) {
		return nil
	}

	cacert, err := r.fetchProviderCert()
	if err != nil {
		r.Log.Error(err, "Failed to fetch Nutanix provider certificate")
		// Don't fail here -- let the migration proceed and fail with a
		// clearer error from CDI if the certificate is actually needed.
		return nil
	}
	setCDICACerts(object, cacert)
	return nil
}

// setCDICACerts stores the trusted CA for CDI HTTP imports. CDI's Go
// HTTP client reads any PEM file under /certs/, but CDI passes
// cainfo=/certs/tls.crt when starting nbdkit curl.
func setCDICACerts(object *core.ConfigMap, cacert []byte) {
	if object.BinaryData == nil {
		object.BinaryData = map[string][]byte{}
	}
	object.BinaryData["ca.pem"] = cacert
	object.BinaryData["tls.crt"] = cacert
}

// fetchProviderCert dials the Nutanix provider URL and returns its leaf
// certificate PEM-encoded, for use as a trusted CA when insecureSkipVerify
// is set instead of a real CA certificate.
func (r *Builder) fetchProviderCert() ([]byte, error) {
	providerURL := r.Source.Provider.Spec.URL
	parsedURL, err := url.Parse(providerURL)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to parse Nutanix provider URL", "url", providerURL)
	}

	cert, err := libutil.GetTlsCertificate(parsedURL, &core.Secret{
		Data: map[string][]byte{"insecureSkipVerify": []byte("true")},
	})
	if err != nil {
		return nil, liberr.Wrap(err, "failed to fetch certificate from Nutanix provider")
	}
	if cert == nil {
		return nil, liberr.New("no certificate returned from Nutanix provider")
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), nil
}

func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
	vm := &model.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return liberr.Wrap(err, "vm", vmRef.String())
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}
	r.mapDisks(vm, persistentVolumeClaims, object)
	r.mapFirmware(vm, object)
	r.mapInput(object)
	r.mapTpm(object)
	r.mapNetworks(vm, object)
	r.mapCPU(vmRef, vm, object, usesInstanceType)
	r.mapMemory(vm, object, usesInstanceType)

	return nil
}

func (r *Builder) mapDisks(vm *model.VM, pvcs []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	bootDisk := bootDiskUUID(vm)

	for _, disk := range vm.Disks {
		if disk.IsCdrom {
			continue
		}
		pvc := r.findPVC(disk.UUID, pvcs)
		if pvc == nil {
			r.Log.Info("PVC not found for disk, skipping",
				"diskID", disk.UUID,
				"vmName", vm.Name)
			continue
		}
		volumeName := disk.UUID
		kVolumes = append(kVolumes, cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		})
		kDisk := cnv.Disk{
			Name: volumeName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					Bus: diskBus(disk.AdapterType),
				},
			},
			Serial: disk.UUID,
		}
		if disk.UUID == bootDisk {
			var bootOrder uint = 1
			kDisk.BootOrder = &bootOrder
		}
		kDisks = append(kDisks, kDisk)
	}
	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}

// bootDiskUUID selects the disk that receives KubeVirt boot order, using
// Nutanix boot_device_order_list from inventory. vm.Disks preserves
// disk_list order, so the first matching block disk wins when multiple
// disks share the same device index on different adapter types.
func bootDiskUUID(vm *model.VM) string {
	for _, bootType := range parseBootDeviceOrder(vm.BootDeviceOrder) {
		if bootType != "DISK" {
			continue
		}
		for _, disk := range vm.Disks {
			if disk.IsCdrom {
				continue
			}
			if disk.DeviceType != "" && disk.DeviceType != "DISK" {
				continue
			}
			return disk.UUID
		}
	}
	for _, disk := range vm.Disks {
		if !disk.IsCdrom {
			return disk.UUID
		}
	}
	return ""
}

func parseBootDeviceOrder(order string) []string {
	if order == "" {
		return []string{"DISK", "CDROM", "NETWORK"}
	}
	parts := strings.Split(order, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, strings.ToUpper(part))
		}
	}
	return result
}

func (r *Builder) findPVC(diskID string, pvcs []*core.PersistentVolumeClaim) *core.PersistentVolumeClaim {
	for _, pvc := range pvcs {
		if pvc.Annotations != nil {
			if pvc.Annotations[planbase.AnnDiskSource] == diskID {
				return pvc
			}
		}
	}
	return nil
}

// diskBus maps a Nutanix disk adapter type to a KubeVirt disk bus. AHV's
// SCSI and SATA/IDE adapters correspond directly to the equivalent KubeVirt
// buses; everything else (e.g. PCI) defaults to virtio, which AHV also
// exposes as a PCI-attached device.
func diskBus(adapterType string) cnv.DiskBus {
	switch adapterType {
	case adapterSCSI:
		return cnv.DiskBusSCSI
	case adapterSATA, adapterIDE:
		return cnv.DiskBusSATA
	default:
		return cnv.DiskBusVirtio
	}
}

// mapFirmware maps AHV's BootType (LEGACY, UEFI, SECURE_BOOT) to a KubeVirt
// firmware/bootloader configuration. SECURE_BOOT implies UEFI plus the SMM
// feature required for OVMF secure boot on KubeVirt.
func (r *Builder) mapFirmware(vm *model.VM, object *cnv.VirtualMachineSpec) {
	firmware := &cnv.Firmware{}
	switch vm.BootType {
	case bootTypeUEFI, bootTypeSecureBoot:
		secureBoot := vm.BootType == bootTypeSecureBoot
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &secureBoot,
			},
		}
		if secureBoot {
			object.Template.Spec.Domain.Features = &cnv.Features{
				SMM: &cnv.FeatureState{
					Enabled: &secureBoot,
				},
			}
		}
	case bootTypeLegacy:
		fallthrough
	default:
		// Unrecognized boot types fall back to BIOS/legacy rather than
		// failing the migration outright.
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Firmware = firmware
}

// mapTpm explicitly disables the vTPM device. AHV VMs may have a vTPM
// configured, but that isn't modeled in Nutanix inventory yet, so there's
// nothing to map -- explicitly disabling avoids inheriting an unrelated
// default from an instance type or preference.
func (r *Builder) mapTpm(object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Enabled: ptr.To(false)}
}

func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{
		{
			Type: "tablet",
			Name: "tablet",
			Bus:  cnv.InputBusVirtio,
		},
	}
}

func (r *Builder) mapNetworks(vm *model.VM, object *cnv.VirtualMachineSpec) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	numNetworks := 0
	pool := planbase.NewNADPool()

	for _, nic := range vm.NICs {
		nicRef := ref.Ref{ID: nic.SubnetUUID}
		pairs := planbase.FindAllMappingsForNICRef(nicRef, r.Map.Network)
		mapped, allocated := planbase.AllocateNetwork(pool, pairs)

		// Skip if no valid mapping found or the destination type is Ignored.
		if !allocated || mapped.Destination.Type == planbase.Ignored {
			continue
		}

		networkName := fmt.Sprintf("net-%d", numNetworks)
		numNetworks++

		kNetwork := cnv.Network{Name: networkName}
		kInterface := cnv.Interface{
			Name:       networkName,
			MacAddress: nic.MACAddress,
		}

		switch mapped.Destination.Type {
		case planbase.Multus:
			kNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
			}
			kInterface.Bridge = &cnv.InterfaceBridge{}
		case planbase.Pod:
			fallthrough
		default:
			kNetwork.Pod = &cnv.PodNetwork{}
			kInterface.Masquerade = &cnv.InterfaceMasquerade{}
		}

		kNetworks = append(kNetworks, kNetwork)
		kInterfaces = append(kInterfaces, kInterface)
	}

	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces
}

func (r *Builder) mapCPU(vmRef ref.Ref, vm *model.VM, object *cnv.VirtualMachineSpec, usesInstanceType bool) {
	if usesInstanceType {
		return
	}
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(vm.NumSockets),
		Cores:   uint32(vm.NumVcpusPerSocket),
		Threads: uint32(vm.NumThreadsPerCore),
	}
	if enableNestedVirt := r.NestedVirtualizationSetting(vmRef, false); enableNestedVirt != nil {
		policy := "optional"
		if !*enableNestedVirt {
			policy = "disable"
		}
		object.Template.Spec.Domain.CPU.Features = append(object.Template.Spec.Domain.CPU.Features,
			cnv.CPUFeature{Name: "vmx", Policy: policy},
			cnv.CPUFeature{Name: "svm", Policy: policy},
		)
	}
}

func (r *Builder) mapMemory(vm *model.VM, object *cnv.VirtualMachineSpec, usesInstanceType bool) {
	if usesInstanceType {
		return
	}
	memory := resource.NewQuantity(vm.MemorySizeMiB*1024*1024, resource.BinarySI)
	object.Template.Spec.Domain.Resources.Requests = core.ResourceList{
		core.ResourceMemory: *memory,
	}
}

// DataVolumes builds one CDI HTTP-import DataVolume per non-CDROM disk,
// sourced from the catalog image PreTransferActions created for that
// disk. By the time this runs, the migration controller has already
// confirmed PreTransferActions reported every image ready, so images are
// expected to be present and finished uploading.
//
// Prism Element's images are served directly over Basic Auth (the v3
// Image Service); Prism Central's v4 Image Service instead requires
// resolving a one-time redirect+cookie handshake up front, since a
// generic HTTP client -- like CDI's importer -- can't complete that
// itself (see resolveImageV4DownloadURL). The cookie is stored in a
// SecretExtraHeaders Secret so it can be rotated without recreating the
// DataVolume if the session expires mid-transfer.
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, _ *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	vm := &model.VM{}
	if err = r.Source.Inventory.Find(vm, vmRef); err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}

	storageMap := map[string]api.DestinationStorage{}
	for _, mapped := range r.Map.Storage.Spec.Map {
		storageMap[mapped.Source.ID] = mapped.Destination
	}

	client := &Client{Context: r.Context}
	if err = client.connect(); err != nil {
		return nil, err
	}
	element, err := client.isPrismElement()
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(r.Source.Provider.Spec.URL, "/")

	for _, disk := range vm.Disks {
		if disk.IsCdrom {
			continue
		}
		destination, mapped := storageMap[disk.StorageContainerUUID]
		if !mapped {
			return nil, liberr.New(
				"no storage mapping for disk storage container",
				"vm", vmRef.String(),
				"disk", disk.UUID,
				"storageContainer", disk.StorageContainerUUID)
		}

		var httpSource *cdi.DataVolumeSourceHTTP
		name := migrationImageName(string(r.Migration.UID), vmRef, disk.UUID)
		if element {
			httpSource, err = r.elementHTTPSource(baseURL, name, secret, configMap, client)
		} else {
			httpSource, err = r.centralHTTPSource(vmRef, disk.UUID, name, configMap, client, dvTemplate.Labels)
		}
		if err != nil {
			return nil, liberr.Wrap(err, "vm", vmRef.String(), "disk", disk.UUID)
		}

		dv := r.mapDataVolume(disk, destination, dvTemplate, httpSource)
		dvs = append(dvs, *dv)
	}

	return dvs, nil
}

// elementHTTPSource builds a DataVolumeSourceHTTP pointing at a Prism
// Element catalog image's v3 file download endpoint, authenticated with
// the same Basic Auth credentials as the rest of the provider's API.
func (r *Builder) elementHTTPSource(baseURL, name string, secret *core.Secret, configMap *core.ConfigMap, client *Client) (*cdi.DataVolumeSourceHTTP, error) {
	entity, found, err := client.findImageByName(name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, liberr.New("catalog image not found", "image", name)
	}
	return &cdi.DataVolumeSourceHTTP{
		URL:           fmt.Sprintf("%s/api/nutanix/v3/images/%s/file", baseURL, entity.Metadata.UUID),
		SecretRef:     secret.Name,
		CertConfigMap: configMap.Name,
	}, nil
}

// centralHTTPSource builds a DataVolumeSourceHTTP pointing at a Prism
// Central catalog image's already-resolved download location. The session
// cookie from resolveImageV4DownloadURL is stored in a dedicated Secret and
// referenced via SecretExtraHeaders (same pattern as OCP VM export tokens)
// so the token is not left in plaintext on the DataVolume spec. There's no
// Basic Auth SecretRef here: the resolved URL/cookie pair is itself the
// credential.
//
// Prism Central redirects to a CVM address that often isn't covered by the
// Prism Element certificate's SAN (the VIP is). The PE VIP serves the same
// entity_download path with the same cookie, so we rewrite the Location
// host to the cluster's external IP when we can discover one.
func (r *Builder) centralHTTPSource(vmRef ref.Ref, diskUUID, name string, configMap *core.ConfigMap, client *Client, labels map[string]string) (*cdi.DataVolumeSourceHTTP, error) {
	downloadURL, cookie, err := r.resolveCentralDownload(client, name)
	if err != nil {
		return nil, err
	}
	cookieSecret, err := r.ensureDownloadCookieSecret(vmRef, diskUUID, cookie, labels)
	if err != nil {
		return nil, err
	}
	return &cdi.DataVolumeSourceHTTP{
		URL:                downloadURL,
		CertConfigMap:      configMap.Name,
		SecretExtraHeaders: []string{cookieSecret.Name},
	}, nil
}

// resolveCentralDownload looks up the v4 catalog image and performs the
// redirect+cookie handshake, rewriting the Location host to the PE VIP when
// possible.
func (r *Builder) resolveCentralDownload(client *Client, imageName string) (downloadURL, cookie string, err error) {
	entity, found, _, err := client.findImageV4ByName(imageName)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", liberr.New("catalog image not found", "image", imageName)
	}
	downloadURL, cookie, err = client.resolveImageV4DownloadURL(entity.ExtID)
	if err != nil {
		return "", "", err
	}
	return client.preferClusterExternalURL(downloadURL), cookie, nil
}

// ensureDownloadCookieSecret creates or updates the per-disk Secret that
// holds the Prism Central download Cookie header for CDI's importer.
func (r *Builder) ensureDownloadCookieSecret(vmRef ref.Ref, diskUUID, cookie string, labels map[string]string) (*core.Secret, error) {
	secretLabels := map[string]string{
		labelDownloadCookie:    "true",
		planbase.AnnDiskSource: diskUUID,
	}
	for k, v := range labels {
		secretLabels[k] = v
	}

	header := cookieHeaderValue(cookie)
	selector := map[string]string{
		labelDownloadCookie:    "true",
		planbase.AnnDiskSource: diskUUID,
	}
	if migrationID, ok := labels["migration"]; ok && migrationID != "" {
		selector["migration"] = migrationID
	}
	list := &core.SecretList{}
	err := r.Destination.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(selector),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String(), "disk", diskUUID)
	}

	if len(list.Items) > 0 {
		secret := &list.Items[0]
		secret.StringData = map[string]string{cookieHeaderKey: header}
		if err = r.Destination.Update(context.TODO(), secret); err != nil {
			return nil, liberr.Wrap(err, "secret", secret.Name, "disk", diskUUID)
		}
		r.Log.V(1).Info("Updated Nutanix download cookie secret.",
			"secret", path.Join(secret.Namespace, secret.Name),
			"vm", vmRef.String(),
			"disk", diskUUID)
		return secret, nil
	}

	secret := &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Plan.Spec.TargetNamespace,
			GenerateName: strings.Join(
				[]string{r.Plan.Name, vmRef.ID, "dlcookie"},
				"-") + "-",
			Labels: secretLabels,
			Annotations: map[string]string{
				planbase.AnnDiskSource: diskUUID,
			},
		},
		StringData: map[string]string{cookieHeaderKey: header},
	}
	if err = r.Destination.Create(context.TODO(), secret); err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String(), "disk", diskUUID)
	}
	r.Log.V(1).Info("Created Nutanix download cookie secret.",
		"secret", path.Join(secret.Namespace, secret.Name),
		"vm", vmRef.String(),
		"disk", diskUUID)
	return secret, nil
}

// setDownloadCookieSecretOwner marks the DataVolume as owner of its
// download-cookie Secret in memory. The caller must persist the Secret.
func (r *Builder) setDownloadCookieSecretOwner(secret *core.Secret, dv *cdi.DataVolume) error {
	if secret == nil || dv == nil || dv.Name == "" {
		return nil
	}

	owner := &cdi.DataVolume{}
	err := r.Destination.Get(
		context.TODO(),
		types.NamespacedName{Namespace: dv.Namespace, Name: dv.Name},
		owner,
	)
	if err != nil {
		return liberr.Wrap(err, "dv.name", dv.Name, "dv.namespace", dv.Namespace)
	}

	return controllerutil.SetOwnerReference(owner, secret, r.Destination.Scheme())
}

// AdoptDownloadCookieSecretOwner sets the DataVolume as owner of its
// download-cookie Secret so Kubernetes GC removes the Secret when the
// DataVolume is deleted. Called from kubevirt.EnsureDataVolumes via a
// type assertion once the DataVolume exists in the API.
func (r *Builder) AdoptDownloadCookieSecretOwner(dv *cdi.DataVolume) error {
	if dv == nil || dv.Name == "" || dv.Spec.Source == nil || dv.Spec.Source.HTTP == nil {
		return nil
	}
	if len(dv.Spec.Source.HTTP.SecretExtraHeaders) == 0 {
		return nil
	}

	secretName := dv.Spec.Source.HTTP.SecretExtraHeaders[0]
	secret := &core.Secret{}
	err := r.Destination.Get(
		context.TODO(),
		types.NamespacedName{Namespace: dv.Namespace, Name: secretName},
		secret,
	)
	if err != nil {
		return liberr.Wrap(err, "secret", secretName, "dv.name", dv.Name)
	}

	if err = r.setDownloadCookieSecretOwner(secret, dv); err != nil {
		return err
	}
	if err = r.Destination.Update(context.TODO(), secret); err != nil {
		return liberr.Wrap(err, "secret", secret.Name, "dv.name", dv.Name)
	}
	return nil
}

func cookieHeaderValue(cookie string) string {
	return "Cookie: " + cookie
}

// RefreshImportCredentials re-resolves the Prism Central download URL/cookie
// for a DataVolume whose importer failed with an auth error, updates the
// SecretExtraHeaders Secret (and the DV URL if the Location changed), and
// returns refreshed=true so the caller can restart the importer pod.
//
// Also re-applies the DataVolume owner reference on the cookie Secret in
// the same Update (see AdoptDownloadCookieSecretOwner for the creation path).
func (r *Builder) RefreshImportCredentials(dv *cdi.DataVolume) (bool, error) {
	if dv == nil || dv.Spec.Source == nil || dv.Spec.Source.HTTP == nil {
		return false, nil
	}
	httpSource := dv.Spec.Source.HTTP
	if len(httpSource.SecretExtraHeaders) == 0 {
		return false, nil
	}

	diskUUID := ""
	if dv.Annotations != nil {
		diskUUID = dv.Annotations[planbase.AnnDiskSource]
	}
	if diskUUID == "" {
		return false, nil
	}

	client := &Client{Context: r.Context}
	if err := client.connect(); err != nil {
		return false, err
	}
	element, err := client.isPrismElement()
	if err != nil {
		return false, err
	}
	if element {
		return false, nil
	}

	vmID := ""
	if dv.Labels != nil {
		vmID = dv.Labels["vmID"]
	}
	if vmID == "" {
		return false, liberr.New("missing vmID label on DataVolume", "dv", path.Join(dv.Namespace, dv.Name))
	}

	imageName := migrationImageName(string(r.Migration.UID), ref.Ref{ID: vmID}, diskUUID)
	downloadURL, cookie, err := r.resolveCentralDownload(client, imageName)
	if err != nil {
		return false, liberr.Wrap(err, "dv", path.Join(dv.Namespace, dv.Name), "disk", diskUUID)
	}

	secretName := httpSource.SecretExtraHeaders[0]
	secret := &core.Secret{}
	err = r.Destination.Get(
		context.TODO(),
		types.NamespacedName{Namespace: dv.Namespace, Name: secretName},
		secret,
	)
	if err != nil {
		return false, liberr.Wrap(err, "secret", secretName)
	}
	secret.StringData = map[string]string{cookieHeaderKey: cookieHeaderValue(cookie)}
	if err = r.setDownloadCookieSecretOwner(secret, dv); err != nil {
		return false, liberr.Wrap(err, "secret", secretName, "dv.name", dv.Name)
	}
	if err = r.Destination.Update(context.TODO(), secret); err != nil {
		return false, liberr.Wrap(err, "secret", secretName)
	}

	if httpSource.URL != downloadURL {
		updated := dv.DeepCopy()
		updated.Spec.Source.HTTP.URL = downloadURL
		if err = r.Destination.Update(context.TODO(), updated); err != nil {
			return false, liberr.Wrap(err, "dv", path.Join(dv.Namespace, dv.Name))
		}
		r.Log.Info("Updated Nutanix download URL after cookie refresh.",
			"dv", path.Join(dv.Namespace, dv.Name),
			"disk", diskUUID)
	}

	r.Log.Info("Refreshed Nutanix download cookie secret.",
		"secret", path.Join(secret.Namespace, secret.Name),
		"dv", path.Join(dv.Namespace, dv.Name),
		"disk", diskUUID)
	return true, nil
}

func (r *Builder) mapDataVolume(
	disk model.Disk,
	destination api.DestinationStorage,
	dvTemplate *cdi.DataVolume,
	httpSource *cdi.DataVolumeSourceHTTP,
) (dv *cdi.DataVolume) {
	storageClass := destination.StorageClass
	dvSpec := cdi.DataVolumeSpec{
		Source: &cdi.DataVolumeSource{HTTP: httpSource},
		Storage: &cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: *resource.NewQuantity(disk.DiskSizeBytes, resource.BinarySI),
				},
			},
			StorageClassName: &storageClass,
		},
	}
	if destination.AccessMode != "" {
		dvSpec.Storage.AccessModes = []core.PersistentVolumeAccessMode{destination.AccessMode}
	}
	if destination.VolumeMode != "" {
		dvSpec.Storage.VolumeMode = &destination.VolumeMode
	}

	dv = dvTemplate.DeepCopy()
	dv.Spec = dvSpec
	if dv.Annotations == nil {
		dv.Annotations = make(map[string]string)
	}
	dv.Annotations[planbase.AnnDiskSource] = disk.UUID
	// Nutanix has no conversion pod to act as the first consumer of a
	// WaitForFirstConsumer storage class during a cold migration to the
	// local cluster (unlike vSphere), so request immediate binding here --
	// otherwise the PVC, and thus the import, would never start.
	dv.Annotations[planbase.AnnBindImmediate] = "true"
	return dv
}

func (r *Builder) Tasks(vmRef ref.Ref) (tasks []*plan.Task, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, disk := range vm.Disks {
		if disk.IsCdrom {
			continue
		}
		mB := disk.DiskSizeBytes / 0x100000
		tasks = append(tasks, &plan.Task{
			Name: disk.UUID,
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

func (r *Builder) TemplateLabels(_ ref.Ref) (labels map[string]string, err error) {
	labels = make(map[string]string)
	return
}

func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	if dv == nil || dv.Annotations == nil {
		return ""
	}
	return dv.Annotations[planbase.AnnDiskSource]
}

func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	if pvc == nil || pvc.Annotations == nil {
		return ""
	}
	return pvc.Annotations[planbase.AnnDiskSource]
}

func (r *Builder) PodEnvironment(_ ref.Ref, _ *core.Secret) (env []core.EnvVar, err error) {
	return nil, nil
}

func (r *Builder) LunPersistentVolumes(_ ref.Ref) (pvs []core.PersistentVolume, err error) {
	return
}

func (r *Builder) LunPersistentVolumeClaims(_ ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	return
}

func (r *Builder) SupportsVolumePopulators() bool {
	return false
}

func (r *Builder) PopulatorVolumes(_ ref.Ref, _ map[string]string, _ string) ([]*core.PersistentVolumeClaim, error) {
	return nil, planbase.VolumePopulatorNotSupportedError
}

func (r *Builder) PopulatorTransferredBytes(_ *core.PersistentVolumeClaim) (int64, error) {
	return 0, planbase.VolumePopulatorNotSupportedError
}

func (r *Builder) PopulatorOffloadInfo(_ *core.PersistentVolumeClaim) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *Builder) PopulatorXcopyUsed(_ *core.PersistentVolumeClaim) (string, bool, error) {
	return "", false, nil
}

func (r *Builder) SetPopulatorDataSourceLabels(_ ref.Ref, _ []*core.PersistentVolumeClaim) error {
	return nil
}

func (r *Builder) GetPopulatorTaskName(_ *core.PersistentVolumeClaim) (string, error) {
	return "", nil
}

func (r *Builder) PreferenceName(_ ref.Ref, _ *core.ConfigMap) (string, error) {
	return "", nil
}

func (r *Builder) ConfigMaps(_ ref.Ref) (list []core.ConfigMap, err error) {
	return nil, nil
}

func (r *Builder) Secrets(_ ref.Ref) (list []core.Secret, err error) {
	return nil, nil
}

func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}

func (r *Builder) NetAppShiftPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) CsiImportPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) SourceVMLabelsAndAnnotations(_ ref.Ref, _ *api.TagMapping) (labels map[string]string, annotations map[string]string, sanitizationReport map[string]string, err error) {
	// TODO: map Nutanix categories to destination labels/annotations
	return
}
