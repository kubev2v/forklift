package provider

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/copyappliance"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const checkName = "vcenter-copy-appliance-template-check"

// withCopyApplianceTemplateSettings configures the feature for the duration of a test. The
// settings are global, and other tests in this package leave them set.
func withCopyApplianceTemplateSettings(t *testing.T) {
	t.Helper()
	previous := settings.Settings
	t.Cleanup(func() { settings.Settings = previous })
	Settings.Features.CopyAppliance = true
	Settings.BaseDiskContainerImage = "registry.example/rhel:9"
	Settings.ContainerImage = "copy-appliance:latest"
}

// checkProvider is a vSphere provider. It carries no conditions: they are set
// during the pass, by runPass() below.
func checkProvider() *api.Provider {
	vsphere := api.VSphere
	return &api.Provider{
		ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "vcenter", UID: "provider-uid"},
		Spec: api.ProviderSpec{
			Type: &vsphere,
			Settings: map[string]string{
				api.CopyApplianceDatastore: "ds1",
				api.CopyApplianceFolder:    "/DC0/vm",
				api.CopyApplianceNetwork:   "VM Network",
			},
		},
		Status: api.ProviderStatus{
			CopyApplianceSSHPrivateSecret: "copy-appliance-ssh-keys-vcenter-private",
			CopyApplianceSSHPublicSecret:  "copy-appliance-ssh-keys-vcenter-public",
		},
	}
}

// checkTemplate is the provider's copy appliance template, built and imported.
func checkTemplate() *api.CopyApplianceTemplate {
	return &api.CopyApplianceTemplate{
		ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "vcenter-copy-appliance-template"},
		Spec:       api.CopyApplianceTemplateSpec{TemplateName: "vcenter-copy-appliance-template", Folder: "/DC0/vm"},
		Status: api.CopyApplianceTemplateStatus{
			Phase: api.CopyApplianceTemplatePhaseSucceeded,
			Template: api.TemplateStatus{
				Moref:      "vm-900",
				DiskHash:   "disk-1",
				ConfigHash: "config-1",
			},
		},
	}
}

// checkAppliance is a check appliance in the given phase, carrying the
// finalizer the copy appliance controller adds. The name is what the API
// server would have generated; the labels are what the check finds it by.
func checkAppliance(phase string) *api.CopyAppliance {
	labeler := copyappliance.Labeler{}
	appliance := &api.CopyAppliance{
		ObjectMeta: meta.ObjectMeta{
			Namespace:         "forklift",
			Name:              checkName,
			Labels:            labeler.CheckLabels(checkProvider()),
			Finalizers:        []string{api.CopyApplianceFinalizer},
			CreationTimestamp: meta.Now(),
		},
	}
	appliance.Status.Phase = phase
	return appliance
}

// readyCheckAppliance is DeployCompleted with Ready=True — what IsDeployReady
// (and the plan wait) requires before treating an appliance as usable.
func readyCheckAppliance() *api.CopyAppliance {
	appliance := checkAppliance(api.PhaseDeployCompleted)
	appliance.Status.SetCondition(libcnd.Condition{
		Type:   libcnd.Ready,
		Status: True,
	})
	return appliance
}

func testCheckScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme core: %v", err)
	}
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme forklift: %v", err)
	}
	return scheme
}

// testCheckOn builds a check over a client the caller has made, for tests that
// need the client to misbehave.
func testCheckOn(t *testing.T, cl client.Client, provider *api.Provider) *applianceCheck {
	t.Helper()
	check := newApplianceCheck(cl, provider)
	// Stands in for the real builder, which reads the inventory service. Only
	// the appliance's identity matters to the check.
	check.build = func(provider *api.Provider, _ *api.CopyApplianceTemplate) (*api.CopyAppliance, error) {
		labeler := copyappliance.Labeler{}
		return &api.CopyAppliance{
			ObjectMeta: meta.ObjectMeta{
				Namespace:    provider.Namespace,
				GenerateName: "forklift-copy-check-",
				Labels:       labeler.CheckLabels(provider),
			},
		}, nil
	}
	return check
}

