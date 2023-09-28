package ocp

import (
	"strings"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
)

func GetNetworkNameAndNamespace(networkName string, vmRef *ref.Ref) (name, namespace string) {
	if !strings.Contains(networkName, "/") {
		namespace = vmRef.Namespace
		name = networkName
	} else {
		splitName := strings.Split(networkName, "/")
		namespace, name = splitName[0], splitName[1]
	}

	return
}
