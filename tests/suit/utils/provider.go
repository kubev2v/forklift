package utils

import (
	"context"
	"fmt"
	"time"

	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewProvider(providerName string, providerType forkliftv1.ProviderType, namespace string, annotations, providerSetting map[string]string, url string, secret *corev1.Secret) forkliftv1.Provider {
	// nicPairs set with the default settings for kind CI.

	providerMeta := v1.ObjectMeta{
		Namespace:   namespace,
		Name:        providerName,
		Annotations: annotations,
	}

	var secretRef corev1.ObjectReference
	if secret != nil {
		secretRef = corev1.ObjectReference{
			Name:      secret.Name,
			Namespace: namespace,
		}
	}

	p := forkliftv1.Provider{
		TypeMeta: v1.TypeMeta{
			Kind:       "Provider",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: providerMeta,
		Spec: forkliftv1.ProviderSpec{
			Type:     &providerType,
			URL:      url,
			Settings: providerSetting,
			Secret:   secretRef,
		},
	}

	return p
}

func WaitForProviderReadyWithTimeout(cl crclient.Client, namespace string, providerName string, timeout time.Duration) (*forkliftv1.Provider, error) {
	providerIdentifier := types.NamespacedName{Namespace: namespace, Name: providerName}

	returnedProvider := &forkliftv1.Provider{}
	err := wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, timeout, true, func(context.Context) (bool, error) {
		err := cl.Get(context.TODO(), providerIdentifier, returnedProvider)
		if err != nil || !returnedProvider.Status.Conditions.IsReady() {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		conditions := returnedProvider.Status.Conditions.List
		msg := "<no conditions>"
		if len(conditions) > 0 {
			msg = conditions[len(conditions)-1].Message
		}
		return nil, fmt.Errorf("provider %s not ready within %v - Phase/condition: %v/%v",
			providerName, timeout, returnedProvider.Status.Phase, msg)
	}
	return returnedProvider, nil
}

// CreateProviderFromDefinition is used by tests to create a Provider
func CreateProviderFromDefinition(cl crclient.Client, def forkliftv1.Provider) error {
	err := cl.Create(context.TODO(), &def, &crclient.CreateOptions{})

	if err == nil || apierrs.IsAlreadyExists(err) {
		return nil
	}
	return err
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
