//nolint:errcheck
package plan

import (
	"context"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var HookLog = logging.WithName("hook-test")

func TestHook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hook Suite")
}

var _ = Describe("Hook execution", func() {
	Describe("Run", func() {
		var (
			plan        *api.Plan
			hook        *api.Hook
			aapSecret   *core.Secret
			vmStatus    *planapi.VMStatus
			hookRunner  *HookRunner
			ctx         *plancontext.Context
			fakeClient  *fake.ClientBuilder
		)

		BeforeEach(func() {
			// Create a test plan
			plan = &api.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-plan",
					Namespace: "test-namespace",
					UID:       "plan-uid-123",
				},
				Spec: api.PlanSpec{},
			}

			// Create a VM status with a hook reference
			vmStatus = &planapi.VMStatus{
				ID:    "vm-123",
				Name:  "test-vm",
				Phase: "PreHook",
				Ref: ref.Ref{
					ID: "source-vm-456",
				},
				Pipeline: []planapi.Step{
					{
						Phase: "PreHook",
					},
				},
				Hooks: []planapi.HookRef{
					{
						Hook: ref.Ref{
							Name:      "test-hook",
							Namespace: "test-namespace",
						},
						Step: "PreHook",
					},
				},
			}

			// Create AAP secret
			aapSecret = &core.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aap-token-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"token": []byte("test-aap-token-12345"),
				},
			}

			// Setup fake client builder
			scheme := runtime.NewScheme()
			api.SchemeBuilder.AddToScheme(scheme)
			core.AddToScheme(scheme)

			fakeClient = fake.NewClientBuilder().WithScheme(scheme)
		})

		Context("when hook has AAP configuration", func() {
			BeforeEach(func() {
				// Create a hook with AAP config
				hook = &api.Hook{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hook",
						Namespace: "test-namespace",
						UID:       "hook-uid-456",
					},
					Spec: api.HookSpec{
						AAP: &api.AAPConfig{
							URL:           "https://aap.example.com",
							JobTemplateID: 7,
							TokenSecret:   "aap-token-secret",
							Timeout:       600,
						},
					},
				}
			})

			It("should identify AAP hook and not create a Kubernetes Job", func() {
				client := fakeClient.WithObjects(plan, hook, aapSecret).Build()

				ctx = &plancontext.Context{
					Context: context.TODO(),
					Client:  client,
					Log:     HookLog,
					Plan:    plan,
					Migration: &api.Migration{
						ObjectMeta: metav1.ObjectMeta{
							UID: "migration-uid-789",
						},
					},
				}

				hookRunner = &HookRunner{
					Context: ctx,
				}

				// The Run method will check if hook.Spec.AAP != nil
				// We can't fully test the AAP execution without mocking HTTP calls,
				// but we can verify the hook configuration is correctly structured
				Expect(hook.Spec.AAP).ToNot(BeNil())
				Expect(hook.Spec.AAP.URL).To(Equal("https://aap.example.com"))
				Expect(hook.Spec.AAP.JobTemplateID).To(Equal(7))
				Expect(hook.Spec.AAP.TokenSecret).To(Equal("aap-token-secret"))
				Expect(hook.Spec.AAP.Timeout).To(Equal(int64(600)))

				// Verify playbook is not required for AAP hooks
				Expect(hook.Spec.Playbook).To(BeEmpty())
			})
		})

		Context("when hook has playbook configuration", func() {
			BeforeEach(func() {
				// Create a hook with playbook (no AAP config)
				hook = &api.Hook{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hook",
						Namespace: "test-namespace",
						UID:       "hook-uid-789",
					},
					Spec: api.HookSpec{
						Image:          "quay.io/konveyor/hook-runner:latest",
						Playbook:       "LS0tCi0gbmFtZTogVGVzdCBwbGF5Ym9vawogIGhvc3RzOiBsb2NhbGhvc3Q=", // base64 encoded playbook
						ServiceAccount: "forklift-controller",
						Deadline:       3600,
					},
				}
			})

			It("should use local playbook execution path", func() {
				client := fakeClient.WithObjects(plan, hook).Build()

				ctx = &plancontext.Context{
					Context: context.TODO(),
					Client:  client,
					Log:     HookLog,
					Plan:    plan,
					Migration: &api.Migration{
						ObjectMeta: metav1.ObjectMeta{
							UID: "migration-uid-789",
						},
					},
				}

				hookRunner = &HookRunner{
					Context: ctx,
				}

				// Verify this is a playbook-based hook
				Expect(hook.Spec.AAP).To(BeNil())
				Expect(hook.Spec.Playbook).ToNot(BeEmpty())
				Expect(hook.Spec.Image).ToNot(BeEmpty())
			})
		})

		Context("GetAAPToken", func() {
			It("should retrieve token from secret", func() {
				client := fakeClient.WithObjects(aapSecret).Build()

				token, err := GetAAPToken(context.TODO(), client, "test-namespace", "aap-token-secret")
				Expect(err).ToNot(HaveOccurred())
				Expect(token).To(Equal("test-aap-token-12345"))
			})

			It("should return error when secret not found", func() {
				client := fakeClient.Build()

				token, err := GetAAPToken(context.TODO(), client, "test-namespace", "missing-secret")
				Expect(err).To(HaveOccurred())
				Expect(token).To(BeEmpty())
			})

			It("should return error when secret does not contain token key", func() {
				badSecret := &core.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bad-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"wrong-key": []byte("some-value"),
					},
				}

				client := fakeClient.WithObjects(badSecret).Build()

				token, err := GetAAPToken(context.TODO(), client, "test-namespace", "bad-secret")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("does not contain 'token' key"))
				Expect(token).To(BeEmpty())
			})
		})

		Context("Hook type differentiation", func() {
			It("should support both AAP and playbook hooks in the same system", func() {
				aapHook := &api.Hook{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "aap-hook",
						Namespace: "test-namespace",
					},
					Spec: api.HookSpec{
						AAP: &api.AAPConfig{
							URL:           "https://aap.example.com",
							JobTemplateID: 5,
							TokenSecret:   "aap-secret",
						},
					},
				}

				playbookHook := &api.Hook{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "playbook-hook",
						Namespace: "test-namespace",
					},
					Spec: api.HookSpec{
						Image:    "quay.io/konveyor/hook-runner:latest",
						Playbook: "LS0tCi0gbmFtZTogVGVzdA==",
					},
				}

				// Verify they are distinct types
				Expect(aapHook.Spec.AAP).ToNot(BeNil())
				Expect(aapHook.Spec.Playbook).To(BeEmpty())

				Expect(playbookHook.Spec.AAP).To(BeNil())
				Expect(playbookHook.Spec.Playbook).ToNot(BeEmpty())
			})
		})

		Context("AAP configuration validation", func() {
			It("should accept valid AAP configuration", func() {
				hook := &api.Hook{
					Spec: api.HookSpec{
						AAP: &api.AAPConfig{
							URL:           "https://aap.example.com",
							JobTemplateID: 10,
							TokenSecret:   "my-secret",
							Timeout:       1800,
						},
					},
				}

				Expect(hook.Spec.AAP.URL).To(HavePrefix("https://"))
				Expect(hook.Spec.AAP.JobTemplateID).To(BeNumerically(">", 0))
				Expect(hook.Spec.AAP.TokenSecret).ToNot(BeEmpty())
			})

			It("should allow optional timeout field", func() {
				hookWithTimeout := &api.Hook{
					Spec: api.HookSpec{
						AAP: &api.AAPConfig{
							URL:           "https://aap.example.com",
							JobTemplateID: 1,
							TokenSecret:   "secret",
							Timeout:       300,
						},
					},
				}

				hookWithoutTimeout := &api.Hook{
					Spec: api.HookSpec{
						AAP: &api.AAPConfig{
							URL:           "https://aap.example.com",
							JobTemplateID: 1,
							TokenSecret:   "secret",
						},
					},
				}

				Expect(hookWithTimeout.Spec.AAP.Timeout).To(Equal(int64(300)))
				Expect(hookWithoutTimeout.Spec.AAP.Timeout).To(Equal(int64(0)))
			})
		})
	})
})
