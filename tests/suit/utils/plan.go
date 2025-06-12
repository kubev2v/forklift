package utils

import (
	"context"
	"fmt"
	"time"

	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreatePlanFromDefinition is used by tests to create a Plan
func CreatePlanFromDefinition(cl crclient.Client, def *forkliftv1.Plan) error {
	err := cl.Create(context.TODO(), def, &crclient.CreateOptions{})
	if err == nil || apierrs.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func NewPlanWithVmName(providerIdentifier forkliftv1.Provider, namespace, planName, storageMap, networkMap string, vmNames []string, targetNameSpace string) *forkliftv1.Plan {
	planDef := newPlan(providerIdentifier, namespace, planName, storageMap, networkMap, targetNameSpace)
	planDef.Spec.VMs = []plan.VM{
		{
			Ref: ref.Ref{Name: vmNames[0]},
		},
	}
	return planDef
}

func NewPlanWithVmId(providerIdentifier forkliftv1.Provider, namespace, planName, storageMap, networkMap, targetNameSpace string, vmIds []string) *forkliftv1.Plan {
	planDef := newPlan(providerIdentifier, namespace, planName, storageMap, networkMap, targetNameSpace)
	planDef.Spec.VMs = []plan.VM{
		{
			Ref: ref.Ref{ID: vmIds[0]},
		},
	}
	return planDef
}

func newPlan(providerIdentifier forkliftv1.Provider, namespace, planName, storageMap, networkMap, targetNameSpace string) *forkliftv1.Plan {
	plan := &forkliftv1.Plan{
		TypeMeta: v1.TypeMeta{
			Kind:       "Plan",
			APIVersion: "forklift.konveyor.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      planName,
			Namespace: providerIdentifier.Namespace,
		},
		Spec: forkliftv1.PlanSpec{
			Provider: provider.Pair{
				Destination: corev1.ObjectReference{
					Name:      TargetProviderName,
					Namespace: providerIdentifier.Namespace,
				},
				Source: corev1.ObjectReference{
					Name:      providerIdentifier.Name,
					Namespace: providerIdentifier.Namespace,
				}},
			Archived:        false,
			Warm:            false,
			TargetNamespace: targetNameSpace,
			Map: plan.Map{
				Storage: corev1.ObjectReference{
					Name:      storageMap,
					Namespace: providerIdentifier.Namespace,
				},
				Network: corev1.ObjectReference{
					Name:      networkMap,
					Namespace: providerIdentifier.Namespace,
				},
			},
		},
	}
	return plan
}

func GetPlan(cl crclient.Client, namespace, planName string) (error, *forkliftv1.Plan) {
	planIdentifier := types.NamespacedName{Namespace: namespace, Name: planName}
	plan := &forkliftv1.Plan{}
	err := cl.Get(context.TODO(), planIdentifier, plan)
	return err, plan
}

func UpdatePlanWarmMigration(cl crclient.Client, plan *forkliftv1.Plan, isWarmMigration bool) error {
	plan.Spec.Warm = isWarmMigration
	return cl.Update(context.TODO(), plan)
}

func WaitForPlanReadyWithTimeout(cl crclient.Client, namespace, planName string, timeout time.Duration) (error, *forkliftv1.Plan) {
	planIdentifier := types.NamespacedName{Namespace: namespace, Name: planName}

	returnedMap := &forkliftv1.Plan{}

	err := wait.PollUntilContextTimeout(context.TODO(), 3*time.Second, timeout, true, func(context.Context) (bool, error) {
		err := cl.Get(context.TODO(), planIdentifier, returnedMap)
		if err != nil || !returnedMap.Status.Conditions.IsReady() {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("Plan %s not ready within %v", planName, timeout), returnedMap
	}
	return nil, returnedMap
}

func CreateNetworkAttachmentDefinition(cl crclient.Client, name, namespace string) (error, *net.NetworkAttachmentDefinition) {
	nad := &net.NetworkAttachmentDefinition{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := cl.Create(context.TODO(), nad, &crclient.CreateOptions{})
	return err, nad
}

func DeleteNetworkAttachmentDefinition(cl crclient.Client, name, namespace string) error {
	nad := &net.NetworkAttachmentDefinition{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := cl.Delete(context.TODO(), nad)
	return err
}
