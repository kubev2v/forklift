package ocp

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"time"

	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	ocp "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	cnv "kubevirt.io/api/core/v1"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	RetryDelay = time.Second * 5
)

// Cluster.
type Cluster interface {
	meta.Object
}

// An OpenShift collector.
type Collector struct {
	// The cluster CR.
	cluster Cluster
	// Credentials secret.
	secret *core.Secret
	// Logger.
	log logging.LevelLogger
	// Collections
	collections []Collection
	// A k8s non-cached client.
	client client.Client
	// cancel function.
	cancel func()
	//
	errors map[string]error
}

// New collector.
func New(
	cluster Cluster,
	secret *core.Secret,
	collections ...Collection) *Collector {
	//
	log := logging.WithName("collector|ocp").WithValues(
		"cluster",
		path.Join(
			cluster.GetNamespace(),
			cluster.GetName()))
	return &Collector{
		collections: collections,
		cluster:     cluster,
		secret:      secret,
		log:         log,
		errors:      make(map[string]error),
	}
}

// The name.
func (r *Collector) Name() string {
	return r.cluster.GetName()
}

// The owner.
func (r *Collector) Owner() meta.Object {
	return r.cluster
}

// Get the DB.
func (r *Collector) DB() libmodel.DB {
	return nil
}

// Get the Client.
func (r *Collector) Client() client.Client {
	return r.client
}

// Reset.
func (r *Collector) Reset() {
}

// Follow link
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	return fmt.Errorf("not implemented")
}

// Collector has achieved parity.
func (r *Collector) HasParity() bool {
	return true
}

// Test connection with credentials.
func (r *Collector) Test() (int, error) {
	if err := r.buildClient(); err != nil {
		return 0, err
	}

	// Test permissions for all resource types that the OCP provider needs
	testResources := []struct {
		resource string
		list     client.ObjectList
	}{
		{"namespaces", &core.NamespaceList{}},
		{"storageclasses", &storage.StorageClassList{}},
		{"network-attachment-definitions", &net.NetworkAttachmentDefinitionList{}},
		{"virtualmachines", &cnv.VirtualMachineList{}},
		{"virtualmachineinstancetypes", &instancetype.VirtualMachineInstancetypeList{}},
		{"virtualmachineclusterinstancetypes", &instancetype.VirtualMachineClusterInstancetypeList{}},
	}

	ctx := context.TODO()
	for _, resource := range testResources {
		listOptions := &client.ListOptions{Limit: 1}

		if err := r.client.List(ctx, resource.list, listOptions); err != nil {
			r.log.Info(
				"Permission test failed for resource",
				"resource", resource.resource,
				"error", err.Error())

			var statusCode int
			apiStatus, ok := err.(k8serrors.APIStatus)
			if ok {
				statusCode = int(apiStatus.Status().Code)
			} else {
				statusCode = http.StatusInternalServerError
			}
			return statusCode, liberr.New(fmt.Sprintf(
				"Failed to get resource %s: %v",
				resource.resource, err))
		}
	}

	return 0, nil
}

// Start the collector.
func (r *Collector) Start() error {
	ctx := context.Background()
	ctx, r.cancel = context.WithCancel(ctx)
	for _, collection := range r.collections {
		collection.Bind(r)
	}
	start := func() {
	try:
		for {
			select {
			case <-ctx.Done():
				break try
			default:
				err := r.start()
				if err != nil {
					r.log.V(3).Error(
						err,
						"start failed.",
						"retry",
						RetryDelay)
					time.Sleep(RetryDelay)
					continue try
				}
				break try
			}
		}
	}

	go start()

	return nil
}

// Start details.
//  1. Build and start the manager.
//  2. Reconcile all of the collections.
//  3. Mark parity.
//  4. Start apply events (coroutine).
func (r *Collector) start() (err error) {
	mark := time.Now()
	r.log.V(3).Info("starting.")
	err = r.buildClient()
	if err != nil {
		return
	}

	r.log.V(3).Info(
		"started.",
		"duration",
		time.Since(mark))

	return
}

// Shutdown the collector.
//  1. Close manager stop channel.
//  2. Close watch event coroutine channel.
//  3. Cancel the context.
func (r *Collector) Shutdown() {
	r.log.V(3).Info("shutdown.")
	if r.cancel != nil {
		r.cancel()
	}
}

// Build non-cached client.
func (r *Collector) buildClient() (err error) {
	provider := r.cluster.(*api.Provider)
	r.client, err = client.New(
		ocp.RestCfg(provider, r.secret),
		client.Options{
			Scheme: scheme.Scheme,
		})

	return
}

func (r *Collector) ClearError(kind string) {
	delete(r.errors, kind)
}

func (r *Collector) SetError(kind string, err error) {
	r.errors[kind] = err
}

// returns a copy of all current errors that were encountered while querying the inventory
func (r *Collector) GetRuntimeErrors() map[string]error {
	errors := make(map[string]error)
	for k, v := range r.errors {
		errors[k] = v
	}
	return errors
}
