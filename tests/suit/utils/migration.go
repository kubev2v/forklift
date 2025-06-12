package utils

import (
	"context"
	"errors"
	"fmt"
	"time"

	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	cnv "kubevirt.io/api/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateMigrationFromDefinition is used by tests to create a Plan
func CreateMigrationFromDefinition(cl crclient.Client, def *forkliftv1.Migration) error {
	err := cl.Create(context.TODO(), def, &crclient.CreateOptions{})

	if err == nil || apierrs.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func NewMigration(namespace string, migrationName string, planName string) *forkliftv1.Migration {

	migration := &forkliftv1.Migration{
		TypeMeta: v1.TypeMeta{
			Kind:       "Migration",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      migrationName,
			Namespace: namespace,
		},
		Spec: forkliftv1.MigrationSpec{
			Plan: corev1.ObjectReference{
				Name:      planName,
				Namespace: namespace,
			},
		},
	}
	return migration
}

func WaitForMigrationSucceededWithTimeout(cl crclient.Client, namespace string, migrationName string, timeout time.Duration) error {
	migrationIdentifier := types.NamespacedName{Namespace: namespace, Name: migrationName}

	returnedMap := &forkliftv1.Migration{}

	err := wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, timeout, true, func(context.Context) (bool, error) {
		err := cl.Get(context.TODO(), migrationIdentifier, returnedMap)

		//terminate the retry if migration failed
		if condition := returnedMap.Status.Conditions.FindCondition("Failed"); condition != nil {
			return true, fmt.Errorf("migration failed %v", returnedMap.Status.VMs[0].Error.Reasons)
		}

		if err != nil || returnedMap.Status.Conditions.FindCondition("Succeeded") == nil {
			// find out the reason why migration failed
			if len(returnedMap.Status.VMs) > 0 && returnedMap.Status.VMs[0].Error != nil {
				err = fmt.Errorf("error: %s", returnedMap.Status.VMs[0].Error.Reasons)
			}

			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("migrtation %s not ready within %v, error: %w", migrationName, timeout, err)
	}
	return nil
}

func GetImportedVm(cl crclient.Client, namespace string, isImportedVm func(cnv.VirtualMachine) bool) (*cnv.VirtualMachine, error) {
	vms := &cnv.VirtualMachineList{}
	if err := cl.List(context.TODO(), vms, &crclient.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}
	for _, vm := range vms.Items {
		if isImportedVm(vm) {
			return &vm, nil
		}
	}

	return nil, errors.New("no imported VM found")
}
