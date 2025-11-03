package plan

import (
	"context"
	"fmt"
	"strconv"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	discovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	sourceNamespace  = "source-namespace"
	destNamespace    = "destination-namespace"
	testNamespace    = "test-namespace"
	sourceName       = "source"
	destName         = "destination"
	sourceSecretName = "source-secret"
	testPlanName     = "test-plan"
	tokenKey         = "token"
	tokenValue       = "token"
	insecureSkipKey  = "inscureSkipVerify"
)

var (
	planValidationLog = logging.WithName("planValidation")
)

var _ = ginkgo.Describe("Plan Validations", func() {
	var (
		fakeClientSet *fake.Clientset
		reconciler    *Reconciler
	)

	ginkgo.BeforeEach(func() {
		reconciler = &Reconciler{
			base.Reconciler{},
		}
		fakeClientSet = fake.NewSimpleClientset()
	})

	ginkgo.Describe("validateOCPVersion", func() {
		ginkgo.DescribeTable("should validate OpenShift version correctly",
			func(major, minor string, shouldError bool) {
				fakeDiscovery, ok := fakeClientSet.Discovery().(*discovery.FakeDiscovery)
				gomega.Expect(ok).To(gomega.BeTrue())
				fakeDiscovery.FakedServerVersion = &version.Info{
					Major: major, Minor: minor,
				}

				err := reconciler.checkOCPVersion(fakeClientSet)

				if shouldError {
					gomega.Expect(err).To(gomega.HaveOccurred())
				} else {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},

			// Directly declare entries here
			ginkgo.Entry("when the OpenShift version is supported", "1", "26", false),
			ginkgo.Entry("when the OpenShift version is not supported", "1", "25", true),
		)
	})

	ginkgo.Describe("validate", func() {
		ginkgo.It("Should setup secret when source is not local cluster", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "https://source", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			source.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})
			destination.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.ensureSecretForProvider(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// secret should be set on plan.Referenced.Secret
			gomega.Expect(plan.Referenced.Secret).NotTo(gomega.BeNil())
		})

		ginkgo.It("Should not setup secret when source is local cluster", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "https://destination", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			source.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})
			destination.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.ensureSecretForProvider(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// secret should NOT be set on plan.Referenced.Secret
			gomega.Expect(plan.Referenced.Secret).To(gomega.BeNil())
		})
	})
})

//nolint:errcheck
func createFakeReconciler(objects ...runtime.Object) *Reconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	api.SchemeBuilder.AddToScheme(scheme)

	client := fakeClient.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()

	return &Reconciler{
		base.Reconciler{
			Client: client,
			Log:    planValidationLog,
		},
	}
}

func createProvider(name, namespace, url string, providerType api.ProviderType, secret *core.ObjectReference) *api.Provider {
	return &api.Provider{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.ProviderSpec{
			Type:   ptr.To(providerType),
			URL:    url,
			Secret: *secret,
		},
	}
}

func createSecret(name, namespace string, insecure bool) *core.Secret {
	return &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			insecureSkipKey: []byte(strconv.FormatBool(insecure)),
			tokenKey:        []byte(tokenValue),
		},
	}
}

func createPlan(name, namespace string, source, destination *api.Provider) *api.Plan {
	return &api.Plan{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.PlanSpec{
			Provider: provider.Pair{
				Source: core.ObjectReference{
					Name:      source.Name,
					Namespace: source.Namespace,
				},
				Destination: core.ObjectReference{
					Name:      destination.Name,
					Namespace: destination.Namespace,
				},
			},
		},
		Referenced: api.Referenced{
			Provider: struct {
				Source      *api.Provider
				Destination *api.Provider
			}{
				Source:      source,
				Destination: destination,
			},
		},
	}
}

