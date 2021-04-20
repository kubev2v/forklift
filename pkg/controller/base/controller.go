package base

import (
	"errors"
	libcnd "github.com/konveyor/controller/pkg/condition"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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

//
// Record for changes in conditions.
// Logged and recorded as `Event`.
func (r *Reconciler) Record(object runtime.Object, cnd libcnd.Conditions) {
	explain := cnd.Explain()
	record := func(cnd libcnd.Condition) {
		event := ""
		switch cnd.Category {
		case libcnd.Critical,
			libcnd.Error,
			libcnd.Warn:
			event = core.EventTypeWarning
		default:
			event = core.EventTypeNormal
		}
		r.EventRecorder.Event(
			object,
			event,
			cnd.Type,
			cnd.Message)
	}
	for _, cnd := range explain.Added {
		r.Log.Info(
			"Condition added.",
			"condition",
			cnd)
		record(cnd)
	}
	for _, cnd := range explain.Updated {
		r.Log.Info(
			"Condition updated.",
			"condition",
			cnd)
		record(cnd)
	}
	for _, cnd := range explain.Deleted {
		r.Log.Info(
			"Condition deleted.",
			"condition",
			cnd)
		record(cnd)
	}
}
