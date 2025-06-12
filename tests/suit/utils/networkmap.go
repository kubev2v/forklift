package utils

import (
	"context"
	"fmt"
	"time"

	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateNetworkMapFromDefinition is used by tests to create a NetworkMap
func CreateNetworkMapFromDefinition(cl crclient.Client, def *forkliftv1.NetworkMap) error {
	err := cl.Create(context.TODO(), def, &crclient.CreateOptions{})

	if err == nil || apierrs.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func NewNetworkMap(namespace string, providerIdentifier forkliftv1.Provider, networkMapName string, sourceNicID string) *forkliftv1.NetworkMap {
	// nicPairs set with the default settings for kind CI.
	nicPairs := []forkliftv1.NetworkPair{
		{
			Source: ref.Ref{ID: sourceNicID},
			Destination: forkliftv1.DestinationNetwork{
				Type: "pod",
			},
		},
	}

	networkMap := &forkliftv1.NetworkMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "NetworkMap",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      networkMapName,
			Namespace: providerIdentifier.Namespace,
		},
		Spec: forkliftv1.NetworkMapSpec{
			Map: nicPairs,
			Provider: provider.Pair{
				Destination: corev1.ObjectReference{
					Name:      "host",
					Namespace: forklift_namespace,
				},
				Source: corev1.ObjectReference{
					Name:      providerIdentifier.Name,
					Namespace: providerIdentifier.Namespace,
				}},
		},
	}
	return networkMap
}

func WaitForNetworkMapReadyWithTimeout(cl crclient.Client, namespace, networkMapName string, timeout time.Duration) error {
	networkMapIdentifier := types.NamespacedName{Namespace: namespace, Name: networkMapName}

	returnedNetworkMap := &forkliftv1.NetworkMap{}

	err := wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, timeout, true, func(context.Context) (bool, error) {
		err := cl.Get(context.TODO(), networkMapIdentifier, returnedNetworkMap)
		if err != nil || !returnedNetworkMap.Status.Conditions.IsReady() {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("NetworkMap %s not ready within %v", networkMapName, timeout)
	}
	return nil
}