var _ = ginkgo.Describe("Inventory Service Retry Logic", func() {
	var (
		reconciler *Reconciler
	)

	ginkgo.BeforeEach(func() {
		reconciler = &Reconciler{
			base.Reconciler{},
		}
	})

	ginkgo.Describe("extractHTTPStatus", func() {
		ginkgo.It("should extract HTTP status from error message", func() {
			status := reconciler.extractHTTPStatus(fmt.Errorf("GET failed. status: 401 url: http://example.com"))
			gomega.Expect(status).To(gomega.Equal(401))

			status = reconciler.extractHTTPStatus(fmt.Errorf("POST failed. status: 403 url: http://example.com"))
			gomega.Expect(status).To(gomega.Equal(403))

			status = reconciler.extractHTTPStatus(fmt.Errorf("PUT failed. status: 500 url: http://example.com"))
			gomega.Expect(status).To(gomega.Equal(500))
		})

		ginkgo.It("should return 0 for errors without status codes", func() {
			status := reconciler.extractHTTPStatus(fmt.Errorf("connection timeout"))
			gomega.Expect(status).To(gomega.Equal(0))

			status = reconciler.extractHTTPStatus(fmt.Errorf("network unreachable"))
			gomega.Expect(status).To(gomega.Equal(0))

			status = reconciler.extractHTTPStatus(fmt.Errorf("invalid error format"))
			gomega.Expect(status).To(gomega.Equal(0))
		})

		ginkgo.It("should handle malformed status codes", func() {
			status := reconciler.extractHTTPStatus(fmt.Errorf("GET failed. status: abc url: http://example.com"))
			gomega.Expect(status).To(gomega.Equal(0))

			status = reconciler.extractHTTPStatus(fmt.Errorf("GET failed. status: url: http://example.com"))
			gomega.Expect(status).To(gomega.Equal(0))
		})

		ginkgo.It("should handle liberr.Error with context", func() {
			// Create a mock liberr.Error with status in context
			liberrErr := &liberr.Error{
				Context: []interface{}{"status", 404, "url", "http://example.com"},
			}
			status := reconciler.extractHTTPStatus(liberrErr)
			gomega.Expect(status).To(gomega.Equal(404))
		})
	})

	ginkgo.Describe("classifyInventoryError", func() {
		ginkgo.It("should classify notRetryable HTTP errors correctly", func() {
			// 401 Unauthorized
			isNotRetryable, reason := reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 401 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeTrue())
			gomega.Expect(reason).To(gomega.ContainSubstring("authentication/authorization error"))

			// 403 Forbidden
			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 403 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeTrue())
			gomega.Expect(reason).To(gomega.ContainSubstring("authentication/authorization error"))

			// 400 Bad Request
			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 400 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeTrue())
			gomega.Expect(reason).To(gomega.ContainSubstring("bad request"))

			// 405 Method Not Allowed
			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 405 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeTrue())
			gomega.Expect(reason).To(gomega.ContainSubstring("method not allowed"))
		})

		ginkgo.It("should classify 404 errors based on context", func() {
			// 404 with "not found" in message - notRetryable
			isNotRetryable, reason := reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 404 url: http://example.com - resource not found"))
			gomega.Expect(isNotRetryable).To(gomega.BeTrue())
			gomega.Expect(reason).To(gomega.ContainSubstring("resource not found"))

			// 404 without "not found" in message - retryable (service unavailable)
			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 404 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.ContainSubstring("inventory service unavailable"))
		})

		ginkgo.It("should classify retryable HTTP errors correctly", func() {
			// 503 Service Unavailable
			isNotRetryable, reason := reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 503 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.ContainSubstring("service unavailable"))

			// 502 Bad Gateway
			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 502 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.ContainSubstring("service unavailable"))

			// 504 Gateway Timeout
			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 504 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.ContainSubstring("service unavailable"))

			// 429 Too Many Requests
			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("GET failed. status: 429 url: http://example.com"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.ContainSubstring("rate limited"))
		})

		ginkgo.It("should classify network errors as retryable", func() {
			isNotRetryable, reason := reconciler.classifyInventoryError(fmt.Errorf("connection timeout"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.ContainSubstring("network connectivity issue"))

			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("connection refused"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.ContainSubstring("network connectivity issue"))

			isNotRetryable, reason = reconciler.classifyInventoryError(fmt.Errorf("network unreachable"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.ContainSubstring("network connectivity issue"))
		})

		ginkgo.It("should default unknown errors to retryable", func() {
			isNotRetryable, reason := reconciler.classifyInventoryError(fmt.Errorf("unknown error"))
			gomega.Expect(isNotRetryable).To(gomega.BeFalse())
			gomega.Expect(reason).To(gomega.Equal("unknown error, defaulting to retryable"))
		})
	})

	ginkgo.Describe("retryWithBackoff", func() {
		ginkgo.It("should succeed on first attempt", func() {
			attemptCount := 0
			operation := func() error {
				attemptCount++
				return nil // Success immediately
			}

			err := reconciler.retryWithBackoff(context.Background(), operation, "test operation")
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(attemptCount).To(gomega.Equal(1))
		})

		ginkgo.It("should retry with exponential backoff and succeed", func() {
			attemptCount := 0
			operation := func() error {
				attemptCount++
				if attemptCount < 3 {
					return fmt.Errorf("temporary network error")
				}
				return nil // Success on 3rd attempt
			}

			err := reconciler.retryWithBackoff(context.Background(), operation, "test operation")
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(attemptCount).To(gomega.Equal(3))
		})

		ginkgo.It("should not retry notRetryable errors", func() {
			attemptCount := 0
			operation := func() error {
				attemptCount++
				return fmt.Errorf("GET failed. status: 401 url: http://example.com")
			}

			err := reconciler.retryWithBackoff(context.Background(), operation, "test operation")
			gomega.Expect(err).To(gomega.Not(gomega.BeNil()))
			gomega.Expect(attemptCount).To(gomega.Equal(1)) // Should not retry notRetryable errors
		})

		ginkgo.It("should exhaust all retries for retryable errors", func() {
			attemptCount := 0
			operation := func() error {
				attemptCount++
				return fmt.Errorf("temporary network error")
			}

			err := reconciler.retryWithBackoff(context.Background(), operation, "test operation")
			gomega.Expect(err).To(gomega.Not(gomega.BeNil()))
			gomega.Expect(attemptCount).To(gomega.Equal(4)) // MaxRetries + 1 = 4 attempts
		})

		ginkgo.It("should respect context cancellation", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			attemptCount := 0
			operation := func() error {
				attemptCount++
				return fmt.Errorf("temporary network error")
			}

			err := reconciler.retryWithBackoff(ctx, operation, "test operation")
			gomega.Expect(err).To(gomega.Equal(context.Canceled))
			gomega.Expect(attemptCount).To(gomega.Equal(1)) // Should not retry after cancellation
		})

		ginkgo.It("should handle context timeout", func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
			defer cancel()

			attemptCount := 0
			operation := func() error {
				attemptCount++
				time.Sleep(time.Millisecond * 20) // Longer than context timeout
				return fmt.Errorf("temporary network error")
			}

			err := reconciler.retryWithBackoff(ctx, operation, "test operation")
			gomega.Expect(err).To(gomega.Equal(context.DeadlineExceeded))
		})

		ginkgo.It("should cap delay at MaxDelay", func() {
			// This test verifies that delay doesn't exceed MaxDelay (30 seconds)
			// We can't easily test the exact timing, but we can verify the logic
			attemptCount := 0
			operation := func() error {
				attemptCount++
				return fmt.Errorf("temporary network error")
			}

			start := time.Now()
			err := reconciler.retryWithBackoff(context.Background(), operation, "test operation")
			duration := time.Since(start)

			gomega.Expect(err).To(gomega.Not(gomega.BeNil()))
			gomega.Expect(attemptCount).To(gomega.Equal(4))
			// Should take at least 2+4+8+16 = 30 seconds, but not much more
			gomega.Expect(duration).To(gomega.BeNumerically(">=", time.Second*30))
			gomega.Expect(duration).To(gomega.BeNumerically("<", time.Second*35)) // Allow some tolerance
		})
	})
})
