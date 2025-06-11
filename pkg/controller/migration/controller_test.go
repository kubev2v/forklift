package migration

import (
	"context"
	"reflect"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconciler_Reconcile(t *testing.T) {
	type fields struct {
		Reconciler base.Reconciler
	}
	type args struct {
		ctx     context.Context
		request reconcile.Request
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantResult reconcile.Result
		wantErr    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Reconciler{
				Reconciler: tt.fields.Reconciler,
			}
			gotResult, err := r.Reconcile(tt.args.ctx, tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Reconcile() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestReconciler_reflectPlan(t *testing.T) {
	type fields struct {
		Reconciler base.Reconciler
	}
	type args struct {
		plan      *api.Plan
		migration *api.Migration
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				Reconciler: tt.fields.Reconciler,
			}
			r.reflectPlan(tt.args.plan, tt.args.migration)
		})
	}
}

func TestReconciler_validate(t *testing.T) {
	type fields struct {
		Reconciler base.Reconciler
	}
	type args struct {
		migration *api.Migration
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantPlan *api.Plan
		wantErr  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{
				Reconciler: tt.fields.Reconciler,
			}
			gotPlan, err := r.validate(tt.args.migration)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPlan, tt.wantPlan) {
				t.Errorf("validate() gotPlan = %v, want %v", gotPlan, tt.wantPlan)
			}
		})
	}
}
