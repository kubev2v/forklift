package plan

import (
	"context"
	cnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/ocp"
	vsphere "github.com/konveyor/virt-controller/pkg/controller/provider/web/vsphere"
	"k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Types
const (
	SourceNotValid      = "SourceProviderNotValid"
	DestinationNotValid = "DestinationProviderNotValid"
	VmNotValid          = "VmNotValid"
)

//
// Categories
const (
	Advisory = cnd.Advisory
	Critical = cnd.Critical
	Error    = cnd.Error
	Warn     = cnd.Warn
)

// Reasons
const (
	NotSet   = "NotSet"
	NotFound = "NotFound"
)

// Statuses
const (
	True  = cnd.True
	False = cnd.False
)

// Messages
const (
	ReadyMessage               = "The migration plan is ready."
	SourceNotValidMessage      = "The `providers.source` not valid."
	DestinationNotValidMessage = "The `providers.destination` not valid."
	VmNotValidMessage          = "The vms (list) contains invalid VMs."
)

var (
	ProviderInvNotReady = liberr.New("provider inventory API not ready")
)

//
// Validate the plan resource.
func (r *Reconciler) validate(plan *api.Plan) error {
	err := r.validateProvider(plan)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = r.validateVMs(plan)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Validate provider field.
func (r *Reconciler) validateProvider(plan *api.Plan) error {
	//
	// Source
	ref := plan.Spec.Provider.Source
	if !libref.RefSet(&ref) {
		plan.Status.SetCondition(
			cnd.Condition{
				Type:     SourceNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  SourceNotValidMessage,
			})
	} else {
		provider := &api.Provider{}
		key := client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		}
		err := r.Get(context.TODO(), key, provider)
		if errors.IsNotFound(err) {
			err = nil
			plan.Status.SetCondition(
				cnd.Condition{
					Type:     SourceNotValid,
					Status:   True,
					Reason:   NotFound,
					Category: Critical,
					Message:  SourceNotValidMessage,
				})
		}
		if err != nil {
			return liberr.Wrap(err)
		}
		pClient := vsphere.Client{Provider: *provider}
		pid := path.Join(provider.Namespace, provider.Name)
		status, err := pClient.Get(&ocp.Provider{}, pid)
		if err != nil {
			return liberr.Wrap(err)
		}
		switch status {
		case http.StatusOK:
		case http.StatusNotFound:
			return ProviderInvNotReady
		}
	}
	//
	// Destination
	ref = plan.Spec.Provider.Destination
	if !libref.RefSet(&ref) {
		plan.Status.SetCondition(
			cnd.Condition{
				Type:     DestinationNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  DestinationNotValidMessage,
			})
		return nil
	} else {
		provider := &api.Provider{}
		key := client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		}
		err := r.Get(context.TODO(), key, provider)
		if errors.IsNotFound(err) {
			err = nil
			plan.Status.SetCondition(
				cnd.Condition{
					Type:     DestinationNotValid,
					Status:   True,
					Reason:   NotFound,
					Category: Critical,
					Message:  DestinationNotValidMessage,
				})
		}
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

//
// Validate listed VMs.
func (r *Reconciler) validateVMs(plan *api.Plan) error {
	ref := plan.Spec.Provider.Source
	provider := api.Provider{}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	err := r.Get(context.TODO(), key, &provider)
	if err != nil {
		if !errors.IsNotFound(err) {
			liberr.Wrap(err)
		} else {
			return nil
		}
	}
	notValid := []string{}
	var pClient web.Client
	var resource web.ClientResource
	switch provider.Type() {
	case api.VSphere:
		pClient = &vsphere.Client{Provider: provider}
		resource = &vsphere.VM{}
	default:
		return liberr.New("provider not supported.")
	}
	for _, vm := range plan.Spec.VMs {
		status, err := pClient.Get(resource, vm.ID)
		if err != nil {
			return liberr.Wrap(err)
		}
		switch status {
		case http.StatusOK:
		case http.StatusPartialContent:
			return ProviderInvNotReady
		case http.StatusNotFound:
			notValid = append(notValid, vm.ID)
		default:
			return liberr.New("")
		}
	}
	if len(notValid) > 0 {
		plan.Status.SetCondition(
			cnd.Condition{
				Type:     VmNotValid,
				Status:   True,
				Reason:   NotFound,
				Category: Critical,
				Message:  VmNotValidMessage,
				Items:    notValid,
			})
	}

	return nil
}
