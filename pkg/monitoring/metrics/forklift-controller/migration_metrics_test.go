package forklift_controller

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func counterValue(counter *prometheus.CounterVec, labels prometheus.Labels) float64 {
	m := &dto.Metric{}
	c, err := counter.GetMetricWith(labels)
	if err != nil {
		return 0
	}
	_ = c.(prometheus.Metric).Write(m)
	if m.Counter == nil {
		return 0
	}
	return m.Counter.GetValue()
}

func resetVMMetrics() {
	processedVMStatuses = make(map[string]struct{})
	migratedVMsCounter.Reset()
}

func vmWithCondition(id, condType string) *plan.VMStatus {
	return &plan.VMStatus{
		VM: plan.VM{Ref: ref.Ref{ID: id}},
		Conditions: libcnd.Conditions{
			List: []libcnd.Condition{
				{Type: condType, Status: "True"},
			},
		},
	}
}

func vmNoCondition(id string) *plan.VMStatus {
	return &plan.VMStatus{
		VM: plan.VM{Ref: ref.Ref{ID: id}},
	}
}

func migration(uid string, vms ...*plan.VMStatus) api.Migration {
	return api.Migration{
		ObjectMeta: meta.ObjectMeta{UID: types.UID(uid)},
		Status: api.MigrationStatus{
			VMs: vms,
		},
	}
}

func TestProcessVMStatuses_Succeeded(t *testing.T) {
	resetVMMetrics()

	m := migration("mig-1", vmWithCondition("vm-a", Succeeded))
	processVMStatuses(m, "vsphere", Cold, Local)

	labels := prometheus.Labels{"status": Succeeded, "provider": "vsphere", "mode": Cold, "target": Local}
	got := counterValue(migratedVMsCounter, labels)
	if got != 1 {
		t.Fatalf("expected Succeeded counter = 1, got %v", got)
	}
}

func TestProcessVMStatuses_Failed(t *testing.T) {
	resetVMMetrics()

	m := migration("mig-2", vmWithCondition("vm-b", Failed))
	processVMStatuses(m, "ovirt", Warm, Remote)

	labels := prometheus.Labels{"status": Failed, "provider": "ovirt", "mode": Warm, "target": Remote}
	got := counterValue(migratedVMsCounter, labels)
	if got != 1 {
		t.Fatalf("expected Failed counter = 1, got %v", got)
	}
}

func TestProcessVMStatuses_Canceled(t *testing.T) {
	resetVMMetrics()

	m := migration("mig-3", vmWithCondition("vm-c", Canceled))
	processVMStatuses(m, "openstack", Live, Local)

	labels := prometheus.Labels{"status": Canceled, "provider": "openstack", "mode": Live, "target": Local}
	got := counterValue(migratedVMsCounter, labels)
	if got != 1 {
		t.Fatalf("expected Canceled counter = 1, got %v", got)
	}
}

func TestProcessVMStatuses_MultipleVMs(t *testing.T) {
	resetVMMetrics()

	m := migration("mig-4",
		vmWithCondition("vm-1", Succeeded),
		vmWithCondition("vm-2", Failed),
		vmWithCondition("vm-3", Succeeded),
	)
	processVMStatuses(m, "vsphere", Cold, Local)

	succeeded := counterValue(migratedVMsCounter, prometheus.Labels{
		"status": Succeeded, "provider": "vsphere", "mode": Cold, "target": Local,
	})
	failed := counterValue(migratedVMsCounter, prometheus.Labels{
		"status": Failed, "provider": "vsphere", "mode": Cold, "target": Local,
	})
	if succeeded != 2 {
		t.Fatalf("expected Succeeded = 2, got %v", succeeded)
	}
	if failed != 1 {
		t.Fatalf("expected Failed = 1, got %v", failed)
	}
}

func TestProcessVMStatuses_SkipsNonTerminalVMs(t *testing.T) {
	resetVMMetrics()

	m := migration("mig-5",
		vmNoCondition("vm-running"),
		vmWithCondition("vm-done", Succeeded),
	)
	processVMStatuses(m, "vsphere", Cold, Local)

	succeeded := counterValue(migratedVMsCounter, prometheus.Labels{
		"status": Succeeded, "provider": "vsphere", "mode": Cold, "target": Local,
	})
	if succeeded != 1 {
		t.Fatalf("expected Succeeded = 1, got %v", succeeded)
	}
}

func TestProcessVMStatuses_Deduplication(t *testing.T) {
	resetVMMetrics()

	m := migration("mig-6", vmWithCondition("vm-x", Succeeded))
	processVMStatuses(m, "vsphere", Cold, Local)
	processVMStatuses(m, "vsphere", Cold, Local)
	processVMStatuses(m, "vsphere", Cold, Local)

	labels := prometheus.Labels{"status": Succeeded, "provider": "vsphere", "mode": Cold, "target": Local}
	got := counterValue(migratedVMsCounter, labels)
	if got != 1 {
		t.Fatalf("expected counter = 1 after 3 calls, got %v", got)
	}
}

func TestProcessVMStatuses_DifferentMigrationsNotDeduplicated(t *testing.T) {
	resetVMMetrics()

	m1 := migration("mig-7a", vmWithCondition("vm-same", Succeeded))
	m2 := migration("mig-7b", vmWithCondition("vm-same", Succeeded))
	processVMStatuses(m1, "vsphere", Cold, Local)
	processVMStatuses(m2, "vsphere", Cold, Local)

	labels := prometheus.Labels{"status": Succeeded, "provider": "vsphere", "mode": Cold, "target": Local}
	got := counterValue(migratedVMsCounter, labels)
	if got != 2 {
		t.Fatalf("expected counter = 2 for same VM in different migrations, got %v", got)
	}
}

func TestProcessVMStatuses_SameVMDifferentStatuses(t *testing.T) {
	resetVMMetrics()

	// Unusual but possible: same migration UID, same VM ID, different statuses
	// inserted across separate calls (e.g. condition list mutated between polls).
	m1 := migration("mig-8", vmWithCondition("vm-y", Succeeded))
	processVMStatuses(m1, "vsphere", Cold, Local)

	m2 := migration("mig-8", vmWithCondition("vm-y", Failed))
	processVMStatuses(m2, "vsphere", Cold, Local)

	succeeded := counterValue(migratedVMsCounter, prometheus.Labels{
		"status": Succeeded, "provider": "vsphere", "mode": Cold, "target": Local,
	})
	failed := counterValue(migratedVMsCounter, prometheus.Labels{
		"status": Failed, "provider": "vsphere", "mode": Cold, "target": Local,
	})
	if succeeded != 1 {
		t.Fatalf("expected Succeeded = 1, got %v", succeeded)
	}
	if failed != 1 {
		t.Fatalf("expected Failed = 1, got %v", failed)
	}
}