// testCheck builds a check over a fake client holding objs.
func testCheck(t *testing.T, provider *api.Provider, objs ...client.Object) *applianceCheck {
	t.Helper()
	scheme := testCheckScheme(t)
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&api.Provider{}, &api.CopyAppliance{}, &api.CopyApplianceTemplate{}).
		Build()
	return testCheckOn(t, cl, provider)
}

// runPass runs the check the way Reconcile does, inside a staging window. The
// staging is the point: it is what decides which conditions survive to the
// next pass. The two conditions the check gates on are re-set here because
// that is what validate and updateContainer do earlier in the same pass —
// neither is durable, so neither is visible until it is set again.
func runPass(t *testing.T, c *applianceCheck) error {
	t.Helper()
	c.provider.Status.BeginStagingConditions()
	c.provider.Status.SetCondition(
		libcnd.Condition{Type: ConnectionTestSucceeded, Status: True, Category: Required},
		libcnd.Condition{Type: InventoryCreated, Status: True, Category: Required})
	err := c.Run(context.TODO())
	c.provider.Status.EndStagingConditions()
	return err
}

// checkAppliances is every appliance in the fake client. Appliances are found
// by label rather than by name, so the assertions list rather than get.
func checkAppliances(t *testing.T, c *applianceCheck) []api.CopyAppliance {
	t.Helper()
	list := &api.CopyApplianceList{}
	if err := c.client.List(context.TODO(), list); err != nil {
		t.Fatalf("list appliances: %v", err)
	}
	return list.Items
}

// theCheckAppliance is the one appliance the check has to work with.
func theCheckAppliance(t *testing.T, c *applianceCheck) *api.CopyAppliance {
	t.Helper()
	items := checkAppliances(t, c)
	if len(items) != 1 {
		t.Fatalf("found %d check appliances, want one", len(items))
	}
	return &items[0]
}

// applianceState reports what became of the check appliance.
func applianceState(t *testing.T, c *applianceCheck) string {
	t.Helper()
	live, terminating := 0, 0
	for _, appliance := range checkAppliances(t, c) {
		if appliance.DeletionTimestamp != nil {
			terminating++
		} else {
			live++
		}
	}
	switch {
	case live > 0:
		return "present"
	case terminating > 0:
		return "terminating"
	default:
		return "gone"
	}
}

