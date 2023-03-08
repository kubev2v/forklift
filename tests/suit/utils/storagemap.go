package utils

import (
	"context"
	"fmt"
	"time"

	forkliftv1 "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/provider"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateStorageMapFromDefinition is used by tests to create a StorageMap
func CreateStorageMapFromDefinition(cl crclient.Client, def *forkliftv1.StorageMap) error {
	err := cl.Create(context.TODO(), def, &crclient.CreateOptions{})
	if err == nil || apierrs.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func NewStorageMap(namespace string, providerIdentifier forkliftv1.Provider, storageMapName string, storageIDs []string, storageClass string) *forkliftv1.StorageMap {

	sdPairs := []forkliftv1.StoragePair{}

	for _, sd := range storageIDs {
		pair := forkliftv1.StoragePair{
			Source: ref.Ref{ID: sd},
			Destination: forkliftv1.DestinationStorage{
				StorageClass: storageClass,
			},
		}
		sdPairs = append(sdPairs, pair)
	}

	storageMap := &forkliftv1.StorageMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "StorageMap",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      storageMapName,
			Namespace: providerIdentifier.Namespace,
		},
		Spec: forkliftv1.StorageMapSpec{
			Map: sdPairs,
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
	return storageMap
}

func WaitForStorageMapReadyWithTimeout(cl crclient.Client, namespace string, storageMapName string, timeout time.Duration) error {
	storageMapIdentifier := types.NamespacedName{Namespace: namespace, Name: storageMapName}

	returnedStorageMap := &forkliftv1.StorageMap{}

	err := wait.PollImmediate(3*time.Second, timeout, func() (bool, error) {
		err := cl.Get(context.TODO(), storageMapIdentifier, returnedStorageMap)
		if err != nil || !returnedStorageMap.Status.Conditions.IsReady() {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		//return fmt.Errorf("StorageMap %s not ready within %v", storageMapName, timeout)
		conditions := returnedStorageMap.Status.Conditions.List
		return fmt.Errorf("StorageMap %s not ready within %v - condition: %v",
			storageMapName, timeout, conditions[len(conditions)-1].Message)
	}
	return nil
}
