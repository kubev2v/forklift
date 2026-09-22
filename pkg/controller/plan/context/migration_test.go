package context

import (
	"slices"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

func boolPtr(v bool) *bool { return &v }

func TestSelinuxRelabelAtBoot(t *testing.T) {
	tests := []struct {
		name     string
		planSpec api.PlanSpec
		vmRef    ref.Ref
		expected bool
	}{
		{
			name:     "defaults to false",
			planSpec: api.PlanSpec{},
			vmRef:    ref.Ref{ID: "vm-1"},
			expected: false,
		},
		{
			name: "plan-level true",
			planSpec: api.PlanSpec{
				SelinuxRelabelAtBoot: true,
			},
			vmRef:    ref.Ref{ID: "vm-1"},
			expected: true,
		},
		{
			name: "vm override disables plan default",
			planSpec: api.PlanSpec{
				SelinuxRelabelAtBoot: true,
				VMs: []plan.VM{
					{Ref: ref.Ref{ID: "vm-1"}, SelinuxRelabelAtBoot: boolPtr(false)},
				},
			},
			vmRef:    ref.Ref{ID: "vm-1"},
			expected: false,
		},
		{
			name: "vm override enables plan default",
			planSpec: api.PlanSpec{
				VMs: []plan.VM{
					{Ref: ref.Ref{ID: "vm-1"}, SelinuxRelabelAtBoot: boolPtr(true)},
				},
			},
			vmRef:    ref.Ref{ID: "vm-1"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Plan: &api.Plan{Spec: tt.planSpec},
			}
			if got := ctx.SelinuxRelabelAtBoot(tt.vmRef); got != tt.expected {
				t.Errorf("SelinuxRelabelAtBoot() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSelinuxRelabelExclude(t *testing.T) {
	tests := []struct {
		name     string
		planSpec api.PlanSpec
		vmRef    ref.Ref
		expected []string
	}{
		{
			name:     "defaults to nil",
			planSpec: api.PlanSpec{},
			vmRef:    ref.Ref{ID: "vm-1"},
			expected: nil,
		},
		{
			name: "plan-level dirs",
			planSpec: api.PlanSpec{
				SelinuxRelabelExclude: []string{"/foo", "/bar"},
			},
			vmRef:    ref.Ref{ID: "vm-1"},
			expected: []string{"/foo", "/bar"},
		},
		{
			name: "vm override replaces plan default",
			planSpec: api.PlanSpec{
				SelinuxRelabelExclude: []string{"/foo"},
				VMs: []plan.VM{
					{Ref: ref.Ref{ID: "vm-1"}, SelinuxRelabelExclude: []string{"/baz"}},
				},
			},
			vmRef:    ref.Ref{ID: "vm-1"},
			expected: []string{"/baz"},
		},
		{
			name: "vm empty slice clears plan default",
			planSpec: api.PlanSpec{
				SelinuxRelabelExclude: []string{"/foo"},
				VMs: []plan.VM{
					{Ref: ref.Ref{ID: "vm-1"}, SelinuxRelabelExclude: []string{}},
				},
			},
			vmRef:    ref.Ref{ID: "vm-1"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Plan: &api.Plan{Spec: tt.planSpec},
			}
			got := ctx.SelinuxRelabelExclude(tt.vmRef)
			if !slices.Equal(got, tt.expected) {
				t.Errorf("SelinuxRelabelExclude() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsResumeConversion(t *testing.T) {
	tests := []struct {
		name      string
		migration *api.Migration
		expected  bool
	}{
		{
			name:      "nil migration returns false",
			migration: nil,
			expected:  false,
		},
		{
			name:      "migration without resumeConversion returns false",
			migration: &api.Migration{},
			expected:  false,
		},
		{
			name: "migration with resumeConversion=true returns true",
			migration: &api.Migration{
				Spec: api.MigrationSpec{
					ResumeConversion: true,
				},
			},
			expected: true,
		},
		{
			name: "migration with resumeConversion=false returns false",
			migration: &api.Migration{
				Spec: api.MigrationSpec{
					ResumeConversion: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Migration: tt.migration,
			}
			if got := ctx.IsResumeConversion(); got != tt.expected {
				t.Errorf("IsResumeConversion() = %v, want %v", got, tt.expected)
			}
		})
	}
}