func TestCopyApplianceTemplateApplianceCheck(t *testing.T) {
	failed := checkAppliance(api.PhaseDeployFailed)
	failed.Status.SetCondition(libcnd.Condition{
		Type:     libcnd.Ready,
		Status:   False,
		Category: Error,
		Message:  "the guest never reported an address",
	})

	released := readyCheckAppliance()
	released.Status.Phase = api.PhaseReleased

	stale := checkAppliance(api.PhaseWaitForNetwork)
	stale.CreationTimestamp = meta.NewTime(time.Now().Add(-2 * copyApplianceCheckDeadline))

	terminating := checkAppliance(api.PhaseWaitForClone)
	deleted := meta.Now()
	terminating.DeletionTimestamp = &deleted

	tests := []struct {
		name string
		// recorded is a verdict left by an earlier pass.
		recorded *libcnd.Condition
		// objects beyond the provider and its template.
		objects []client.Object
		// template replaces the built and imported one.
		template *api.CopyApplianceTemplate
		// noTemplate skips creating a CopyApplianceTemplate in the fake client.
		noTemplate bool

		wantBlockedBy string // reason of CopyApplianceCheckNotReady, "" for ready
		wantRecorded  string // reason of CopyApplianceChecked, "" for none
		wantMessage   string // substring of whichever condition was set
		wantAppliance string
	}{
		{
			name:          "a provider that has never been checked deploys an appliance",
			wantBlockedBy: CopyApplianceCheckPending,
			wantMessage:   "started",
			wantAppliance: "present",
		},
		{
			name:          "an appliance that came up records a pass and is torn down",
			objects:       []client.Object{readyCheckAppliance()},
			wantRecorded:  CopyApplianceCheckPassed,
			wantMessage:   "passed",
			wantAppliance: "terminating",
		},
		{
			name:          "a released check appliance is still a pass",
			objects:       []client.Object{released},
			wantRecorded:  CopyApplianceCheckPassed,
			wantMessage:   "passed",
			wantAppliance: "terminating",
		},
		{
			name:          "an appliance that failed records why",
			objects:       []client.Object{failed},
			wantBlockedBy: CopyApplianceCheckFailed,
			wantRecorded:  CopyApplianceCheckFailed,
			wantMessage:   "the guest never reported an address",
			wantAppliance: "terminating",
		},
		{
			name:          "an appliance still converging is waited on",
			objects:       []client.Object{checkAppliance(api.PhaseWaitForNetwork)},
			wantBlockedBy: CopyApplianceCheckPending,
			wantMessage:   api.PhaseWaitForNetwork,
			wantAppliance: "present",
		},
		{
			name:          "an appliance that never converged is failed rather than waited on forever",
			objects:       []client.Object{stale},
			wantBlockedBy: CopyApplianceCheckFailed,
			wantRecorded:  CopyApplianceCheckFailed,
			wantMessage:   "did not come up within 30m0s",
			wantAppliance: "terminating",
		},
		{
			// The name is generated, so a replacement does not collide with
			// the one still going away.
			name:          "a teardown in progress does not hold up the next check",
			objects:       []client.Object{terminating},
			wantBlockedBy: CopyApplianceCheckPending,
			wantMessage:   "started",
			wantAppliance: "present",
		},
		{
			name: "a recorded pass for the same inputs deploys nothing",
			recorded: &libcnd.Condition{
				Reason: CopyApplianceCheckPassed,
				Items:  []string{"vm-900", "disk-1", "config-1", "copy-appliance:latest"},
			},
			wantRecorded:  CopyApplianceCheckPassed,
			wantAppliance: "gone",
		},
		{
			name: "a recorded failure for the same inputs keeps blocking without redeploying",
			recorded: &libcnd.Condition{
				Reason:  CopyApplianceCheckFailed,
				Message: "the guest never reported an address",
				Items:   []string{"vm-900", "disk-1", "config-1", "copy-appliance:latest"},
			},
			wantBlockedBy: CopyApplianceCheckFailed,
			wantRecorded:  CopyApplianceCheckFailed,
			wantMessage:   "the guest never reported an address",
			wantAppliance: "gone",
		},
		{
			name: "a rebuilt template invalidates the recorded pass",
			recorded: &libcnd.Condition{
				Reason: CopyApplianceCheckPassed,
				Items:  []string{"vm-800", "disk-0", "config-1", "copy-appliance:latest"},
			},
			wantBlockedBy: CopyApplianceCheckPending,
			wantMessage:   "started",
			wantAppliance: "present",
		},
		{
			name:          "a template that has not been built yet is waited on",
			template:      &api.CopyApplianceTemplate{ObjectMeta: meta.ObjectMeta{Namespace: "forklift", Name: "vcenter-copy-appliance-template"}},
			wantBlockedBy: CopyApplianceCheckPending,
			wantMessage:   "Waiting for the copy appliance template",
			wantAppliance: "gone",
		},
		{
			name:          "a missing template does not block the provider",
			noTemplate:    true,
			wantAppliance: "gone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withCopyApplianceTemplateSettings(t)
			provider := checkProvider()
			if tt.recorded != nil {
				recorded := *tt.recorded
				recorded.Type = CopyApplianceChecked
				recorded.Status = True
				recorded.Category = Advisory
				recorded.Durable = true
				provider.Status.SetCondition(recorded)
			}
			objects := []client.Object{provider}
			if !tt.noTemplate {
				template := tt.template
				if template == nil {
					template = checkTemplate()
				}
				objects = append(objects, template)
			}
			objects = append(objects, tt.objects...)
			c := testCheck(t, provider, objects...)

			if err := runPass(t, c); err != nil {
				t.Fatalf("Run: %v", err)
			}

			blocker := provider.Status.FindCondition(CopyApplianceCheckNotReady)
			switch {
			case tt.wantBlockedBy == "" && blocker != nil:
				t.Errorf("blocked by %s: %s, want the provider unblocked", blocker.Reason, blocker.Message)
			case tt.wantBlockedBy != "" && blocker == nil:
				t.Errorf("not blocked, want %s", tt.wantBlockedBy)
			case blocker != nil:
				if blocker.Reason != tt.wantBlockedBy {
					t.Errorf("blocked by %s, want %s", blocker.Reason, tt.wantBlockedBy)
				}
				if blocker.Category != Critical {
					t.Errorf("blocker category = %s, want %s", blocker.Category, Critical)
				}
			}
			if (blocker != nil) != provider.Status.HasBlockerCondition() {
				t.Errorf("HasBlockerCondition = %v, want %v",
					provider.Status.HasBlockerCondition(), blocker != nil)
			}

			record := provider.Status.FindCondition(CopyApplianceChecked)
			switch {
			case tt.wantRecorded == "" && record != nil:
				t.Errorf("recorded %s, want no record", record.Reason)
			case tt.wantRecorded != "" && record == nil:
				t.Errorf("no record, want %s", tt.wantRecorded)
			case record != nil:
				if record.Reason != tt.wantRecorded {
					t.Errorf("recorded %s, want %s", record.Reason, tt.wantRecorded)
				}
				// Advisory and durable, or it would block the next pass
				// before updateContainer could run.
				if record.Category != Advisory || !record.Durable {
					t.Errorf("record is %s durable=%v, want %s durable=true",
						record.Category, record.Durable, Advisory)
				}
			}

			if tt.wantMessage != "" {
				message := ""
				if record != nil {
					message = record.Message
				}
				if blocker != nil {
					message = blocker.Message
				}
				if !strings.Contains(message, tt.wantMessage) {
					t.Errorf("message = %q, want it to contain %q", message, tt.wantMessage)
				}
			}

			if got := applianceState(t, c); got != tt.wantAppliance {
				t.Errorf("appliance is %s, want %s", got, tt.wantAppliance)
			}
		})
	}
}

