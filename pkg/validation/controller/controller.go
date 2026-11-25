// Copyright (C) 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	validationv1alpha1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	validationmetrics "github.com/kubev2v/forklift/pkg/validation/metrics"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerpkg "sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	validationLabel         = "forklift.konveyor.io/validation"
	validationUIDLabel      = "forklift.konveyor.io/uid"
	runLabel                = "forklift.konveyor.io/run"
	tokenKey                = "token"
	caKey                   = "ca.crt"
	reportKey               = "ctrf.json"
	maxResults              = 128
	statusPassedDetailLimit = 32
	maxFailureExemplars     = 2
	maxFailureDetails       = 64
	maxStatusBytes          = 700 * 1024
	maxExecutionID          = 128
	maxExecutionName        = 256
	maxStepName             = 256
	maxMessage              = 512
	maxParameters           = 16
	maxParameterKey         = 128
	maxParameterValue       = 256
	maxSteps                = 16
	maxSuiteTimeout         = 86400
	maxQueueTimeout         = 86400
	maxExecutionTimeout     = 3600
	maxConcurrentRuns       = 64
	defaultSuiteTimeout     = 900
	defaultExecutionTimeout = 180
	jobTTL                  = 7 * 24 * 60 * 60
	jobTerminationGrace     = 120
	finalizer               = "forklift.konveyor.io/validation-finalizer"
	leaseDuration           = 2 * time.Minute
	leaseRenewAfter         = 30 * time.Second
	reportGrace             = 30 * time.Second
	basicAllNodesProfile    = "basic-all-nodes-v1"
)

type VirtualizationValidationReconciler struct {
	client.Client
	APIReader      client.Reader
	Scheme         *runtime.Scheme
	ValidatorImage string
	Recorder       record.EventRecorder
	Metrics        *validationmetrics.ValidationMetrics
}

//+kubebuilder:rbac:groups=forklift.konveyor.io,resources=virtualizationvalidations;virtualizationvalidations/status;virtualizationvalidations/finalizers,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;delete;update;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

