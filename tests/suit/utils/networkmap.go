package utils

import (
	"context"
	forkliftv1 "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/provider"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	networkMapPollInterval = 3 * time.Second
	networkMapCreateTime   = 270 * time.Second
	dataVolumeDeleteTime   = 270 * time.Second
	dataVolumePhaseTime    = 270 * time.Second
)

// CreateNetworkMapFromDefinition is used by tests to create a NetworkMap
func CreateNetworkMapFromDefinition(cl crclient.Client, namespace string, def *forkliftv1.NetworkMap) error {
	err := wait.PollImmediate(networkMapPollInterval, networkMapCreateTime, func() (bool, error) {
		var err error
		err = cl.Create(context.TODO(), def, &crclient.CreateOptions{})

		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return err
	}
	return nil
}

func NewNetworkMap(namespace string, providerIdentifier forkliftv1.Provider, networkMapName string) *forkliftv1.NetworkMap {
	// nicPairs set with the default settings for kind CI.
	nicPairs := []forkliftv1.NetworkPair{
		{
			//TODO: externalize nicPairs
			Source: ref.Ref{ID: "6b6b7239-5ea1-4f08-a76e-be150ab8eb89"},
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
			Namespace: namespace,
		},
		Spec: forkliftv1.NetworkMapSpec{
			Map: nicPairs,
			Provider: provider.Pair{
				Destination: corev1.ObjectReference{
					Name:      "host",
					Namespace: namespace,
				},
				Source: corev1.ObjectReference{
					Name:      providerIdentifier.Name,
					Namespace: providerIdentifier.Namespace,
				}},
		},
	}
	return networkMap
}
