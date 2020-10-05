package plan

import (
	"context"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/vsphere"
	kubevirt "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"gopkg.in/yaml.v2"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// migration label (value=UID)
	kMigration = "migration"
	// plan label (value=UID)
	kPlan = "plan"
	// VM label (value=vmID)
	kVM = "vmID"
)

//
// Represents kubevirt.
type KubeVirt struct {
	// Plan.
	Plan *api.Plan
	// Source.
	Source struct {
		// Provider.
		Provider *api.Provider
		// Secret.
		Secret *core.Secret
		// Provider API client.
		Client web.Client
	}
	// Destination.
	Destination struct {
		// Provider.
		Provider *api.Provider
		// k8s client.
		Client client.Client
	}
}

//
// Map of VM Import CRs keyed by vmID.
type ImportMap map[string]*kubevirt.VirtualMachineImport

//
// List related VM Import CRs.
func (r *KubeVirt) ListImports() (ImportMap, error) {
	result := ImportMap{}
	selector := labels.SelectorFromSet(
		map[string]string{
			kMigration: string(r.Plan.Status.Migration.Active),
			kPlan:      string(r.Plan.GetUID()),
		})
	list := &kubevirt.VirtualMachineImportList{}
	err := r.Destination.Client.List(
		context.TODO(),
		&client.ListOptions{
			Namespace:     r.Plan.Namespace,
			LabelSelector: selector,
		},
		list)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for _, vmImport := range list.Items {
		if vmImport.Labels != nil {
			vmID := vmImport.Labels[kVM]
			result[vmID] = &vmImport
		}
	}

	return result, nil
}

//
// Create the VM Import CR on the destination.
func (r *KubeVirt) CreateImport(vmID string) (err error) {
	newImport, err := r.buildImport(vmID)
	if err != nil {
		return
	}
	err = r.Destination.Client.Create(context.TODO(), newImport)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = nil
		} else {
			err = liberr.Wrap(err)
		}
	}

	return
}

//
// Ensure the namespace exists on the destination.
func (r *KubeVirt) EnsureNamespace() (err error) {
	ns := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: r.Plan.Namespace,
		},
	}
	err = r.Destination.Client.Create(context.TODO(), ns)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = nil
		}
	}

	return
}

//
// Ensure the VM Import mapping exists on the destination.
func (r *KubeVirt) EnsureMapping() (err error) {
	mapping, err := r.buildMapping()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.Destination.Client.Create(context.TODO(), mapping)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			found := &kubevirt.ResourceMapping{}
			err = r.Destination.Client.Get(
				context.TODO(),
				client.ObjectKey{
					Namespace: mapping.Namespace,
					Name:      mapping.Name,
				},
				found)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			found.Spec = mapping.Spec
			err = r.Destination.Client.Update(context.TODO(), found)
			if err != nil {
				err = liberr.Wrap(err)
			}
		}
	} else {
		err = liberr.Wrap(err)
	}

	return
}

//
// Ensure the VM Import secret exists on the destination.
func (r *KubeVirt) EnsureSecret() (err error) {
	secret, err := r.buildSecret()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.Destination.Client.Create(context.TODO(), secret)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			found := &core.Secret{}
			err = r.Destination.Client.Get(
				context.TODO(),
				client.ObjectKey{
					Namespace: secret.Namespace,
					Name:      secret.Name,
				},
				found)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			found.StringData = secret.StringData
			err = r.Destination.Client.Update(context.TODO(), found)
			if err != nil {
				err = liberr.Wrap(err)
			}
		}
	} else {
		err = liberr.Wrap(err)
	}

	return
}