type ctrfTest struct {
	TestID      string         `json:"testId"`
	ExecutionID string         `json:"executionId"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Duration    int64          `json:"duration"`
	Start       int64          `json:"start"`
	Stop        int64          `json:"stop"`
	Message     string         `json:"message"`
	Parameters  map[string]any `json:"parameters"`
	Retries     int            `json:"retries"`
	Flaky       bool           `json:"flaky"`
	Extra       map[string]any `json:"extra"`
	Steps       []ctrfStep     `json:"steps"`
}

type ctrfStep struct {
	Name   string         `json:"name"`
	Status string         `json:"status"`
	Extra  map[string]any `json:"extra"`
}

type ctrfReport struct {
	ReportFormat string `json:"reportFormat"`
	SpecVersion  string `json:"specVersion"`
	ReportID     string `json:"reportId"`
	RunID        string `json:"runId"`
	GeneratedBy  string `json:"generatedBy"`
	Results      struct {
		Summary struct {
			Tests   int `json:"tests"`
			Passed  int `json:"passed"`
			Failed  int `json:"failed"`
			Pending int `json:"pending"`
			Skipped int `json:"skipped"`
			Other   int `json:"other"`
		} `json:"summary"`
		Tests []ctrfTest                 `json:"tests"`
		Extra map[string]json.RawMessage `json:"extra"`
	} `json:"results"`
}

type ctrfReduction struct {
	Level                       int32 `json:"level"`
	TotalExecutions             int32 `json:"totalExecutions"`
	PendingCollapsed            int32 `json:"pendingCollapsed"`
	PassedCollapsed             int32 `json:"passedCollapsed"`
	RequiredFailuresOmitted     int32 `json:"requiredFailuresOmitted"`
	AdvisoryFailuresOmitted     int32 `json:"advisoryFailuresOmitted"`
	RequiredInconclusiveOmitted int32 `json:"requiredInconclusiveOmitted"`
	AdvisoryInconclusiveOmitted int32 `json:"advisoryInconclusiveOmitted"`
}

func (r *VirtualizationValidationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	validation := &validationv1alpha1.VirtualizationValidation{}
	if err := r.Get(ctx, req.NamespacedName, validation); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !validation.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, validation)
	}
	if !contains(validation.Finalizers, finalizer) {
		validation.Finalizers = append(validation.Finalizers, finalizer)
		return ctrl.Result{}, r.Update(ctx, validation)
	}
	if validation.Spec.Run.Cancel {
		return ctrl.Result{}, r.cancel(ctx, validation)
	}
	if err := validSpec(validation); err != nil {
		return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseError, validationv1alpha1.ValidationVerdictInconclusive, "InvalidSpec", err.Error())
	}
	profile, err := trustedProfile(validation.Spec.Run.Profile, validation.Spec.Workload)
	if err != nil {
		return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseError, validationv1alpha1.ValidationVerdictInconclusive, "InvalidChecks", err.Error())
	}
	secret := &corev1.Secret{}
	if err := r.apiReader().Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: validation.Spec.Target.CredentialsSecret.Name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseError, validationv1alpha1.ValidationVerdictInconclusive, "CredentialsNotFound", err.Error())
		}
		return ctrl.Result{}, err
	}
	if _, found := secret.Data[tokenKeyFor(validation)]; !found {
		return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseError, validationv1alpha1.ValidationVerdictInconclusive, "InvalidCredentials", fmt.Sprintf("secret %q has no %q key", secret.Name, tokenKeyFor(validation)))
	}
	if !validation.Spec.Target.InsecureSkipTLS && caKeyFor(validation) != "" {
		if _, found := secret.Data[caKeyFor(validation)]; !found {
			return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseError, validationv1alpha1.ValidationVerdictInconclusive, "InvalidCredentials", fmt.Sprintf("secret %q has no %q key", secret.Name, caKeyFor(validation)))
		}
	}

	hash := inputHash(validation, profile)
	if validation.Status.ObservedInputHash != hash {
		if err := r.releaseLease(ctx, validation); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.deleteSupersededResources(ctx, validation, hash); err != nil {
			return ctrl.Result{}, err
		}
		validation.Status = validationv1alpha1.VirtualizationValidationStatus{
			ObservedGeneration: validation.Generation,
			ObservedInputHash:  hash,
			Image:              r.ValidatorImage,
		}
		// Do not rely on a status-only update being delivered as another primary
		// resource event. Requeue explicitly so the next pass acquires the lease
		// and creates the run resources.
		if err := r.Status().Update(ctx, validation); err != nil {
			return ctrl.Result{}, err
		}
		ctrl.LoggerFrom(ctx).Info("Initialized validation run", "validation", client.ObjectKeyFromObject(validation), "profile", validation.Spec.Run.Profile, "generation", validation.Generation)
		return ctrl.Result{Requeue: true}, nil
	}
	if terminal(validation.Status.Phase) {
		return ctrl.Result{}, nil
	}
	leaseHeld, err := r.acquireLease(ctx, validation)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !leaseHeld {
		if validation.Status.QueuedAt == nil {
			now := metav1.Now()
			validation.Status.QueuedAt = &now
		}
		if time.Since(validation.Status.QueuedAt.Time) >= effectiveQueueTimeout(validation.Spec.Run.QueueTimeoutSeconds, validation.Spec.Run.SuiteTimeoutSeconds) {
			return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseError, validationv1alpha1.ValidationVerdictInconclusive, "QueueTimeout", "validation exceeded its target lease queue timeout")
		}
		return ctrl.Result{RequeueAfter: leaseRenewAfter}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhasePending, validationv1alpha1.ValidationVerdictUnknown, "WaitingForLease", "another validation is already running for this target")
	}
	validation.Status.QueuedAt = nil

	jobName := runName(validation, "vv")
	resultName := runName(validation, "vr")
	if err := r.ensureResources(ctx, validation, jobName, resultName); err != nil {
		return ctrl.Result{}, err
	}
	result := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: resultName}, result); err != nil {
		return ctrl.Result{}, err
	}
	if changed, err := r.observeReport(ctx, validation, result); err != nil || changed {
		return ctrl.Result{}, err
	}
	job := &batchv1.Job{}
	key := types.NamespacedName{Namespace: validation.Namespace, Name: jobName}
	if err := r.Get(ctx, key, job); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if validation.Status.StartedAt != nil {
			if err := r.releaseLease(ctx, validation); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseError, validationv1alpha1.ValidationVerdictInconclusive, "JobMissing", "validator Job disappeared before producing a terminal result")
		}
		job = r.job(validation, jobName, resultName, profile)
		if err := ctrl.SetControllerReference(validation, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, job); err != nil {
			return ctrl.Result{}, err
		}
		now := metav1.Now()
		validation.Status.StartedAt = &now
		return ctrl.Result{RequeueAfter: leaseRenewAfter}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseRunning, validationv1alpha1.ValidationVerdictUnknown, "JobCreated", "validator job was created")
	}

	if job.Status.Succeeded > 0 {
		if !validation.Status.ReportComplete {
			return r.awaitFinalReport(ctx, validation, job)
		}
		if err := r.releaseLease(ctx, validation); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseCompleted, validation.Status.Verdict, "Completed", "validator job completed with a final CTRF report")
	}
	if job.Status.Failed > 0 {
		if !validation.Status.ReportComplete {
			return r.awaitFinalReport(ctx, validation, job)
		}
		if err := r.releaseLease(ctx, validation); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseCompleted, validation.Status.Verdict, "ChecksCompleted", "validator job completed with unsuccessful checks")
	}
	return ctrl.Result{RequeueAfter: leaseRenewAfter}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseRunning, validationv1alpha1.ValidationVerdictUnknown, "JobRunning", "validator job is running")
}

func (r *VirtualizationValidationReconciler) job(validation *validationv1alpha1.VirtualizationValidation, name, resultName, profile string) *batchv1.Job {
	suiteTimeout := effectiveSuiteTimeout(validation.Spec.Run.SuiteTimeoutSeconds)
	ttl := int32(jobTTL)
	terminationGrace := int64(jobTerminationGrace)
	env := []corev1.EnvVar{
		{Name: "VIRT_VALIDATE_URL", Value: validation.Spec.Target.URL},
		{Name: "VIRT_VALIDATE_TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: validation.Spec.Target.CredentialsSecret.Name}, Key: tokenKeyFor(validation)}}},
		{Name: "VIRT_VALIDATE_RESULT_CONFIGMAP", Value: resultName},
		{Name: "VIRT_VALIDATE_RESULT_CONFIGMAP_NAMESPACE", Value: validation.Namespace},
		{Name: "VIRT_VALIDATE_VALIDATION_UID", Value: string(validation.UID)},
	}
	env = append(env,
		corev1.EnvVar{Name: "VIRT_VALIDATE_PROFILE", Value: profile},
		corev1.EnvVar{Name: "VIRT_VALIDATE_NAMESPACE", Value: validation.Spec.Workload.Namespace},
		corev1.EnvVar{Name: "VIRT_VALIDATE_DATA_SOURCE", Value: dataSourceName(validation.Spec.Workload.DataSource)},
		corev1.EnvVar{Name: "VIRT_VALIDATE_EXECUTION_TIMEOUT_SECONDS", Value: strconv.FormatInt(effectiveExecutionTimeout(validation.Spec.Run.ExecutionTimeoutSeconds), 10)},
		corev1.EnvVar{Name: "VIRT_VALIDATE_CLEANUP_POLICY", Value: effectiveCleanupPolicy(validation.Spec.Run.CleanupPolicy)},
	)
	tools := toolDownloads(validation)
	if tools.ClusterDomain != "" {
		env = append(env, corev1.EnvVar{Name: "VIRT_VALIDATE_CLUSTER_DOMAIN", Value: tools.ClusterDomain})
	}
	if tools.OCURL != "" {
		env = append(env, corev1.EnvVar{Name: "VIRT_VALIDATE_OC_URL", Value: tools.OCURL})
	}
	if tools.VirtctlURL != "" {
		env = append(env, corev1.EnvVar{Name: "VIRT_VALIDATE_VIRTCTL_URL", Value: tools.VirtctlURL})
	}
	if validation.Spec.Run.MaxConcurrentExecutions > 0 {
		env = append(env, corev1.EnvVar{Name: "VIRT_VALIDATE_CONCURRENCY", Value: strconv.FormatInt(int64(validation.Spec.Run.MaxConcurrentExecutions), 10)})
	}
	if validation.Spec.Target.InsecureSkipTLS {
		env = append(env, corev1.EnvVar{Name: "VIRT_VALIDATE_INSECURE_SKIP_TLS", Value: "true"})
	} else if caKeyFor(validation) != "" {
		env = append(env, corev1.EnvVar{Name: "VIRT_VALIDATE_CA_FILE", Value: "/var/run/virt-validation/ca.crt"})
	}
	env = append(env, proxyEnvironment()...)
	readOnlyRootFilesystem := true
	allowPrivilegeEscalation := false
	runAsNonRoot := true
	mounts := []corev1.VolumeMount{{Name: "tools", MountPath: "/usr/local/bin"}, {Name: "tmp", MountPath: "/tmp"}}
	volumes := []corev1.Volume{{Name: "tools", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}, {Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	if !validation.Spec.Target.InsecureSkipTLS && caKeyFor(validation) != "" {
		mounts = append(mounts, corev1.VolumeMount{Name: "credentials", MountPath: "/var/run/virt-validation", ReadOnly: true})
		volumes = append(volumes, corev1.Volume{Name: "credentials", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: validation.Spec.Target.CredentialsSecret.Name, Items: []corev1.KeyToPath{{Key: caKeyFor(validation), Path: "ca.crt"}}}}})
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: validation.Namespace, Labels: labelsFor(validation)},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr[int32](0),
			ActiveDeadlineSeconds:   &suiteTimeout,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labelsFor(validation)}, Spec: corev1.PodSpec{
				RestartPolicy:                 corev1.RestartPolicyNever,
				TerminationGracePeriodSeconds: &terminationGrace,
				ServiceAccountName:            name,
				SecurityContext:               &corev1.PodSecurityContext{RunAsNonRoot: &runAsNonRoot, SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
				Containers: []corev1.Container{{
					Name: "validator", Image: r.ValidatorImage, Command: []string{"/opt/app-root/src/bin/run-validation"}, Env: env,
					VolumeMounts:    mounts,
					Resources:       corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("128Mi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("512Mi")}},
					SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: &allowPrivilegeEscalation, ReadOnlyRootFilesystem: &readOnlyRootFilesystem, RunAsNonRoot: &runAsNonRoot, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}},
				}},
				Volumes: volumes,
			}},
		},
	}
}

func (r *VirtualizationValidationReconciler) ensureResources(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation, name, resultName string) error {
	resources := []client.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: resultName, Namespace: validation.Namespace, Labels: labelsFor(validation), Annotations: map[string]string{"forklift.konveyor.io/report-format": "CTRF", "forklift.konveyor.io/report-contract": "v1"}}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: validation.Namespace, Labels: labelsFor(validation)}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: validation.Namespace, Labels: labelsFor(validation)}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"configmaps"}, ResourceNames: []string{resultName}, Verbs: []string{"get", "patch", "update"}}}},
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: validation.Namespace, Labels: labelsFor(validation)}, Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: name, Namespace: validation.Namespace}}, RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}},
		r.validationNetworkPolicy(validation),
	}
	for _, resource := range resources {
		if err := r.apiReader().Get(ctx, client.ObjectKeyFromObject(resource), resource); err == nil {
			if !metav1.IsControlledBy(resource, validation) || resource.GetLabels()[runLabel] != runHashLabel(validation.Status.ObservedInputHash) {
				return fmt.Errorf("refusing to reuse %s %q: it is not owned by this validation run", resource.GetObjectKind().GroupVersionKind().Kind, resource.GetName())
			}
			continue
		} else if !apierrors.IsNotFound(err) {
			return err
		}
		if err := ctrl.SetControllerReference(validation, resource, r.Scheme); err != nil {
			return err
		}
		if err := r.Create(ctx, resource); err != nil {
			return err
		}
	}
	return nil
}

// validationNetworkPolicy confines one validation run's credential-bearing Job
// Pods to DNS and HTTPS. DNS permits both the Service port (53) and the
// OpenShift DNS backend port (5353): OVN-Kubernetes evaluates egress policy
// after Service load balancing, where the latter can be visible. NetworkPolicy
// does not support FQDN destinations, so the HTTPS rule deliberately covers
// both the hub API and the configured remote API/tool-download endpoint; the
// target URL remains constrained by CRD validation to HTTPS.
func (r *VirtualizationValidationReconciler) validationNetworkPolicy(validation *validationv1alpha1.VirtualizationValidation) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: runName(validation, "vnp"), Namespace: validation.Namespace, Labels: labelsFor(validation)},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: labelsFor(validation)},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{Ports: dnsPolicyPorts()},
				{Ports: targetAPIPolicyPorts(validation.Spec.Target.URL)},
			},
		},
	}
}

func dnsPolicyPorts() []networkingv1.NetworkPolicyPort {
	return []networkingv1.NetworkPolicyPort{
		networkPolicyPort(corev1.ProtocolUDP, 53),
		networkPolicyPort(corev1.ProtocolTCP, 53),
		networkPolicyPort(corev1.ProtocolUDP, 5353),
		networkPolicyPort(corev1.ProtocolTCP, 5353),
	}
}

func targetAPIPolicyPorts(targetURL string) []networkingv1.NetworkPolicyPort {
	ports := []int{443, 6443}
	if targetPort, err := targetURLPort(targetURL); err == nil && targetPort != 443 && targetPort != 6443 {
		ports = append(ports, targetPort)
	}
	policyPorts := make([]networkingv1.NetworkPolicyPort, 0, len(ports))
	for _, port := range ports {
		policyPorts = append(policyPorts, networkPolicyPort(corev1.ProtocolTCP, port))
	}
	return policyPorts
}

func networkPolicyPort(protocol corev1.Protocol, port int) networkingv1.NetworkPolicyPort {
	value := intstr.FromInt(port)
	return networkingv1.NetworkPolicyPort{Protocol: &protocol, Port: &value}
}

func (r *VirtualizationValidationReconciler) observeReport(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation, result *corev1.ConfigMap) (bool, error) {
	if result.ResourceVersion == validation.Status.ObservedReportResourceVersion || result.Data[reportKey] == "" {
		return false, nil
	}
	var report ctrfReport
	if err := json.Unmarshal([]byte(result.Data[reportKey]), &report); err != nil || !validReport(report) {
		// The runner updates the ConfigMap while it is executing. Do not turn a
		// transient, malformed snapshot into a terminal validation error: retain
		// the previous good snapshot and try again on the next reconciliation.
		return false, nil
	}
	executions := make([]validationv1alpha1.VirtualizationValidationExecution, 0, len(report.Results.Tests))
	for _, test := range report.Results.Tests {
		if test.Extra["forklift.konveyor.io/synthetic"] == "true" {
			continue
		}
		var parameters map[string]string
		parametersTruncated := len(test.Parameters) > maxParameters
		keys := make([]string, 0, len(test.Parameters))
		for key := range test.Parameters {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if len(parameters) == maxParameters {
				parametersTruncated = true
				break
			}
			boundedKey := truncate(key, maxParameterKey)
			if len(boundedKey) != len(key) {
				parametersTruncated = true
			}
			if _, exists := parameters[boundedKey]; exists {
				parametersTruncated = true
				continue
			}
			value := test.Parameters[key]
			text, ok := value.(string)
			if !ok {
				parametersTruncated = true
				continue
			}
			if len(text) > maxParameterValue {
				parametersTruncated = true
			}
			if parameters == nil {
				parameters = make(map[string]string, min(len(test.Parameters), maxParameters))
			}
			parameters[boundedKey] = truncate(text, maxParameterValue)
		}
		steps := make([]validationv1alpha1.ValidationStep, 0, min(len(test.Steps), maxSteps))
		for _, step := range test.Steps[:min(len(test.Steps), maxSteps)] {
			var extra map[string]string
			extraTruncated := len(step.Extra) > maxParameters
			keys := make([]string, 0, len(step.Extra))
			for key := range step.Extra {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if len(extra) == maxParameters {
					extraTruncated = true
					break
				}
				boundedKey := truncate(key, maxParameterKey)
				if len(boundedKey) != len(key) {
					extraTruncated = true
				}
				if _, exists := extra[boundedKey]; exists {
					extraTruncated = true
					continue
				}
				value, ok := step.Extra[key].(string)
				if !ok {
					extraTruncated = true
					continue
				}
				if len(value) > maxParameterValue {
					extraTruncated = true
				}
				if extra == nil {
					extra = make(map[string]string, min(len(step.Extra), maxParameters))
				}
				extra[boundedKey] = truncate(value, maxParameterValue)
			}
			steps = append(steps, validationv1alpha1.ValidationStep{Name: truncate(step.Name, maxStepName), Outcome: ctrfOutcome(step.Status, step.Extra), Reason: ctrfStepReason(step), Extra: extra, ExtraTruncated: extraTruncated})
		}
		var subject *validationv1alpha1.ValidationSubjectReference
		if strings.HasPrefix(test.ExecutionID, "node/") {
			subject = &validationv1alpha1.ValidationSubjectReference{Kind: "Node", Name: strings.TrimPrefix(test.ExecutionID, "node/")}
		}
		outcome := ctrfTestOutcome(test, steps)
		checkID := stableCheckID(test.TestID)
		severity := severityForCheck(checkID)
		reason := executionReason(outcome, steps)
		if reportedReason, found := test.Extra["forklift.konveyor.io/reason"].(string); found {
			reason = reportedReason
		} else if severity == validationv1alpha1.ValidationSeverityAdvisory && outcome == validationv1alpha1.ValidationOutcomePassed && test.Parameters["forklift.konveyor.io/advisory-warning"] == "true" {
			reason = "AdvisoryWarning"
		}
		executions = append(executions, validationv1alpha1.VirtualizationValidationExecution{CheckID: checkID, ExecutionID: truncate(publicExecutionID(test.ExecutionID), maxExecutionID), Name: truncate(publicExecutionName(test.Name, test.TestID, checkID), maxExecutionName), Outcome: outcome, Severity: severity, Reason: reason, Subject: subject, DurationMilliseconds: test.Duration, StartedAt: ctrfTime(test.Start), CompletedAt: ctrfTime(test.Stop), Message: truncate(test.Message, maxMessage), Parameters: parameters, ParametersTruncated: parametersTruncated, RetryCount: int32(test.Retries), Flaky: test.Flaky, Steps: steps, StepsTruncated: len(test.Steps) > maxSteps})
	}
	old := validation.Status.DeepCopy()
	validation.Status.ObservedReportResourceVersion = result.ResourceVersion
	artifactReduction, _ := reportReduction(report)
	validation.Status.Report = &validationv1alpha1.ValidationReportStatus{SpecVersion: report.SpecVersion, Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(result.Data[reportKey]))), Reduction: artifactReduction}
	validation.Status.Artifact = &validationv1alpha1.ValidationArtifactReference{APIVersion: "v1", Name: result.Name, Namespace: result.Namespace, Kind: "ConfigMap", Key: reportKey, Truncated: artifactReduction.Level > 0}
	validation.Status.ReportComplete = report.Results.Summary.Pending == 0
	validation.Status.Summary = validationv1alpha1.VirtualizationValidationSummary{Total: int32(report.Results.Summary.Tests), Passed: int32(report.Results.Summary.Passed), Failed: int32(report.Results.Summary.Failed), Pending: int32(report.Results.Summary.Pending), Skipped: int32(report.Results.Summary.Skipped), Other: int32(report.Results.Summary.Other)}
	allExecutions := append([]validationv1alpha1.VirtualizationValidationExecution(nil), executions...)
	validation.Status.Executions = executions
	validation.Status.Reduction = artifactReduction
	reduceStatusProjection(&validation.Status)
	validation.Status.Verdict = aggregateVerdict(validation.Status.Summary, validation.Status.Executions, validation.Status.Reduction, validation.Status.ReportComplete)
	if reflect.DeepEqual(*old, validation.Status) {
		return false, nil
	}
	if err := r.Status().Update(ctx, validation); err != nil {
		return false, err
	}
	if r.Metrics != nil && validation.Status.ReportComplete {
		r.Metrics.ObserveFinalReport(validation.Spec.Run.Profile, allExecutions, artifactReduction)
	}
	if validation.Status.ReportComplete {
		ctrl.LoggerFrom(ctx).Info("Observed final CTRF report", "validation", client.ObjectKeyFromObject(validation), "profile", validation.Spec.Run.Profile, "verdict", validation.Status.Verdict, "total", validation.Status.Summary.Total, "passed", validation.Status.Summary.Passed, "failed", validation.Status.Summary.Failed, "skipped", validation.Status.Summary.Skipped, "other", validation.Status.Summary.Other, "reductionLevel", artifactReduction.Level)
	}
	return true, nil
}

func (r *VirtualizationValidationReconciler) setStatus(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation, phase validationv1alpha1.ValidationPhase, verdict validationv1alpha1.ValidationVerdict, reason, message string) error {
	old := validation.Status.DeepCopy()
	validation.Status.Phase = phase
	validation.Status.Verdict = verdict
	validation.Status.ObservedGeneration = validation.Generation
	validation.Status.JobName = runName(validation, "vv")
	completed := metav1.ConditionFalse
	progressing := metav1.ConditionTrue
	valid := metav1.ConditionUnknown
	if phase == validationv1alpha1.ValidationPhaseCompleted || phase == validationv1alpha1.ValidationPhaseCancelled || phase == validationv1alpha1.ValidationPhaseError {
		completed = metav1.ConditionTrue
		progressing = metav1.ConditionFalse
		switch verdict {
		case validationv1alpha1.ValidationVerdictValid, validationv1alpha1.ValidationVerdictValidWithWarnings:
			valid = metav1.ConditionTrue
		case validationv1alpha1.ValidationVerdictInvalid:
			valid = metav1.ConditionFalse
		}
		if validation.Status.CompletedAt == nil {
			now := metav1.Now()
			validation.Status.CompletedAt = &now
		}
	} else {
		validation.Status.CompletedAt = nil
	}
	now := metav1.Now()
	meta.SetStatusCondition(&validation.Status.Conditions, metav1.Condition{Type: "Completed", Status: completed, Reason: reason, Message: message, ObservedGeneration: validation.Generation, LastTransitionTime: now})
	meta.SetStatusCondition(&validation.Status.Conditions, metav1.Condition{Type: "Progressing", Status: progressing, Reason: reason, Message: message, ObservedGeneration: validation.Generation, LastTransitionTime: now})
	meta.SetStatusCondition(&validation.Status.Conditions, metav1.Condition{Type: "Valid", Status: valid, Reason: reason, Message: message, ObservedGeneration: validation.Generation, LastTransitionTime: now})
	if reflect.DeepEqual(*old, validation.Status) {
		return nil
	}
	if err := r.Status().Update(ctx, validation); err != nil {
		return err
	}
	if r.Metrics != nil {
		r.Metrics.ObserveTerminal(validation, old.Phase)
		if old.Phase != validationv1alpha1.ValidationPhaseError && phase == validationv1alpha1.ValidationPhaseError {
			r.Metrics.ObserveTerminalError(validation.Spec.Run.Profile, reason)
			if reason == "QueueTimeout" {
				r.Metrics.ObserveQueueTimeout(validation.Spec.Run.Profile)
			}
		}
		if old.QueuedAt != nil {
			queueOutcome := ""
			switch {
			case old.Phase == validationv1alpha1.ValidationPhasePending && phase == validationv1alpha1.ValidationPhaseRunning && validation.Status.QueuedAt == nil:
				queueOutcome = "acquired"
			case reason == "QueueTimeout":
				queueOutcome = "timed_out"
			case reason == "Cancelled" && validation.Status.StartedAt == nil:
				queueOutcome = "cancelled"
			}
			if queueOutcome != "" {
				r.Metrics.ObserveQueue(validation.Spec.Run.Profile, queueOutcome, now.Sub(old.QueuedAt.Time))
			}
		}
	}
	if old.Phase != phase || conditionReason(old.Conditions, "Progressing") != reason {
		event := "Validation status updated"
		switch {
		case reason == "JobCreated":
			event = "Validator Job created"
		case reason == "WaitingForLease":
			event = "Validation queued for target lease"
		case terminal(phase):
			event = "Validation finished"
		}
		values := []any{"validation", client.ObjectKeyFromObject(validation), "profile", validation.Spec.Run.Profile, "job", validation.Status.JobName, "phase", phase, "verdict", verdict, "reason", reason}
		if terminal(phase) && validation.Status.StartedAt != nil && validation.Status.CompletedAt != nil {
			values = append(values, "durationSeconds", validation.Status.CompletedAt.Sub(validation.Status.StartedAt.Time).Seconds())
		}
		ctrl.LoggerFrom(ctx).Info(event, values...)
	}
	if r.Recorder != nil && (old.Phase != phase || conditionReason(old.Conditions, "Progressing") != reason) {
		r.Recorder.Event(validation, corev1.EventTypeNormal, reason, message)
	}
	return nil
}

func (r *VirtualizationValidationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&validationv1alpha1.VirtualizationValidation{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		WithOptions(controllerpkg.Options{MaxConcurrentReconciles: 4}).
		Complete(r)
}

func (r *VirtualizationValidationReconciler) apiReader() client.Reader {
	if r.APIReader != nil {
		return r.APIReader
	}
	return r.Client
}

func runName(validation *validationv1alpha1.VirtualizationValidation, prefix string) string {
	name := strings.ToLower(validation.Name)
	if len(name) > 36 {
		name = name[:36]
	}
	nameHash := sha256.Sum256([]byte(validation.Name))
	hash := validation.Status.ObservedInputHash
	if len(hash) > 12 {
		hash = hash[:12]
	}
	if hash == "" {
		hash = "pending"
	}
	return fmt.Sprintf("%s-%s-%x-%s", prefix, name, nameHash[:4], hash)
}

func trustedProfile(profile string, workload validationv1alpha1.ValidationWorkload) (string, error) {
	if profile != "basic-v1" && profile != basicAllNodesProfile {
		return "", fmt.Errorf("profile %q is not supported", profile)
	}
	if workload.Namespace == "" || workload.DataSource.Namespace == "" || workload.DataSource.Name == "" {
		return "", fmt.Errorf("the basic-v1 profile requires workload.namespace and workload.dataSource")
	}
	if dataSourceAPIGroup(workload.DataSource) != "cdi.kubevirt.io" || dataSourceKind(workload.DataSource) != "DataSource" {
		return "", fmt.Errorf("the basic-v1 profile requires a cdi.kubevirt.io DataSource")
	}
	return profile, nil
}

func inputHash(validation *validationv1alpha1.VirtualizationValidation, profile string) string {
	tools := toolDownloads(validation)
	input := strings.Join([]string{validation.Spec.Run.ID, normalizedTargetURL(validation.Spec.Target.URL), validation.Spec.Target.CredentialsSecret.Name, tokenKeyFor(validation), caKeyFor(validation), fmt.Sprint(validation.Spec.Target.InsecureSkipTLS), tools.ClusterDomain, tools.OCURL, tools.VirtctlURL, validation.Spec.Workload.Namespace, dataSourceAPIGroup(validation.Spec.Workload.DataSource), dataSourceKind(validation.Spec.Workload.DataSource), dataSourceName(validation.Spec.Workload.DataSource), validation.Spec.Run.Profile, profile, fmt.Sprint(effectiveSuiteTimeout(validation.Spec.Run.SuiteTimeoutSeconds)), fmt.Sprint(effectiveExecutionTimeout(validation.Spec.Run.ExecutionTimeoutSeconds)), effectiveCleanupPolicy(validation.Spec.Run.CleanupPolicy), fmt.Sprint(validation.Spec.Run.MaxConcurrentExecutions), fmt.Sprint(effectiveQueueTimeout(validation.Spec.Run.QueueTimeoutSeconds, validation.Spec.Run.SuiteTimeoutSeconds))}, "\x00")
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum[:])
}

func conditionReason(conditions []metav1.Condition, conditionType string) string {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Reason
		}
	}
	return ""
}

func normalizedTargetURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimRight(strings.ToLower(rawURL), "/")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}

func leaseName(validation *validationv1alpha1.VirtualizationValidation) string {
	input := validation.Namespace + "\x00" + normalizedTargetURL(validation.Spec.Target.URL)
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("vv-lease-%x", sum[:6])
}

func labelsFor(validation *validationv1alpha1.VirtualizationValidation) map[string]string {
	uid := string(validation.UID)
	if uid == "" {
		uid = validation.Namespace + "-" + validation.Name
	}
	return map[string]string{validationLabel: validation.Name, validationUIDLabel: uid, runLabel: runHashLabel(validation.Status.ObservedInputHash)}
}

func runHashLabel(hash string) string {
	const maxLabelValueLength = 63
	if len(hash) <= maxLabelValueLength {
		return hash
	}
	return hash[:maxLabelValueLength]
}

func (r *VirtualizationValidationReconciler) acquireLease(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation) (bool, error) {
	key := types.NamespacedName{Namespace: validation.Namespace, Name: leaseName(validation)}
	holder := string(validation.UID)
	if holder == "" {
		holder = validation.Namespace + "/" + validation.Name
	}
	lease := &coordinationv1.Lease{}
	now := metav1.MicroTime{Time: time.Now()}
	if err := r.Get(ctx, key, lease); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		lease = &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}, Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder, AcquireTime: &now, RenewTime: &now, LeaseDurationSeconds: ptr(int32(leaseDuration / time.Second))}}
		return true, r.Create(ctx, lease)
	}
	if lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity != holder && !leaseExpired(lease, now) {
		return false, nil
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != holder {
		lease.Spec.HolderIdentity = &holder
		lease.Spec.AcquireTime = &now
		transitions := int32(0)
		if lease.Spec.LeaseTransitions != nil {
			transitions = *lease.Spec.LeaseTransitions + 1
		}
		lease.Spec.LeaseTransitions = &transitions
	}
	lease.Spec.RenewTime = &now
	lease.Spec.LeaseDurationSeconds = ptr(int32(leaseDuration / time.Second))
	return true, r.Update(ctx, lease)
}

func (r *VirtualizationValidationReconciler) releaseLease(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation) error {
	holder := string(validation.UID)
	if holder == "" {
		holder = validation.Namespace + "/" + validation.Name
	}
	lease := &coordinationv1.Lease{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: leaseName(validation)}, lease); err != nil {
		return client.IgnoreNotFound(err)
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != holder {
		return nil
	}
	lease.Spec.HolderIdentity = nil
	lease.Spec.RenewTime = nil
	return r.Update(ctx, lease)
}

func leaseExpired(lease *coordinationv1.Lease, now metav1.MicroTime) bool {
	if lease.Spec.HolderIdentity == nil || lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return true
	}
	return now.After(lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second))
}

func (r *VirtualizationValidationReconciler) awaitFinalReport(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation, job *batchv1.Job) (ctrl.Result, error) {
	if job.Status.CompletionTime != nil && time.Since(job.Status.CompletionTime.Time) >= reportGrace {
		if err := r.releaseLease(ctx, validation); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseError, validationv1alpha1.ValidationVerdictInconclusive, "IncompleteReport", "validator Job completed without a final CTRF report")
	}
	return ctrl.Result{RequeueAfter: 5 * time.Second}, r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseRunning, validationv1alpha1.ValidationVerdictUnknown, "AwaitingFinalReport", "waiting for the final CTRF report")
}

func (r *VirtualizationValidationReconciler) deleteSupersededResources(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation, hash string) error {
	selector := map[string]string{validationUIDLabel: string(validation.UID)}
	if validation.UID == "" {
		selector = map[string]string{validationLabel: validation.Name}
	}
	for _, objects := range []client.ObjectList{&batchv1.JobList{}, &corev1.ConfigMapList{}, &corev1.ServiceAccountList{}, &rbacv1.RoleList{}, &rbacv1.RoleBindingList{}, &networkingv1.NetworkPolicyList{}} {
		if err := r.apiReader().List(ctx, objects, client.InNamespace(validation.Namespace), client.MatchingLabels(selector)); err != nil {
			return err
		}
		items, err := meta.ExtractList(objects)
		if err != nil {
			return err
		}
		for _, item := range items {
			resource := item.(client.Object)
			if resource.GetLabels()[runLabel] != runHashLabel(hash) {
				if err := r.Delete(ctx, resource); err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

func (r *VirtualizationValidationReconciler) finalize(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation) error {
	if err := r.releaseLease(ctx, validation); err != nil {
		return err
	}
	if err := r.deleteLeaseIfUnused(ctx, validation); err != nil {
		return err
	}
	validation.Finalizers = remove(validation.Finalizers, finalizer)
	return r.Update(ctx, validation)
}

func (r *VirtualizationValidationReconciler) deleteLeaseIfUnused(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation) error {
	validations := &validationv1alpha1.VirtualizationValidationList{}
	if err := r.List(ctx, validations, client.InNamespace(validation.Namespace)); err != nil {
		return err
	}
	for index := range validations.Items {
		other := &validations.Items[index]
		if other.UID != validation.UID && other.DeletionTimestamp.IsZero() && leaseName(other) == leaseName(validation) {
			return nil
		}
	}
	lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: leaseName(validation), Namespace: validation.Namespace}}
	return client.IgnoreNotFound(r.Delete(ctx, lease))
}

func (r *VirtualizationValidationReconciler) cancel(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation) error {
	jobName := validation.Status.JobName
	if jobName == "" {
		jobName = runName(validation, "vv")
	}
	job := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: jobName}, job); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if job.Name != "" {
		if err := r.Delete(ctx, job); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	if result, err := r.cancelReport(ctx, validation); err != nil {
		return err
	} else if result != nil {
		if _, err := r.observeReport(ctx, validation, result); err != nil {
			return err
		}
	}
	if err := r.releaseLease(ctx, validation); err != nil {
		return err
	}
	return r.setStatus(ctx, validation, validationv1alpha1.ValidationPhaseCancelled, validationv1alpha1.ValidationVerdictInconclusive, "Cancelled", "validation was cancelled by request")
}

func (r *VirtualizationValidationReconciler) cancelReport(ctx context.Context, validation *validationv1alpha1.VirtualizationValidation) (*corev1.ConfigMap, error) {
	result := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: runName(validation, "vr")}, result); apierrors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if result.Data[reportKey] == "" {
		return nil, nil
	}
	report := ctrfReport{}
	if err := json.Unmarshal([]byte(result.Data[reportKey]), &report); err != nil || !validReport(report) {
		return nil, nil
	}
	for i := range report.Results.Tests {
		test := &report.Results.Tests[i]
		if test.Status == "pending" {
			test.Status = "skipped"
			test.Message = "not run because validation was cancelled"
		}
		for j := range test.Steps {
			step := &test.Steps[j]
			if step.Status == "pending" {
				step.Status = "skipped"
				if step.Extra == nil {
					step.Extra = map[string]any{}
				}
				step.Extra["state"] = "cancelled"
			}
		}
	}
	report.Results.Summary.Skipped += report.Results.Summary.Pending
	report.Results.Summary.Pending = 0
	data, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	result.Data[reportKey] = string(data)
	if err := r.Update(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

func contains(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

func remove(values []string, value string) []string {
	filtered := values[:0]
	for _, current := range values {
		if current != value {
			filtered = append(filtered, current)
		}
	}
	return filtered
}

func validReport(report ctrfReport) bool {
	summary := report.Results.Summary
	if report.ReportFormat != "CTRF" || report.SpecVersion == "" || summary.Tests <= 0 || summary.Passed < 0 || summary.Failed < 0 || summary.Pending < 0 || summary.Skipped < 0 || summary.Other < 0 || summary.Tests != summary.Passed+summary.Failed+summary.Pending+summary.Skipped+summary.Other {
		return false
	}
	reduction, reduced := reportReduction(report)
	if (!reduced && summary.Tests != len(report.Results.Tests)) || (reduced && (len(report.Results.Tests) > summary.Tests || reduction.TotalExecutions != int32(summary.Tests) || reduction.Level < 1 || reduction.Level > 7 || reduction.PendingCollapsed < 0 || reduction.PendingCollapsed > int32(summary.Pending+summary.Skipped) || reduction.PassedCollapsed < 0 || reduction.PassedCollapsed > int32(summary.Passed) || reduction.RequiredFailuresOmitted < 0 || reduction.AdvisoryFailuresOmitted < 0 || reduction.RequiredInconclusiveOmitted < 0 || reduction.AdvisoryInconclusiveOmitted < 0 || reduction.RequiredFailuresOmitted+reduction.AdvisoryFailuresOmitted+reduction.RequiredInconclusiveOmitted+reduction.AdvisoryInconclusiveOmitted > int32(summary.Failed+summary.Skipped+summary.Other))) {
		return false
	}
	for _, test := range report.Results.Tests {
		if test.Name == "" || !contains([]string{"passed", "failed", "pending", "skipped", "other"}, test.Status) {
			return false
		}
	}
	return true
}

func reportReduction(report ctrfReport) (validationv1alpha1.ValidationReportReduction, bool) {
	raw, found := report.Results.Extra["forklift.konveyor.io/reduction"]
	if !found {
		return validationv1alpha1.ValidationReportReduction{}, false
	}
	var reduction ctrfReduction
	if err := json.Unmarshal(raw, &reduction); err != nil {
		return validationv1alpha1.ValidationReportReduction{}, false
	}
	return validationv1alpha1.ValidationReportReduction{Level: reduction.Level, TotalExecutions: reduction.TotalExecutions, PendingCollapsed: reduction.PendingCollapsed, PassedCollapsed: reduction.PassedCollapsed, RequiredFailuresOmitted: reduction.RequiredFailuresOmitted, AdvisoryFailuresOmitted: reduction.AdvisoryFailuresOmitted, RequiredInconclusiveOmitted: reduction.RequiredInconclusiveOmitted, AdvisoryInconclusiveOmitted: reduction.AdvisoryInconclusiveOmitted}, true
}

func ctrfOutcome(status string, extra map[string]any) validationv1alpha1.ValidationOutcome {
	if extra != nil {
		switch extra["state"] {
		case "timedOut":
			return validationv1alpha1.ValidationOutcomeTimedOut
		case "cancelled":
			return validationv1alpha1.ValidationOutcomeCancelled
		case "running":
			return validationv1alpha1.ValidationOutcomeRunning
		}
	}
	switch status {
	case "passed":
		return validationv1alpha1.ValidationOutcomePassed
	case "failed":
		return validationv1alpha1.ValidationOutcomeFailed
	case "skipped":
		return validationv1alpha1.ValidationOutcomeSkipped
	case "other":
		return validationv1alpha1.ValidationOutcomeError
	default:
		return validationv1alpha1.ValidationOutcomePending
	}
}

func ctrfTime(milliseconds int64) *metav1.Time {
	if milliseconds <= 0 {
		return nil
	}
	timestamp := metav1.NewTime(time.UnixMilli(milliseconds))
	return &timestamp
}

func ctrfTestOutcome(test ctrfTest, steps []validationv1alpha1.ValidationStep) validationv1alpha1.ValidationOutcome {
	if outcome, found := test.Extra["forklift.konveyor.io/outcome"].(string); found {
		switch validationv1alpha1.ValidationOutcome(outcome) {
		case validationv1alpha1.ValidationOutcomeFailed, validationv1alpha1.ValidationOutcomeTimedOut, validationv1alpha1.ValidationOutcomeError, validationv1alpha1.ValidationOutcomeSkipped, validationv1alpha1.ValidationOutcomeCancelled:
			return validationv1alpha1.ValidationOutcome(outcome)
		}
	}
	for _, step := range steps {
		if step.Outcome == validationv1alpha1.ValidationOutcomeTimedOut || step.Outcome == validationv1alpha1.ValidationOutcomeCancelled || step.Outcome == validationv1alpha1.ValidationOutcomeError {
			return step.Outcome
		}
	}
	return ctrfOutcome(test.Status, nil)
}

func ctrfStepReason(step ctrfStep) string {
	outcome := ctrfOutcome(step.Status, step.Extra)
	if outcome == validationv1alpha1.ValidationOutcomeTimedOut {
		switch strings.ToLower(step.Name) {
		case "snapshot":
			return "SnapshotTimedOut"
		case "live migrate vm":
			return "MigrationTimedOut"
		case "start vm":
			return "VMBootTimedOut"
		}
	}
	return ctrfReasonForOutcome(outcome)
}

func executionReason(outcome validationv1alpha1.ValidationOutcome, steps []validationv1alpha1.ValidationStep) string {
	for _, step := range steps {
		if step.Outcome == outcome && step.Reason != "" {
			return step.Reason
		}
	}
	return ctrfReasonForOutcome(outcome)
}

func ctrfReasonForOutcome(outcome validationv1alpha1.ValidationOutcome) string {
	switch outcome {
	case validationv1alpha1.ValidationOutcomeTimedOut:
		return "ExecutionTimedOut"
	case validationv1alpha1.ValidationOutcomeCancelled:
		return "Cancelled"
	case validationv1alpha1.ValidationOutcomeFailed:
		return "CheckFailed"
	case validationv1alpha1.ValidationOutcomeSkipped:
		return "Skipped"
	case validationv1alpha1.ValidationOutcomeError:
		return "InfrastructureError"
	default:
		return ""
	}
}

// stableCheckID keeps the Kubernetes API independent from runner paths and
// versioned profile names. Forklift may add mappings without changing the contract.
func stableCheckID(testID string) string {
	if checkID, found := stableCheckIDs[testID]; found {
		return checkID
	}
	sum := sha256.Sum256([]byte(testID))
	return fmt.Sprintf("unknown/%x", sum[:6])
}

var stableCheckIDs = map[string]string{
	"basic-v1/canary-workflow":                                "canary/workflow",
	"basic-all-nodes-v1/canary-workflow":                      "canary/workflow",
	"basic-v1/canary-vm":                                      "canary/workflow",
	"10-openshift.d/00-login.d/test.sh":                       "openshift/login",
	"50-openshift-virtualization.d/00-installation.d/test.sh": "virtualization/installation",
	"50-openshift-virtualization.d/10-quota.d/test.sh":        "virtualization/quota",
}

func publicExecutionID(executionID string) string {
	if strings.HasPrefix(executionID, "checks.d/") || strings.Contains(executionID, ".d/") {
		return "cluster"
	}
	return executionID
}

func publicExecutionName(name, testID, checkID string) string {
	if name == testID || strings.HasPrefix(name, "checks.d/") || strings.Contains(name, ".d/") {
		return checkID
	}
	return name
}

func aggregateVerdict(summary validationv1alpha1.VirtualizationValidationSummary, executions []validationv1alpha1.VirtualizationValidationExecution, reduction validationv1alpha1.ValidationReportReduction, complete bool) validationv1alpha1.ValidationVerdict {
	if !complete {
		return validationv1alpha1.ValidationVerdictUnknown
	}
	if reduction.RequiredFailuresOmitted > 0 {
		return validationv1alpha1.ValidationVerdictInvalid
	}
	if reduction.RequiredInconclusiveOmitted > 0 {
		return validationv1alpha1.ValidationVerdictInconclusive
	}
	hasWarning := false
	nonPassing := reduction.RequiredFailuresOmitted + reduction.AdvisoryFailuresOmitted + reduction.RequiredInconclusiveOmitted + reduction.AdvisoryInconclusiveOmitted
	if reduction.AdvisoryFailuresOmitted > 0 || reduction.AdvisoryInconclusiveOmitted > 0 {
		hasWarning = true
	}
	for _, execution := range executions {
		if execution.Severity == validationv1alpha1.ValidationSeverityAdvisory && execution.Reason == "AdvisoryWarning" {
			hasWarning = true
		}
		switch execution.Outcome {
		case validationv1alpha1.ValidationOutcomeTimedOut, validationv1alpha1.ValidationOutcomeError, validationv1alpha1.ValidationOutcomeSkipped, validationv1alpha1.ValidationOutcomeCancelled:
			nonPassing++
			if execution.Severity == validationv1alpha1.ValidationSeverityRequired {
				return validationv1alpha1.ValidationVerdictInconclusive
			}
			hasWarning = true
		case validationv1alpha1.ValidationOutcomeFailed:
			nonPassing++
			if execution.Severity == validationv1alpha1.ValidationSeverityRequired {
				return validationv1alpha1.ValidationVerdictInvalid
			}
			hasWarning = true
		}
	}
	if int32(summary.Failed+summary.Skipped+summary.Other) > nonPassing {
		return validationv1alpha1.ValidationVerdictInconclusive
	}
	if hasWarning {
		return validationv1alpha1.ValidationVerdictValidWithWarnings
	}
	return validationv1alpha1.ValidationVerdictValid
}

func severityForCheck(checkID string) validationv1alpha1.ValidationSeverity {
	if severity, found := checkSeverities[checkID]; found {
		return severity
	}
	return validationv1alpha1.ValidationSeverityRequired
}

var checkSeverities = map[string]validationv1alpha1.ValidationSeverity{
	"openshift/login":             validationv1alpha1.ValidationSeverityRequired,
	"virtualization/installation": validationv1alpha1.ValidationSeverityRequired,
	"virtualization/quota":        validationv1alpha1.ValidationSeverityAdvisory,
	"canary/workflow":             validationv1alpha1.ValidationSeverityRequired,
}

func reduceStatusProjection(status *validationv1alpha1.VirtualizationValidationStatus) {
	if len(status.Executions) == 0 {
		return
	}
	if status.Reduction.TotalExecutions == 0 {
		status.Reduction.TotalExecutions = status.Summary.Total
	}
	needsReduction := func() bool {
		data, err := json.Marshal(status)
		return err != nil || len(status.Executions) > maxResults || len(data) > maxStatusBytes
	}
	if !needsReduction() {
		return
	}
	retained := make([]validationv1alpha1.VirtualizationValidationExecution, 0, len(status.Executions))
	for _, execution := range status.Executions {
		if execution.Outcome == validationv1alpha1.ValidationOutcomePending || execution.Outcome == validationv1alpha1.ValidationOutcomeRunning {
			status.Reduction.PendingCollapsed++
			continue
		}
		retained = append(retained, execution)
	}
	status.Executions = retained
	status.Reduction.Level = max(status.Reduction.Level, 2)
	if !needsReduction() {
		return
	}
	for index := range status.Executions {
		execution := &status.Executions[index]
		if execution.Outcome == validationv1alpha1.ValidationOutcomePassed {
			execution.Parameters = nil
			execution.Steps = nil
			execution.Message = ""
			execution.DurationMilliseconds = 0
			execution.StartedAt = nil
			execution.CompletedAt = nil
		}
	}
	status.Reduction.Level = max(status.Reduction.Level, 3)
	passed := 0
	for _, execution := range status.Executions {
		if execution.Outcome == validationv1alpha1.ValidationOutcomePassed {
			passed++
		}
	}
	if needsReduction() || passed > statusPassedDetailLimit {
		retained = retained[:0]
		keptPassed := false
		for _, execution := range status.Executions {
			if execution.Outcome == validationv1alpha1.ValidationOutcomePassed {
				if !keptPassed {
					execution.Representative = true
					retained = append(retained, execution)
					keptPassed = true
					continue
				}
				status.Reduction.PassedCollapsed++
				continue
			}
			retained = append(retained, execution)
		}
		status.Executions = retained
		status.Reduction.Level = max(status.Reduction.Level, 4)
	}
	if !needsReduction() {
		return
	}
	retained = retained[:0]
	exemplars := map[string]int{}
	for _, execution := range status.Executions {
		if !unsuccessful(execution.Outcome) {
			retained = append(retained, execution)
			continue
		}
		key := execution.CheckID + "/" + execution.Reason
		exemplars[key]++
		if exemplars[key] <= maxFailureExemplars {
			retained = append(retained, execution)
			continue
		}
		recordOmittedOutcome(&status.Reduction, execution)
	}
	status.Executions = retained
	status.Reduction.Level = max(status.Reduction.Level, 5)
	if !needsReduction() {
		return
	}
	retained = retained[:0]
	unsuccessfulExecutions := make([]validationv1alpha1.VirtualizationValidationExecution, 0, len(status.Executions))
	for _, execution := range status.Executions {
		if unsuccessful(execution.Outcome) {
			unsuccessfulExecutions = append(unsuccessfulExecutions, execution)
		} else {
			retained = append(retained, execution)
		}
	}
	sort.SliceStable(unsuccessfulExecutions, func(i, j int) bool {
		return unsuccessfulExecutions[i].Severity == validationv1alpha1.ValidationSeverityRequired && unsuccessfulExecutions[j].Severity != validationv1alpha1.ValidationSeverityRequired
	})
	for index, execution := range unsuccessfulExecutions {
		if index < maxFailureDetails {
			retained = append(retained, execution)
			continue
		}
		recordOmittedOutcome(&status.Reduction, execution)
	}
	status.Executions = retained
	status.Reduction.Level = max(status.Reduction.Level, 6)
	if !needsReduction() {
		return
	}
	for _, execution := range status.Executions {
		if unsuccessful(execution.Outcome) {
			recordOmittedOutcome(&status.Reduction, execution)
		}
	}
	status.Executions = nil
	status.Reduction.Level = 7
}

func unsuccessful(outcome validationv1alpha1.ValidationOutcome) bool {
	return outcome != validationv1alpha1.ValidationOutcomePassed && outcome != validationv1alpha1.ValidationOutcomePending && outcome != validationv1alpha1.ValidationOutcomeRunning
}

func recordOmittedOutcome(reduction *validationv1alpha1.ValidationReportReduction, execution validationv1alpha1.VirtualizationValidationExecution) {
	if execution.Outcome == validationv1alpha1.ValidationOutcomeFailed {
		if execution.Severity == validationv1alpha1.ValidationSeverityAdvisory {
			reduction.AdvisoryFailuresOmitted++
		} else {
			reduction.RequiredFailuresOmitted++
		}
		return
	}
	if execution.Severity == validationv1alpha1.ValidationSeverityAdvisory {
		reduction.AdvisoryInconclusiveOmitted++
	} else {
		reduction.RequiredInconclusiveOmitted++
	}
}

func validSpec(validation *validationv1alpha1.VirtualizationValidation) error {
	if validation.Spec.Target.CredentialsSecret.Name == "" {
		return fmt.Errorf("target.credentialsSecret is required")
	}
	tools := toolDownloads(validation)
	for name, value := range map[string]string{"target.toolDownloads.ocURL": tools.OCURL, "target.toolDownloads.virtctlURL": tools.VirtctlURL} {
		if value != "" {
			endpoint, err := url.ParseRequestURI(value)
			if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
				return fmt.Errorf("%s must be an absolute HTTPS URL", name)
			}
		}
	}
	target, err := url.ParseRequestURI(validation.Spec.Target.URL)
	if err != nil || target.Scheme != "https" || target.Host == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return fmt.Errorf("target.url must be an absolute HTTPS URL without user information, query, or fragment")
	}
	if _, err := targetURLPort(validation.Spec.Target.URL); err != nil {
		return fmt.Errorf("target.url has an invalid port: %w", err)
	}
	if validation.Spec.Run.SuiteTimeoutSeconds < 0 || validation.Spec.Run.SuiteTimeoutSeconds > maxSuiteTimeout {
		return fmt.Errorf("suiteTimeoutSeconds must be between 1 and %d when specified", maxSuiteTimeout)
	}
	if validation.Spec.Run.ExecutionTimeoutSeconds < 0 || validation.Spec.Run.ExecutionTimeoutSeconds > maxExecutionTimeout {
		return fmt.Errorf("executionTimeoutSeconds must be between 1 and %d when specified", maxExecutionTimeout)
	}
	if effectiveExecutionTimeout(validation.Spec.Run.ExecutionTimeoutSeconds) > effectiveSuiteTimeout(validation.Spec.Run.SuiteTimeoutSeconds) {
		return fmt.Errorf("executionTimeoutSeconds must not exceed suiteTimeoutSeconds")
	}
	if policy := validation.Spec.Run.CleanupPolicy; policy != "" && policy != "Always" && policy != "OnFailure" && policy != "Never" {
		return fmt.Errorf("cleanupPolicy must be Always, OnFailure, or Never")
	}
	if validation.Spec.Run.MaxConcurrentExecutions < 0 || validation.Spec.Run.MaxConcurrentExecutions > maxConcurrentRuns {
		return fmt.Errorf("maxConcurrentExecutions must be between 1 and %d when specified", maxConcurrentRuns)
	}
	if validation.Spec.Run.QueueTimeoutSeconds < 0 || validation.Spec.Run.QueueTimeoutSeconds > maxQueueTimeout {
		return fmt.Errorf("queueTimeoutSeconds must be between 0 and %d", maxQueueTimeout)
	}
	if validation.Spec.Workload.Namespace != "" && len(kvalidation.IsDNS1123Label(validation.Spec.Workload.Namespace)) > 0 {
		return fmt.Errorf("workload.namespace must be a DNS label")
	}
	if validation.Spec.Workload.DataSource.Namespace != "" && len(kvalidation.IsDNS1123Label(validation.Spec.Workload.DataSource.Namespace)) > 0 {
		return fmt.Errorf("workload.dataSource.namespace must be a DNS label")
	}
	if validation.Spec.Workload.DataSource.Name != "" && len(kvalidation.IsDNS1123Label(validation.Spec.Workload.DataSource.Name)) > 0 {
		return fmt.Errorf("workload.dataSource.name must be a DNS label")
	}
	return nil
}

func targetURLPort(rawURL string) (int, error) {
	target, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return 0, err
	}
	port := target.Port()
	if port == "" {
		return 443, nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return 0, fmt.Errorf("must be between 1 and 65535")
	}
	return number, nil
}

func toolDownloads(validation *validationv1alpha1.VirtualizationValidation) validationv1alpha1.ValidationToolDownloads {
	if validation.Spec.Target.ToolDownloads == nil {
		return validationv1alpha1.ValidationToolDownloads{}
	}
	return *validation.Spec.Target.ToolDownloads
}

func effectiveSuiteTimeout(timeout int64) int64 {
	if timeout == 0 {
		return defaultSuiteTimeout
	}
	return timeout
}

func effectiveExecutionTimeout(timeout int64) int64 {
	if timeout == 0 {
		return defaultExecutionTimeout
	}
	return timeout
}

func effectiveQueueTimeout(timeout, suiteTimeout int64) time.Duration {
	if timeout == 0 {
		timeout = effectiveSuiteTimeout(suiteTimeout)
	}
	return time.Duration(timeout) * time.Second
}

func effectiveCleanupPolicy(policy string) string {
	if policy == "" {
		return "Always"
	}
	return policy
}

func tokenKeyFor(validation *validationv1alpha1.VirtualizationValidation) string {
	if validation.Spec.Target.CredentialsSecret.TokenKey == "" {
		return tokenKey
	}
	return validation.Spec.Target.CredentialsSecret.TokenKey
}

func caKeyFor(validation *validationv1alpha1.VirtualizationValidation) string {
	return validation.Spec.Target.CredentialsSecret.CAKey
}

func dataSourceAPIGroup(reference validationv1alpha1.WorkloadDataSourceReference) string {
	if reference.APIGroup == "" {
		return "cdi.kubevirt.io"
	}
	return reference.APIGroup
}

func dataSourceKind(reference validationv1alpha1.WorkloadDataSourceReference) string {
	if reference.Kind == "" {
		return "DataSource"
	}
	return reference.Kind
}

func dataSourceName(reference validationv1alpha1.WorkloadDataSourceReference) string {
	return reference.Namespace + "/" + reference.Name
}

func terminal(phase validationv1alpha1.ValidationPhase) bool {
	return phase == validationv1alpha1.ValidationPhaseCompleted || phase == validationv1alpha1.ValidationPhaseCancelled || phase == validationv1alpha1.ValidationPhaseError
}

func proxyEnvironment() []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, 4)
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
		if value, found := os.LookupEnv(name); found && value != "" {
			result = append(result, corev1.EnvVar{Name: name, Value: value})
		}
	}
	return result
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func ptr[T any](value T) *T { return &value }
