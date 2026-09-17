/*
Copyright 2026 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package virtualizationvalidation reconciles hub-local target virtualization checks.
package virtualizationvalidation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/kubev2v/forklift/pkg/settings"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	Name                 = "virtualization-validation"
	ValidationImageEnv   = "VIRT_VALIDATION_IMAGE"
	ResultConfigMapKey   = "report.json"
	ValidationLabel      = "forklift.konveyor.io/virtualization-validation"
	ValidationInputLabel = "forklift.konveyor.io/validation-input"
	ProviderInProgress   = "VirtualizationValidationInProgress"
	ProviderSucceeded    = "VirtualizationValidationSucceeded"
	ProviderFailed       = "VirtualizationValidationFailed"
	ProviderUnavailable  = "VirtualizationUnavailable"
	ValidationFinalizer  = "forklift.konveyor.io/virtualization-validation"
)

var log = logging.WithName(Name)
var Settings = &settings.Settings

func Add(mgr manager.Manager) error {
	r := &Reconciler{Reconciler: base.Reconciler{
		EventRecorder: mgr.GetEventRecorderFor(Name),
		Client:        mgr.GetClient(),
		Log:           log,
	}, Scheme: mgr.GetScheme()}
	c, err := controller.New(Name, mgr, controller.Options{
		Reconciler: r, MaxConcurrentReconciles: Settings.MaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}
	if err := c.Watch(source.Kind(mgr.GetCache(), &api.VirtualizationValidation{},
		&handler.TypedEnqueueRequestForObject[*api.VirtualizationValidation]{},
		predicate.TypedGenerationChangedPredicate[*api.VirtualizationValidation]{})); err != nil {
		return err
	}
	for _, object := range []runtime.Object{&batch.Job{}, &core.ConfigMap{}} {
		if err := c.Watch(source.Kind(mgr.GetCache(), object.(client.Object),
			handler.TypedEnqueueRequestForOwner[client.Object](mgr.GetScheme(), mgr.GetRESTMapper(), &api.VirtualizationValidation{}, handler.OnlyControllerOwner()))); err != nil {
			return err
		}
	}
	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

type Reconciler struct {
	base.Reconciler
	Scheme *runtime.Scheme
}

func (r Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, err error) {
	v := &api.VirtualizationValidation{}
	if err = r.Get(ctx, request.NamespacedName, v); err != nil {
		if k8serr.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return
	}
	if !v.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, r.finalize(ctx, v)
	}
	if controllerutil.AddFinalizer(v, ValidationFinalizer) {
		return reconcile.Result{}, r.Update(ctx, v)
	}
	defer func() {
		v.Status.ObservedGeneration = v.Generation
		if updateErr := r.Status().Update(ctx, v); updateErr != nil && err == nil {
			err = updateErr
		}
	}()

	v.Status.BeginStagingConditions()
	defer v.Status.EndStagingConditions()

	if v.Spec.Profile != "" && v.Spec.Profile != api.VirtualizationValidationProviderProfile {
		r.failed(v, "UnsupportedProfile", "only the Provider validation profile is implemented")
		return
	}
	checks, validationErr := providerProfileChecks(v.Spec.Checks)
	if validationErr != nil {
		r.failed(v, "UnsupportedCheck", validationErr.Error())
		return
	}
	if v.Spec.ProviderRef.Name == "" {
		r.failed(v, "ProviderNotSet", "spec.providerRef.name is required")
		return
	}
	provider := &api.Provider{}
	providerNamespace := v.Spec.ProviderRef.Namespace
	if providerNamespace == "" {
		providerNamespace = v.Namespace
	}
	if err = r.Get(ctx, types.NamespacedName{Namespace: providerNamespace, Name: v.Spec.ProviderRef.Name}, provider); err != nil {
		if k8serr.IsNotFound(err) {
			r.failed(v, "ProviderNotFound", "referenced Provider was not found")
			err = nil
		}
		return
	}
	if provider.Type() != api.OpenShift {
		r.failed(v, "UnsupportedProvider", "the Provider must have type openshift")
		return
	}
	if provider.Spec.URL == "" {
		r.failed(v, "HostProviderUnsupported", "hub host Providers are not supported by the initial validation profile")
		return
	}
	secretNamespace := provider.Spec.Secret.Namespace
	if secretNamespace == "" {
		secretNamespace = provider.Namespace
	}
	secret := &core.Secret{}
	if err = r.Get(ctx, types.NamespacedName{Namespace: secretNamespace, Name: provider.Spec.Secret.Name}, secret); err != nil {
		if k8serr.IsNotFound(err) {
			r.failed(v, "ProviderSecretNotFound", "referenced Provider credential Secret was not found")
			err = nil
		}
		return
	}
	if len(secret.Data[api.Token]) == 0 {
		r.failed(v, "ProviderTokenNotFound", "referenced Provider credential Secret has no token")
		return
	}
	image := os.Getenv(ValidationImageEnv)
	if image == "" {
		r.failed(v, "ValidationImageNotConfigured", ValidationImageEnv+" is not configured")
		return
	}
	hash := inputHash(v, provider, secret, image, checks)
	if v.Status.ObservedInputHash != "" && v.Status.ObservedInputHash != hash {
		v.Status = api.VirtualizationValidationStatus{ObservedInputHash: hash}
		v.Status.BeginStagingConditions()
	}
	v.Status.ObservedInputHash, v.Status.Image = hash, image

	name := resourceName(v, hash)
	credentials := validationCredentials(v, name, secret)
	if err = controllerutil.SetControllerReference(v, credentials, r.Scheme); err != nil {
		return
	}
	if err = r.Create(ctx, credentials); err != nil && !k8serr.IsAlreadyExists(err) {
		return
	}
	serviceAccount := validationServiceAccount(v, name)
	if err = controllerutil.SetControllerReference(v, serviceAccount, r.Scheme); err != nil {
		return
	}
	if err = r.Create(ctx, serviceAccount); err != nil && !k8serr.IsAlreadyExists(err) {
		return
	}
	role := resultConfigMapRole(v, name)
	if err = controllerutil.SetControllerReference(v, role, r.Scheme); err != nil {
		return
	}
	if err = r.Create(ctx, role); err != nil && !k8serr.IsAlreadyExists(err) {
		return
	}
	roleBinding := resultConfigMapRoleBinding(v, name)
	if err = controllerutil.SetControllerReference(v, roleBinding, r.Scheme); err != nil {
		return
	}
	if err = r.Create(ctx, roleBinding); err != nil && !k8serr.IsAlreadyExists(err) {
		return
	}
	cm := resultConfigMap(v, name)
	if err = controllerutil.SetControllerReference(v, cm, r.Scheme); err != nil {
		return
	}
	if err = r.Create(ctx, cm); err != nil && !k8serr.IsAlreadyExists(err) {
		return
	}
	if err = r.Get(ctx, types.NamespacedName{Namespace: v.Namespace, Name: name}, cm); err != nil {
		return
	}
	v.Status.ResultRef = &core.ObjectReference{Kind: "ConfigMap", Namespace: cm.Namespace, Name: cm.Name}
	reportComplete, err := r.observeReport(v, cm)
	if err != nil {
		r.failed(v, "InvalidReport", err.Error())
		return
	}

	job := validatorJob(v, provider, credentials, image, name, checks)
	if err = controllerutil.SetControllerReference(v, job, r.Scheme); err != nil {
		return
	}
	if err = r.Create(ctx, job); err != nil && !k8serr.IsAlreadyExists(err) {
		return
	}
	if err = r.Get(ctx, types.NamespacedName{Namespace: v.Namespace, Name: name}, job); err != nil {
		return
	}
	v.Status.JobRef = &core.ObjectReference{Kind: "Job", Namespace: job.Namespace, Name: job.Name}

	r.observeJob(v, job, reportComplete)
	if err = r.projectProviderStatus(ctx, provider, v); err != nil {
		return
	}
	return
}

func (r Reconciler) projectProviderStatus(ctx context.Context, provider *api.Provider, v *api.VirtualizationValidation) error {
	var desired libcnd.Condition
	switch v.Status.Phase {
	case api.VirtualizationValidationPending, api.VirtualizationValidationRunning:
		desired = libcnd.Condition{Type: ProviderInProgress, Status: libcnd.True, Reason: validationConditionReason(v), Category: api.CategoryAdvisory, Durable: true,
			Message: fmt.Sprintf("Virtualization validation %q is in progress.", v.Name)}
	case api.VirtualizationValidationSucceeded:
		desired = libcnd.Condition{Type: ProviderSucceeded, Status: libcnd.True, Reason: validationConditionReason(v), Category: api.CategoryAdvisory, Durable: true,
			Message: fmt.Sprintf("Virtualization validation %q completed successfully.", v.Name)}
	case api.VirtualizationValidationFailed:
		desired = libcnd.Condition{Type: ProviderFailed, Status: libcnd.True, Reason: validationConditionReason(v), Category: api.CategoryWarn, Durable: true,
			Message: fmt.Sprintf("Virtualization validation %q failed: %d checks failed.", v.Name, v.Status.Summary.Failed)}
	default:
		return nil
	}

	return r.mutateProviderStatus(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: provider.Name}, func(current *api.Provider) bool {
		changed := false
		for _, conditionType := range []string{ProviderInProgress, ProviderSucceeded, ProviderFailed} {
			if conditionType != desired.Type {
				if current.Status.FindCondition(conditionType) != nil {
					changed = true
				}
				current.Status.DeleteCondition(conditionType)
			}
		}
		if existing := current.Status.FindCondition(desired.Type); existing == nil || !existing.Equal(desired) {
			current.Status.SetCondition(desired)
			changed = true
		}
		if !changed {
			return false
		}
		return true
	})
}

func (r Reconciler) finalize(ctx context.Context, v *api.VirtualizationValidation) error {
	providerNamespace := v.Spec.ProviderRef.Namespace
	if providerNamespace == "" {
		providerNamespace = v.Namespace
	}
	if v.Spec.ProviderRef.Name != "" {
		if err := r.mutateProviderStatus(ctx, types.NamespacedName{Namespace: providerNamespace, Name: v.Spec.ProviderRef.Name}, func(provider *api.Provider) bool {
			changed := false
			for _, conditionType := range []string{ProviderInProgress, ProviderSucceeded, ProviderFailed} {
				condition := provider.Status.FindCondition(conditionType)
				if condition != nil && condition.Reason == validationConditionReason(v) {
					provider.Status.DeleteCondition(conditionType)
					changed = true
				}
			}
			if !changed {
				return false
			}
			return true
		}); err != nil {
			if k8serr.IsNotFound(err) {
				// The Provider has already gone; validation cleanup can proceed.
			} else {
				return err
			}
		}
	}
	if controllerutil.RemoveFinalizer(v, ValidationFinalizer) {
		return r.Update(ctx, v)
	}
	return nil
}

func (r Reconciler) mutateProviderStatus(ctx context.Context, key types.NamespacedName, mutate func(*api.Provider) bool) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		provider := &api.Provider{}
		if err := r.Get(ctx, key, provider); err != nil {
			return err
		}
		if !mutate(provider) {
			return nil
		}
		if err := r.Status().Update(ctx, provider); err != nil {
			if !k8serr.IsConflict(err) {
				return err
			}
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func validationConditionReason(v *api.VirtualizationValidation) string {
	return "Validation" + string(v.UID)
}

// CTRFReport is the bounded portion of the CTRF document consumed by Forklift.
// The complete document remains in the hub-local ConfigMap for diagnostic use.
type CTRFReport struct {
	ReportFormat string `json:"reportFormat"`
	SpecVersion  string `json:"specVersion"`
	Results      struct {
		Summary struct {
			Tests   int `json:"tests"`
			Passed  int `json:"passed"`
			Failed  int `json:"failed"`
			Pending int `json:"pending"`
			Skipped int `json:"skipped"`
		} `json:"summary"`
		Tests []struct {
			Name     string `json:"name"`
			Status   string `json:"status"`
			Message  string `json:"message"`
			Duration int    `json:"duration"`
		} `json:"tests"`
	} `json:"results"`
}

func (r Reconciler) observeReport(v *api.VirtualizationValidation, cm *core.ConfigMap) (bool, error) {
	reportJSON := cm.Data[ResultConfigMapKey]
	if reportJSON == "" {
		return false, nil
	}
	var report CTRFReport
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
		return false, fmt.Errorf("result ConfigMap contains invalid JSON: %w", err)
	}
	if report.ReportFormat != "CTRF" || report.SpecVersion == "" {
		return false, fmt.Errorf("result ConfigMap does not contain a versioned CTRF report")
	}
	s := report.Results.Summary
	if s.Tests <= 0 || s.Passed < 0 || s.Failed < 0 || s.Pending < 0 || s.Skipped < 0 || s.Tests != s.Passed+s.Failed+s.Pending+s.Skipped || s.Tests != len(report.Results.Tests) {
		return false, fmt.Errorf("result ConfigMap contains an invalid CTRF summary")
	}
	v.Status.Summary = api.VirtualizationValidationSummary{Total: s.Tests, Passed: s.Passed, Failed: s.Failed, Pending: s.Pending, Skipped: s.Skipped}
	v.Status.CheckResults = make([]api.VirtualizationValidationCheckResult, 0, len(report.Results.Tests))
	for _, test := range report.Results.Tests {
		if len(v.Status.CheckResults) == 32 {
			break
		}
		if test.Name == "" || !validCTRFStatus(test.Status) {
			return false, fmt.Errorf("result ConfigMap contains an invalid CTRF test result")
		}
		v.Status.CheckResults = append(v.Status.CheckResults, api.VirtualizationValidationCheckResult{
			ID: test.Name, Status: test.Status, Message: truncate(test.Message, 4096), Duration: test.Duration,
		})
	}
	return s.Pending == 0, nil
}

func (r Reconciler) observeJob(v *api.VirtualizationValidation, job *batch.Job, reportComplete bool) {
	for _, c := range job.Status.Conditions {
		switch c.Type {
		case batch.JobComplete:
			if !reportComplete {
				r.failed(v, "IncompleteReport", "Validator Job completed without a final CTRF report.")
				return
			}
			v.Status.Phase = api.VirtualizationValidationSucceeded
			now := meta.Now()
			v.Status.CompletedAt = &now
			v.Status.SetCondition(libcnd.Condition{Type: api.ConditionSucceeded, Status: libcnd.True, Category: api.CategoryRequired, Message: "Validator Job completed with a final CTRF report."})
			return
		case batch.JobFailed:
			r.failed(v, "JobFailed", c.Message)
			return
		}
	}
	if job.Status.StartTime == nil {
		v.Status.Phase = api.VirtualizationValidationPending
		v.Status.SetCondition(libcnd.Condition{Type: api.ConditionPending, Status: libcnd.True, Category: api.CategoryAdvisory, Message: "Waiting for validator Job to start."})
		return
	}
	v.Status.Phase = api.VirtualizationValidationRunning
	if v.Status.StartedAt == nil {
		started := *job.Status.StartTime
		v.Status.StartedAt = &started
	}
	v.Status.SetCondition(libcnd.Condition{Type: api.ConditionRunning, Status: libcnd.True, Category: api.CategoryAdvisory, Message: "Validator Job is running."})
}

func (r Reconciler) failed(v *api.VirtualizationValidation, reason, message string) {
	v.Status.Phase = api.VirtualizationValidationFailed
	now := meta.Now()
	v.Status.CompletedAt = &now
	v.Status.SetCondition(libcnd.Condition{Type: api.ConditionFailed, Status: libcnd.True, Reason: reason, Category: api.CategoryWarn, Message: message})
}

// providerProfileChecks maps stable Forklift check IDs to explicitly reviewed,
// read-only VCV check paths.  Passing VCV's raw substring filters through the
// API would let callers select an empty or future mutating check set.
func providerProfileChecks(requested []string) ([]string, error) {
	if len(requested) == 0 {
		requested = []string{"platform"}
	}
	for _, check := range requested {
		if check != "platform" {
			return nil, fmt.Errorf("check %q is not supported by the Provider profile", check)
		}
	}
	return []string{
		"10-openshift.d/00-installation.d",
		"10-openshift.d/02-cluster-version.d",
		"10-openshift.d/03-cluster-operators.d",
		"10-openshift.d/10-nodes.d",
		"10-openshift.d/13-node-health.d",
		"50-openshift-virtualization.d/00-installation.d",
	}, nil
}

func validCTRFStatus(status string) bool {
	switch status {
	case "passed", "failed", "pending", "skipped":
		return true
	default:
		return false
	}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func inputHash(v *api.VirtualizationValidation, provider *api.Provider, secret *core.Secret, image string, checks []string) string {
	checks = append([]string(nil), checks...)
	sort.Strings(checks)
	input := strings.Join([]string{string(provider.UID), string(provider.Type()), provider.Spec.URL, secret.ResourceVersion, string(v.Spec.Profile), strings.Join(checks, ","), image, v.Spec.RunNonce}, "\x00")
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func resourceName(v *api.VirtualizationValidation, hash string) string {
	identity := string(v.UID)
	if len(identity) > 8 {
		identity = identity[:8]
	}
	if identity == "" {
		identity = v.Name
	}
	name := fmt.Sprintf("virt-validation-%s-%s", identity, hash[:12])
	return strings.TrimSuffix(name, "-")
}

func labelsFor(v *api.VirtualizationValidation, hash string) map[string]string {
	return map[string]string{ValidationLabel: string(v.UID), ValidationInputLabel: hash[:12]}
}

func resultConfigMap(v *api.VirtualizationValidation, name string) *core.ConfigMap {
	return &core.ConfigMap{ObjectMeta: meta.ObjectMeta{Name: name, Namespace: v.Namespace, Labels: labelsFor(v, v.Status.ObservedInputHash)}, Data: map[string]string{ResultConfigMapKey: ""}}
}

func validationCredentials(v *api.VirtualizationValidation, name string, providerSecret *core.Secret) *core.Secret {
	ca, _ := util.GetCACert(providerSecret)
	insecure := providerSecret.Data[api.Insecure]
	if len(insecure) == 0 {
		insecure = []byte("false")
	}
	return &core.Secret{
		ObjectMeta: meta.ObjectMeta{Name: name + "-credentials", Namespace: v.Namespace, Labels: labelsFor(v, v.Status.ObservedInputHash)},
		Data:       map[string][]byte{api.Token: providerSecret.Data[api.Token], "ca.crt": ca, api.Insecure: insecure},
	}
}

func validationServiceAccount(v *api.VirtualizationValidation, name string) *core.ServiceAccount {
	return &core.ServiceAccount{ObjectMeta: meta.ObjectMeta{Name: name, Namespace: v.Namespace, Labels: labelsFor(v, v.Status.ObservedInputHash)}}
}

func resultConfigMapRole(v *api.VirtualizationValidation, name string) *rbac.Role {
	return &rbac.Role{ObjectMeta: meta.ObjectMeta{Name: name, Namespace: v.Namespace, Labels: labelsFor(v, v.Status.ObservedInputHash)}, Rules: []rbac.PolicyRule{{
		APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{name}, Verbs: []string{"get", "patch", "update"},
	}}}
}

func resultConfigMapRoleBinding(v *api.VirtualizationValidation, name string) *rbac.RoleBinding {
	return &rbac.RoleBinding{ObjectMeta: meta.ObjectMeta{Name: name, Namespace: v.Namespace, Labels: labelsFor(v, v.Status.ObservedInputHash)},
		Subjects: []rbac.Subject{{Kind: rbac.ServiceAccountKind, Name: name, Namespace: v.Namespace}},
		RoleRef:  rbac.RoleRef{APIGroup: rbac.GroupName, Kind: "Role", Name: name}}
}

func validatorJob(v *api.VirtualizationValidation, provider *api.Provider, secret *core.Secret, image, name string, checks []string) *batch.Job {
	secretName := secret.Name
	return &batch.Job{
		ObjectMeta: meta.ObjectMeta{Name: name, Namespace: v.Namespace, Labels: labelsFor(v, v.Status.ObservedInputHash)},
		Spec: batch.JobSpec{
			BackoffLimit:          ptr.To[int32](0),
			ActiveDeadlineSeconds: ptr.To[int64](900),
			Template: core.PodTemplateSpec{Spec: core.PodSpec{
				RestartPolicy:      core.RestartPolicyNever,
				ServiceAccountName: name,
				Containers: []core.Container{{
					Name:  "validator",
					Image: image,
					Env: []core.EnvVar{
						{Name: "VIRT_VALIDATE_URL", Value: provider.Spec.URL},
						{Name: "VIRT_VALIDATE_CA_FILE", Value: "/var/run/forklift-validation/ca.crt"},
						{Name: "VIRT_VALIDATE_RESULT_CONFIGMAP", Value: name},
						{Name: "VIRT_VALIDATE_RESULT_CONFIGMAP_NAMESPACE", ValueFrom: &core.EnvVarSource{FieldRef: &core.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
						{Name: "VIRT_VALIDATE_CHECKS", Value: strings.Join(checks, ",")},
					},
					VolumeMounts: []core.VolumeMount{{Name: "credentials", MountPath: "/var/run/forklift-validation", ReadOnly: true}},
				}},
				Volumes: []core.Volume{{Name: "credentials", VolumeSource: core.VolumeSource{Secret: &core.SecretVolumeSource{SecretName: secretName}}}},
			}},
		},
	}
}
