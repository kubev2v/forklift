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
		{"aap job template only", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			JobTemplateID: 1,
		}}}, true},
		{"aap with local image rejected", &Hook{Spec: HookSpec{
			Image: "quay.io/x",
			AAP: &AAPConfig{
				JobTemplateID: 1,
			},
		}}, false},
		{"aap with playbook rejected", &Hook{Spec: HookSpec{
			Playbook: "eA==",
			AAP: &AAPConfig{
				JobTemplateID: 1,
			},
		}}, false},
		{"aap zero template id", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			JobTemplateID: 0,
		}}}, false},
		{"aap negative template id", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			JobTemplateID: -1,
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

func TestHookAAPRunnable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		hook       *Hook
		clusterURL string
		clusterTok string
		want       bool
	}{
		{"nil hook", nil, "", "", true},
		{"non-aap hook", &Hook{Spec: HookSpec{Image: "i"}}, "", "", true},
		{"cluster config", &Hook{Spec: HookSpec{AAP: &AAPConfig{JobTemplateID: 1}}},
			"https://aap", "sec", true},
		{"cluster url only", &Hook{Spec: HookSpec{AAP: &AAPConfig{JobTemplateID: 1}}},
			"https://aap", "", false},
		{"per-hook only", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			JobTemplateID: 1,
			URL:           "https://aap",
			TokenSecret:   &core.ObjectReference{Name: "s"},
		}}}, "", "", true},
		{"neither", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			JobTemplateID: 1,
		}}}, "", "", false},
		{"bad template id", &Hook{Spec: HookSpec{AAP: &AAPConfig{
			JobTemplateID: 0,
			URL:           "https://aap",
			TokenSecret:   &core.ObjectReference{Name: "s"},
		}}}, "https://aap", "sec", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HookAAPRunnable(tc.hook, tc.clusterURL, tc.clusterTok); got != tc.want {
				t.Fatalf("HookAAPRunnable() = %v, want %v", got, tc.want)
			}
		})
	}
}