// The whole point of the record is that the check does not run again. This
// drives two passes through staging, which is what would drop a record that was
// not durable.
func TestCopyApplianceTemplateApplianceCheckRunsOnce(t *testing.T) {
	withCopyApplianceTemplateSettings(t)
	provider := checkProvider()
	c := testCheck(t, provider, provider, checkTemplate(),
		readyCheckAppliance())

	if err := runPass(t, c); err != nil {
		t.Fatalf("first pass: %v", err)
	}

	record := provider.Status.FindCondition(CopyApplianceChecked)
	if record == nil || record.Reason != CopyApplianceCheckPassed {
		t.Fatalf("first pass recorded %v, want a pass", record)
	}
	if provider.Status.HasBlockerCondition() {
		t.Fatal("the provider is blocked after a passing check")
	}

	// The appliance is terminating rather than gone, because the fake client
	// honours its finalizer. Release it the way its own controller would.
	appliance := theCheckAppliance(t, c)
	appliance.Finalizers = nil
	if err := c.client.Update(context.TODO(), appliance); err != nil {
		t.Fatalf("release finalizer: %v", err)
	}

	if err := runPass(t, c); err != nil {
		t.Fatalf("second pass: %v", err)
	}

	if provider.Status.FindCondition(CopyApplianceChecked) == nil {
		t.Error("the record did not survive the second pass; is it durable?")
	}
	if provider.Status.HasBlockerCondition() {
		t.Error("the provider is blocked on the second pass")
	}
	if got := applianceState(t, c); got != "gone" {
		t.Errorf("appliance is %s on the second pass, want it not redeployed", got)
	}
}

