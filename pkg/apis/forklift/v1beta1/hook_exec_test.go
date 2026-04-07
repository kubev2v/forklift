package v1beta1

import (
	"testing"

	core "k8s.io/api/core/v1"
)

func TestHookExecutionConfigValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		hook *Hook
		want bool
	}{
		{"nil hook", nil, false},
		{"local image only", &Hook{Spec: HookSpec{Image: "quay.io/x:y"}}, true},
		{"local playbook only", &Hook{Spec: HookSpec{Playbook: "cGxheQ=="}}, false},
		{"local whitespace image", &Hook{Spec: HookSpec{Image: "  "}}, false},
		{"local image and playbook", &Hook{Spec: HookSpec{Image: "i", Playbook: "cGxheQ=="}}, true},
		{"aap complete", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			URL: "https://aap", JobTemplateID: 1,
			TokenSecret: core.ObjectReference{Name: "s"},
		}}}, true},
		{"aap with local image rejected", &Hook{Spec: HookSpec{
			Image: "quay.io/x",
			AAP: &AAPConfig{
				URL: "https://aap", JobTemplateID: 1,
				TokenSecret: core.ObjectReference{Name: "s"},
			},
		}}, false},
		{"aap with playbook rejected", &Hook{Spec: HookSpec{
			Playbook: "eA==",
			AAP: &AAPConfig{
				URL: "https://aap", JobTemplateID: 1,
				TokenSecret: core.ObjectReference{Name: "s"},
			},
		}}, false},
		{"aap zero template id", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			URL: "https://aap", JobTemplateID: 0,
			TokenSecret: core.ObjectReference{Name: "s"},
		}}}, false},
		{"aap negative template id", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			URL: "https://aap", JobTemplateID: -1,
			TokenSecret: core.ObjectReference{Name: "s"},
		}}}, false},
		{"aap whitespace url", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			URL: "   ", JobTemplateID: 1,
			TokenSecret: core.ObjectReference{Name: "s"},
		}}}, false},
		{"aap missing url", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			JobTemplateID: 1, TokenSecret: core.ObjectReference{Name: "s"},
		}}}, false},
		{"aap whitespace token name", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			URL: "https://aap", JobTemplateID: 1,
			TokenSecret: core.ObjectReference{Name: "   "},
		}}}, false},
		{"aap missing token name", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			URL: "https://aap", JobTemplateID: 1, TokenSecret: core.ObjectReference{Name: ""},
		}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HookExecutionConfigValid(tc.hook); got != tc.want {
				t.Fatalf("HookExecutionConfigValid() = %v, want %v", got, tc.want)
			}
		})
	}
}
