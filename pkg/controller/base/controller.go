package base

import (
	"errors"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	FastReQ = time.Millisecond * 500
	SlowReQ = time.Second * 3
	LongReQ = time.Second * 30
)

//
// Base reconciler.
type Reconciler struct {
	record.EventRecorder
	client.Client
	Log *logging.Logger
}

//
// Reconcile started.
func (r *Reconciler) Started() {
	r.Log.Info("Reconcile started.")
}

//
// Reconcile ended.
func (r *Reconciler) Ended(reQin time.Duration, err error) (reQ time.Duration) {
	defer r.Log.Info(
		"Reconcile ended.",
		"reQ",
		reQ)
	reQ = reQin
	if err == nil {
		return
	}
	reQ = SlowReQ
	if k8serr.IsConflict(err) {
		r.Log.Info(err.Error())
		return
	}
	if errors.As(err, &web.ProviderNotReadyError{}) {
		r.Log.V(1).Info(
			"Provider inventory not ready.")
		return
	}
	r.Log.Error(
		err,
		"Reconcile failed.")

	return
}