// A settled check used to fire a Delete on every pass, whether or not there was
// anything left to delete: a write per provider per reconcile, forever.
func TestCopyApplianceTemplateCheckDoesNotDeleteWhatIsNotThere(t *testing.T) {
	withCopyApplianceTemplateSettings(t)
	provider := checkProvider()
	provider.Status.SetCondition(libcnd.Condition{
		Type:     CopyApplianceChecked,
		Status:   True,
		Reason:   CopyApplianceCheckPassed,
		Category: Advisory,
		Durable:  true,
		Items:    []string{"vm-900", "disk-1", "config-1", "copy-appliance:latest"},
	})

	deletes := 0
	scheme := testCheckScheme(t)
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(provider, checkTemplate()).
		WithStatusSubresource(&api.Provider{}, &api.CopyAppliance{}, &api.CopyApplianceTemplate{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				deletes++
				return cl.Delete(ctx, obj, opts...)
			},
		}).
		Build()
	c := testCheckOn(t, cl, provider)

	for i := range 2 {
		if err := runPass(t, c); err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
	}

	if deletes != 0 {
		t.Errorf("issued %d deletes, want none: there is no appliance to delete", deletes)
	}
}

// A template that cannot be read is not a template that is still being built,
// and saying so is the only way the user finds out the apiserver is refusing.
func TestCopyApplianceTemplateCheckReportsATemplateReadFailure(t *testing.T) {
	withCopyApplianceTemplateSettings(t)
	provider := checkProvider()
	scheme := testCheckScheme(t)
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(provider, checkTemplate()).
		WithStatusSubresource(&api.Provider{}, &api.CopyAppliance{}, &api.CopyApplianceTemplate{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, isTemplate := obj.(*api.CopyApplianceTemplate); isTemplate {
					return errors.New("etcd is unavailable")
				}
				return cl.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	c := testCheckOn(t, cl, provider)

	err := runPass(t, c)

	if err == nil {
		t.Fatal("Run succeeded although the template could not be read")
	}
	blocker := provider.Status.FindCondition(CopyApplianceCheckNotReady)
	if blocker == nil {
		t.Fatal("the provider is not blocked")
	}
	if !strings.Contains(blocker.Message, "etcd is unavailable") {
		t.Errorf("message = %q, want it to name the real failure", blocker.Message)
	}
	if strings.Contains(blocker.Message, "Waiting for the copy appliance template") {
		t.Errorf("message = %q, want it not to claim the template is still building", blocker.Message)
	}
}

func TestCopyApplianceTemplateCheckReportsAnApplianceReadFailure(t *testing.T) {
	withCopyApplianceTemplateSettings(t)
	provider := checkProvider()
	scheme := testCheckScheme(t)
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(provider, checkTemplate()).
		WithStatusSubresource(&api.Provider{}, &api.CopyAppliance{}, &api.CopyApplianceTemplate{}).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, isAppliance := list.(*api.CopyApplianceList); isAppliance {
					return errors.New("etcd is unavailable")
				}
				return cl.List(ctx, list, opts...)
			},
		}).
		Build()
	c := testCheckOn(t, cl, provider)

	err := runPass(t, c)

	if err == nil {
		t.Fatal("Run succeeded although the appliance could not be read")
	}
	blocker := provider.Status.FindCondition(CopyApplianceCheckNotReady)
	if blocker == nil {
		t.Fatal("the provider is not blocked")
	}
	if !strings.Contains(blocker.Message, "etcd is unavailable") {
		t.Errorf("message = %q, want it to name the real failure", blocker.Message)
	}
}

// Neither condition may block at the top of a pass. updateContainer and the
// copy appliance template sync both bail on HasBlockerCondition and both run before
// the check does, so a recorded failure that blocked would stop the inventory
// from updating and stop the template from ever being rebuilt to fix the very
// failure that was recorded. The record stays out of the way by being
// Advisory; the blocker stays out of the way by not being durable.
func TestCopyApplianceTemplateCheckDoesNotBlockTheStartOfTheNextPass(t *testing.T) {
	provider := checkProvider()
	c := &applianceCheck{provider: provider}
	items := []string{"vm-900", "disk-1", "config-1", "copy-appliance:latest"}
	c.fail(items, "template vm-900 (disk disk-1, config config-1), appliance image copy-appliance:latest",
		"the guest never reported an address")

	if !provider.Status.HasBlockerCondition() {
		t.Fatal("precondition: the failed provider is not blocked")
	}

	provider.Status.BeginStagingConditions()

	if provider.Status.HasBlockerCondition() {
		t.Error("the provider is blocked before the check has re-run; " +
			"updateContainer and the template sync will not run")
	}
	if provider.Status.FindCondition(CopyApplianceChecked) == nil {
		t.Error("the record is not readable during staging; is it durable?")
	}
}

