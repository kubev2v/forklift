package plan

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = ginkgo.Describe("HookRunner template ServiceAccount", func() {
	var (
		savedGlobalSA string
		runner        *HookRunner
		configMap     *core.ConfigMap
	)

	ginkgo.BeforeEach(func() {
		savedGlobalSA = Settings.Migration.ServiceAccount
		Settings.Migration.HooksContainerRequestsCpu = "100m"
		Settings.Migration.HooksContainerRequestsMemory = "128Mi"
		Settings.Migration.HooksContainerLimitsCpu = "1"
		Settings.Migration.HooksContainerLimitsMemory = "512Mi"

		configMap = &core.ConfigMap{
			ObjectMeta: meta.ObjectMeta{
				Name:      "hook-config",
				Namespace: "test-ns",
			},
		}
	})

	ginkgo.AfterEach(func() {
		Settings.Migration.ServiceAccount = savedGlobalSA
	})

	newRunner := func(hookSA, planSA, globalSA string) *HookRunner {
		Settings.Migration.ServiceAccount = globalSA
		return &HookRunner{
			Context: &plancontext.Context{
				Plan: &api.Plan{
					Spec: api.PlanSpec{
						ServiceAccount: planSA,
					},
				},
			},
			hook: &api.Hook{
				Spec: api.HookSpec{
					Image:          "quay.io/kubev2v/hook-runner:latest",
					ServiceAccount: hookSA,
				},
			},
		}
	}

	ginkgo.It("should use hook SA when all three are set", func() {
		runner = newRunner("hook-sa", "plan-sa", "global-sa")
		tmpl := runner.template(configMap)
		Expect(tmpl.Spec.ServiceAccountName).To(Equal("hook-sa"))
	})

	ginkgo.It("should fall back to plan SA when hook SA is empty", func() {
		runner = newRunner("", "plan-sa", "global-sa")
		tmpl := runner.template(configMap)
		Expect(tmpl.Spec.ServiceAccountName).To(Equal("plan-sa"))
	})

	ginkgo.It("should fall back to global SA when hook and plan SAs are empty", func() {
		runner = newRunner("", "", "global-sa")
		tmpl := runner.template(configMap)
		Expect(tmpl.Spec.ServiceAccountName).To(Equal("global-sa"))
	})

	ginkgo.It("should leave ServiceAccountName empty when all SAs are empty", func() {
		runner = newRunner("", "", "")
		tmpl := runner.template(configMap)
		Expect(tmpl.Spec.ServiceAccountName).To(BeEmpty())
	})

	ginkgo.It("should prefer hook SA over plan and global", func() {
		runner = newRunner("hook-sa", "plan-sa", "global-sa")
		tmpl := runner.template(configMap)
		Expect(tmpl.Spec.ServiceAccountName).To(Equal("hook-sa"))
	})

	ginkgo.It("should prefer plan SA over global when hook SA is empty", func() {
		runner = newRunner("", "plan-sa", "global-sa")
		tmpl := runner.template(configMap)
		Expect(tmpl.Spec.ServiceAccountName).To(Equal("plan-sa"))
	})
})