//
// Build the VM Import CR.
func (r *KubeVirt) buildImport(vmID string) (object *kubevirt.VirtualMachineImport, err error) {
	source, err := r.buildSource(vmID)
	if err != nil {
		return
	}
	labels := map[string]string{
		kMigration: string(r.Plan.Status.Migration.Active),
		kPlan:      string(r.Plan.UID),
		kVM:        vmID,
	}
	object = &kubevirt.VirtualMachineImport{
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Plan.Namespace,
			Name:      r.Plan.NameForImport(vmID),
			Labels:    labels,
		},
		Spec: kubevirt.VirtualMachineImportSpec{
			Source: *source,
			ProviderCredentialsSecret: kubevirt.ObjectIdentifier{
				Namespace: &r.Plan.Namespace,
				Name:      r.Plan.NameForSecret(),
			},
			ResourceMapping: &kubevirt.ObjectIdentifier{
				Namespace: &r.Plan.Namespace,
				Name:      r.Plan.NameForMapping(),
			},
		},
	}

	return
}

//
// Build the ResourceMapping CR.
func (r *KubeVirt) buildMapping() (object *kubevirt.ResourceMapping, err error) {
	labels := map[string]string{
		kMigration: string(r.Plan.Status.Migration.Active),
		kPlan:      string(r.Plan.UID),
	}
	object = &kubevirt.ResourceMapping{
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Plan.Namespace,
			Name:      r.Plan.NameForMapping(),
			Labels:    labels,
		},
	}
	switch r.Source.Provider.Type() {
	case api.VSphere:
		netMap := []kubevirt.NetworkResourceMappingItem{}
		dsMap := []kubevirt.StorageResourceMappingItem{}
		for _, network := range r.Plan.Status.Migration.Map.Networks {
			netMap = append(
				netMap,
				kubevirt.NetworkResourceMappingItem{
					Source: kubevirt.Source{
						ID: &network.Source.ID,
					},
					Target: kubevirt.ObjectIdentifier{
						Namespace: &network.Destination.Namespace,
						Name:      network.Destination.Name,
					},
				})
		}
		for _, ds := range r.Plan.Status.Migration.Map.Datastores {
			dsMap = append(
				dsMap,
				kubevirt.StorageResourceMappingItem{
					Source: kubevirt.Source{
						ID: &ds.Source.ID,
					},
					Target: kubevirt.ObjectIdentifier{
						Name: ds.Destination.StorageClass,
					},
				})
		}
		object.Spec.VmwareMappings = &kubevirt.VmwareMappings{
			NetworkMappings: &netMap,
			StorageMappings: &dsMap,
		}
	}

	return
}

//
// Build the VM Import secret.
func (r *KubeVirt) buildSecret() (object *core.Secret, err error) {
	labels := map[string]string{
		kMigration: string(r.Plan.Status.Migration.Active),
		kPlan:      string(r.Plan.UID),
	}
	object = &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Plan.Namespace,
			Name:      r.Plan.NameForSecret(),
			Labels:    labels,
		},
	}
	switch r.Source.Provider.Type() {
	case api.VSphere:
		in := r.Source.Secret.Data
		out := map[string]string{
			"apiUrl":     r.Source.Provider.Spec.URL,
			"username":   string(in["user"]),
			"password":   string(in["password"]),
			"thumbprint": string(in["thumbprint"]),
		}
		content, mErr := yaml.Marshal(out)
		if mErr != nil {
			mErr = liberr.Wrap(err)
			return
		}
		object.StringData = map[string]string{
			"vmware": string(content),
		}
	}

	return
}

//
// Build the VM Import Source.
func (r *KubeVirt) buildSource(vmID string) (object *kubevirt.VirtualMachineImportSourceSpec, err error) {
	object = &kubevirt.VirtualMachineImportSourceSpec{}
	switch r.Source.Provider.Type() {
	case api.VSphere:
		vm := &vsphere.VM{}
		status, pErr := r.Source.Client.Get(vm, vmID)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		switch status {
		case http.StatusOK:
			uuid := vm.UUID
			object.Vmware = &kubevirt.VirtualMachineImportVmwareSourceSpec{
				VM: kubevirt.VirtualMachineImportVmwareSourceVMSpec{
					ID: &uuid,
				},
			}
		default:
			err = liberr.New(
				fmt.Sprintf(
					"VM %s uuid lookup failed: %s",
					vmID,
					http.StatusText(status)))
		}
	}

	return
}
