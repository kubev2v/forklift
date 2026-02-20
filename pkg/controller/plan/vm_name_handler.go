package plan

import (
	"context"

	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/validation"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *KubeVirt) changeVmNameDNS1123(vmName string, vmNamespace string) (generatedName string, err error) {
	generatedName = util.SanitizeLabel(vmName)
	nameExist, errName := r.checkIfVmNameExistsInNamespace(generatedName, vmNamespace)
	if errName != nil {
		err = liberr.Wrap(errName)
		return
	}
	if nameExist {
		// If the name exists and it's at max allowed length, remove 5 chars from the end
		// so we won't reach the limit after appending vmId
		max := validation.DNS1123LabelMaxLength
		if len(generatedName) > max-5 {
			generatedName = generatedName[:max-5]
		}
		generatedName = generatedName + "-" + util.GenerateRandomSuffix()
	}
	return
}

// Checks if VM with the newly generated name exists on the destination
func (r *KubeVirt) checkIfVmNameExistsInNamespace(name string, namespace string) (nameExist bool, err error) {
	list := &cnv.VirtualMachineList{}
	nameField := "metadata.name"
	namespaceField := "metadata.namespace"
	listOptions := &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{
			nameField:      name,
			namespaceField: namespace,
		}),
	}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		listOptions,
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		nameExist = true
		return
	}
	// Checks that the new name does not match a valid
	// VM name in the same plan
	for _, vm := range r.Migration.Status.VMs {
		if vm.Name == name {
			nameExist = true
			return
		}
	}
	nameExist = false
	return
}