func TestCopyApplianceTemplateCheckConditions(t *testing.T) {
	items := []string{"vm-900", "disk-1", "config-1", "copy-appliance:latest"}
	describe := "template vm-900 (disk disk-1, config config-1), appliance image copy-appliance:latest"

	blocked := &applianceCheck{provider: checkProvider()}
	blocked.block(CopyApplianceCheckPending, "waiting")
	switch cnd := blocked.provider.Status.FindCondition(CopyApplianceCheckNotReady); {
	case cnd == nil:
		t.Error("block set no condition")
	case cnd.Category != Critical:
		t.Errorf("blocker category = %s, want %s", cnd.Category, Critical)
	case cnd.Durable:
		t.Error("the blocker is durable; it would survive staging and stop the next pass")
	}

	passed := &applianceCheck{provider: checkProvider()}
	passed.pass(items, describe)
	switch cnd := passed.provider.Status.FindCondition(CopyApplianceChecked); {
	case cnd == nil:
		t.Error("pass set no condition")
	case cnd.Category != Advisory:
		t.Errorf("record category = %s, want %s", cnd.Category, Advisory)
	case !cnd.Durable:
		t.Error("the record is not durable; the check would run again every pass")
	case !slices.Equal(cnd.Items, items):
		t.Errorf("record items = %v, want %v", cnd.Items, items)
	}
	if passed.provider.Status.FindCondition(CopyApplianceCheckNotReady) != nil {
		t.Error("a passing check blocked the provider")
	}

	failed := &applianceCheck{provider: checkProvider()}
	failed.fail(items, describe, "the guest never reported an address")
	record := failed.provider.Status.FindCondition(CopyApplianceChecked)
	blocker := failed.provider.Status.FindCondition(CopyApplianceCheckNotReady)
	if record == nil || blocker == nil {
		t.Fatalf("fail set record=%v blocker=%v, want both", record, blocker)
	}
	if record.Message != blocker.Message {
		t.Errorf("record says %q but the blocker says %q", record.Message, blocker.Message)
	}
	if !strings.Contains(record.Message, "the guest never reported an address") {
		t.Errorf("message = %q, want it to carry the reason", record.Message)
	}
	for _, item := range record.Items {
		if !strings.Contains(record.Message, item) {
			t.Errorf("item %q is not referenced in the message %q", item, record.Message)
		}
	}
}

// A provider being deleted has nothing to validate, and adding a blocker to it
// would only get in the way of the finalizers running.
func TestCopyApplianceTemplateApplianceCheckSkipsADeletedProvider(t *testing.T) {
	withCopyApplianceTemplateSettings(t)
	provider := checkProvider()
	deleted := meta.Now()
	provider.DeletionTimestamp = &deleted
	provider.Finalizers = []string{"forklift"}
	c := testCheck(t, provider, provider, checkTemplate())

	if err := runPass(t, c); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if provider.Status.HasBlockerCondition() {
		t.Error("a provider being deleted was blocked by the appliance check")
	}
	if got := applianceState(t, c); got != "gone" {
		t.Errorf("appliance is %s, want none deployed for a deleted provider", got)
	}
}

// The appliance is owned by the provider, so deleting the provider takes any
// appliance it left behind with it.
func TestCopyApplianceTemplateCheckApplianceIsOwnedByTheProvider(t *testing.T) {
	withCopyApplianceTemplateSettings(t)
	provider := checkProvider()
	c := testCheck(t, provider, provider, checkTemplate())

	if err := runPass(t, c); err != nil {
		t.Fatalf("Run: %v", err)
	}

	owner := meta.GetControllerOf(theCheckAppliance(t, c))
	if owner == nil || owner.UID != provider.UID {
		t.Errorf("owner = %v, want the provider", owner)
	}
}
