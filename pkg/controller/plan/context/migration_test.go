package context

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

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
