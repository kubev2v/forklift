package utils

import (
	"context"
	"fmt"
	forkliftv1 "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewProvider(providerName string, providerType forkliftv1.ProviderType, namespace string, url string, secret *corev1.Secret) forkliftv1.Provider {
	// nicPairs set with the default settings for kind CI.

	providerMeta := v1.ObjectMeta{
		Namespace: "konveyor-forklift",
		//Name:      providerName,
		Name: "ovirt-provider",
	}

	ovirtProvider := forkliftv1.OVirt

	//vsphere := forkliftv1.VSphere
	p := forkliftv1.Provider{
		TypeMeta: v1.TypeMeta{
			Kind:       "Provider",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: providerMeta,
		Spec: forkliftv1.ProviderSpec{
			Type: &ovirtProvider,
			URL:  url,
			Secret: corev1.ObjectReference{
				Name:      secret.Name,
				Namespace: namespace,
			},
		},
	}

	return p
}

// CreateProviderFromDefinition is used by tests to create a Provider
func CreateProviderFromDefinition(cl crclient.Client, namespace string, def forkliftv1.Provider) error {
	err := cl.Create(context.TODO(), &def, &crclient.CreateOptions{})

	if err == nil || apierrs.IsAlreadyExists(err) {
		return nil
	}
	return err
	//err := wait.PollImmediate(networkMapPollInterval, networkMapCreateTime, func() (bool, error) {
	//	var err error
	//	err = cl.Create(context.TODO(), def, &crclient.CreateOptions{})
	//
	//	if err == nil || apierrs.IsAlreadyExists(err) {
	//		return true, nil
	//	}
	//	return false, err
	//})
	//if err != nil {
	//	return err
	//}
	//return nil
}

// GetProvider returns provider object
func GetProvider(cl crclient.Client, providerName string, namespace string) (*forkliftv1.Provider, error) {
	returnedProvider := &forkliftv1.Provider{}

	providerObj := v1.ObjectMeta{
		Namespace: namespace,
		Name:      providerName,
	}

	providerIdentifier := types.NamespacedName{Namespace: providerObj.Namespace, Name: providerObj.Name}
	fmt.Fprintf(ginkgo.GinkgoWriter, "DEBUG: %s", providerObj.Namespace)
	err := cl.Get(context.TODO(), providerIdentifier, returnedProvider)
	if err != nil {
		return nil, err
	}
	return returnedProvider, nil
}
