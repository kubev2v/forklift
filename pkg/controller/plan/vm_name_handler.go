package plan

import (
	"context"
	"math/rand"
	"regexp"
	"strings"
	"time"

	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"k8s.io/apimachinery/pkg/fields"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *KubeVirt) changeVmNameDNS1123(vmName string, vmNamespace string) (generatedName string, err error) {
	generatedName = changeVmName(vmName)
	nameExist, errName := r.checkIfVmNameExistsInNamespace(generatedName, vmNamespace)
	if errName != nil {
		err = liberr.Wrap(errName)
		return
	}
	if nameExist {
		// If the name exists and it's at max allowed length, remove 5 chars from the end
		// so we won't reach the limit after appending vmId
		if len(generatedName) == NameMaxLength {
			generatedName = generatedName[0 : NameMaxLength-5]
		}
		generatedName = generatedName + "-" + generateRandVmNameSuffix()
	}
	return
}

// changes VM name to match DNS1123 RFC convention.
func changeVmName(currName string) string {
	var underscoreExcluded = regexp.MustCompile("[_]")
	var nameExcludeChars = regexp.MustCompile("[^a-z0-9-]")

	newName := strings.ToLower(currName)
	if len(newName) > NameMaxLength {
		newName = newName[0:NameMaxLength]
	}
	if underscoreExcluded.MatchString(newName) {
		newName = underscoreExcluded.ReplaceAllString(newName, "-")
	}
	if nameExcludeChars.MatchString(newName) {
		newName = nameExcludeChars.ReplaceAllString(newName, "")
	}
	newName = strings.Trim(newName, "-")
	if len(newName) == 0 {
		newName = "vm-" + generateRandVmNameSuffix()
	}
	return newName
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

// Generates a random string of length four, consisting of lowercase letters and digits.
func generateRandVmNameSuffix() string {
	const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"
	source := rand.NewSource(time.Now().UTC().UnixNano())
	seededRand := rand.New(source)

	b := make([]byte, 4)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
