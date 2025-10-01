package admitters

import (
	"context"

	v1 "k8s.io/api/storage/v1"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1beta1"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/forklift-api/webhooks/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

type PlanAdmitter struct {
	Client              client.Client
	plan                api.Plan
	sourceProvider      api.Provider
	destinationProvider api.Provider
}

func (admitter *PlanAdmitter) validateStorage() error {

	if admitter.plan.IsWarm() {
		log.Info("Warm migration supports all storages, passing")
		return nil
	}

	if admitter.sourceProvider.Type() == api.VSphere {
		log.Info("Provider supports all storages, passing")
		return nil
	}

	if !admitter.destinationProvider.IsHost() {
		log.Info("Migration to a remote provider supports all storages, passing")
		return nil
	}

	storageClasses := v1.StorageClassList{}
	err := admitter.Client.List(context.TODO(), &storageClasses, &client.ListOptions{})
	if err != nil {
		log.Error(err, "Couldn't get the cluster storage classes")
		return err
	}

	storageMap := api.StorageMap{}
	err = admitter.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Map.Storage.Namespace,
			Name:      admitter.plan.Spec.Map.Storage.Name,
		},
		&storageMap)
	if err != nil {
		log.Error(err, "Couldn't get the storage map")
		return err
	}
	storagePairList := storageMap.Spec.Map
	var badStorageClasses []string
	for _, storagePair := range storagePairList {
		scName := storagePair.Destination.StorageClass
		for _, sc := range storageClasses.Items {
			if scName == sc.Name && sc.Provisioner == "kubernetes.io/no-provisioner" {
				badStorageClasses = append(badStorageClasses, sc.Name)
			}
		}
	}
	if len(badStorageClasses) > 0 {
		err := liberr.New(fmt.Sprintf("Static storage class(es) found: %v", badStorageClasses))
		log.Error(err, "Static storage class(es) found failing", "classes", badStorageClasses)
		return err
	}

	log.Info("Passed storage validation")
	return nil
}

func (admitter *PlanAdmitter) validateWarmMigrations() error {
	providerType := admitter.sourceProvider.Type()
	if providerType == api.OpenStack && admitter.plan.IsWarm() {
		err := liberr.New("warm migration is not supported by the provider")
		log.Error(err, "provider", providerType)
		return err
	}
	return nil
}

func (admitter *PlanAdmitter) validateLUKS() error {
	hasLUKS := false
	for _, vm := range admitter.plan.Spec.VMs {
		if vm.LUKS.Name != "" {
			hasLUKS = true
			break
		}
	}
	if !hasLUKS {
		return nil
	}

	providerType := admitter.sourceProvider.Type()
	if providerType != api.VSphere && providerType != api.Ova {
		err := liberr.New(fmt.Sprintf("migration of encrypted disks from source provider of type %s is not supported", providerType))
		log.Error(err, "Provider type (non-VSphere & non-OVA) does not support LUKS")
		return err
	}

	return nil
}

func (admitter *PlanAdmitter) Admit(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	log.Info("Plan admitter was called")
	raw := ar.Request.Object.Raw

	err := json.Unmarshal(raw, &admitter.plan)
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	err = admitter.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Provider.Source.Namespace,
			Name:      admitter.plan.Spec.Provider.Source.Name,
		},
		&admitter.sourceProvider)

	if err != nil {
		if admitter.plan.Spec.Archived {
			log.Info("Plan is archived, skipping validation")
			return util.ToAdmissionResponseAllow()
		} else {
			log.Error(err, "Failed to get source provider, can't determine permissions")
			return util.ToAdmissionResponseError(err)
		}
	}

	providerGR, err := api.GetGroupResource(&api.Provider{})
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	// Check whether the user has permission to access the source provider
	err = util.PermitUser(ar.Request, admitter.Client, providerGR, admitter.sourceProvider.Name, admitter.sourceProvider.Namespace, util.Get)
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	err = admitter.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Provider.Destination.Namespace,
			Name:      admitter.plan.Spec.Provider.Destination.Name,
		},
		&admitter.destinationProvider)
	if err != nil {
		log.Error(err, "Failed to get destination provider, can't determine permissions")
		return util.ToAdmissionResponseError(err)
	}

	// Check whether the user has permission to access the destination provider
	err = util.PermitUser(ar.Request, admitter.Client, providerGR, admitter.destinationProvider.Name, admitter.destinationProvider.Namespace, util.Get)
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	admitter.plan.Referenced.Provider.Source = &admitter.sourceProvider
	admitter.plan.Referenced.Provider.Destination = &admitter.destinationProvider

	if admitter.destinationProvider.IsHost() {
		// Check whether the user has permission to create VMs in the target namespace
		err = util.PermitUser(ar.Request, admitter.Client, cnv.Resource("virtualmachines"), "", admitter.plan.Spec.TargetNamespace, util.Create)
		if err != nil {
			log.Error(err, "Unable to migrate to namespace")
			return util.ToAdmissionResponseError(err)
		}
	}

	// Check whether user has permission to access the VMs from the plan
	if admitter.sourceProvider.IsHost() {
		for _, planvm := range admitter.plan.Spec.VMs {
			err = util.PermitUser(ar.Request, admitter.Client, cnv.Resource("virtualmachines"), planvm.Name, planvm.Namespace, util.Get)
			if err != nil {
				log.Error(err, "Unable to access VM")
				return util.ToAdmissionResponseError(err)
			}
		}
	}

	err = admitter.validateStorage()
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	err = admitter.validateWarmMigrations()
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	err = admitter.validateLUKS()
	if err != nil {
		return util.ToAdmissionResponseError(err)
	}

	return util.ToAdmissionResponseAllow()
}
