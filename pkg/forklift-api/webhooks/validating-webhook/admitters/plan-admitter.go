package admitters

import (
	"context"

	v1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1beta1"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks/util"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
)

type PlanAdmitter struct {
	Client              client.Client
	plan                api.Plan
	sourceProvider      api.Provider
	destinationProvider api.Provider
}

func (admitter *PlanAdmitter) validateStorage() error {

	if admitter.plan.Spec.Warm {
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
	isWarmMigration := admitter.plan.Spec.Warm
	if providerType == api.OpenStack && isWarmMigration {
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

	// coldLocal, vErr := admitter.plan.VSphereColdLocal()
	// if vErr != nil {
	// 	log.Error(vErr, "Could not analyze plan, failing")
	// 	return vErr
	// }
	// if !coldLocal {
	// 	err := liberr.New("migration of encrypted disks is not supported for warm migrations or migrations to remote providers")
	// 	log.Error(err, "Warm migration does not support LUKS")
	// 	return err
	// }
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
		log.Error(err, "Couldn't get the source provider, passing unwillingly")
		return util.ToAdmissionResponseAllow()
	}

	err = admitter.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: admitter.plan.Spec.Provider.Destination.Namespace,
			Name:      admitter.plan.Spec.Provider.Destination.Name,
		},
		&admitter.destinationProvider)
	if err != nil {
		log.Error(err, "Couldn't get the destination provider, passing unwillingly")
		return util.ToAdmissionResponseAllow()
	}

	admitter.plan.Referenced.Provider.Source = &admitter.sourceProvider
	admitter.plan.Referenced.Provider.Destination = &admitter.destinationProvider

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
