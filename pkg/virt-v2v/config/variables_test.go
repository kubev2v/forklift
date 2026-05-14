package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectNoFstrimSupport(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "RHEL 10",
			content:  "NAME=\"Red Hat Enterprise Linux\"\nVERSION=\"10\"\nID=\"rhel\"\n",
			expected: true,
		},
		{
			name:     "RHEL 9 unquoted",
			content:  "NAME=Red Hat Enterprise Linux\nVERSION=9\nID=rhel\n",
			expected: true,
		},
		{
			name:     "CentOS Stream 10",
			content:  "NAME=\"CentOS Stream\"\nVERSION=\"10\"\nID=\"centos\"\n",
			expected: false,
		},
		{
			name:     "CentOS Stream 9",
			content:  "NAME=\"CentOS Stream\"\nVERSION=\"9\"\nID=\"centos\"\n",
			expected: false,
		},
		{
			name:     "Fedora 41",
			content:  "NAME=\"Fedora Linux\"\nVERSION=\"41\"\nID=fedora\n",
			expected: false,
		},
		{
			name:     "missing file",
			content:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.content == "" {
				got := detectNoFstrimSupport("/nonexistent/os-release")
				if got != tt.expected {
					t.Errorf("detectNoFstrimSupport() = %v, want %v", got, tt.expected)
				}
				return
			}

			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "os-release")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := detectNoFstrimSupport(path)
			if got != tt.expected {
				t.Errorf("detectNoFstrimSupport() = %v, want %v", got, tt.expected)
			}
		})
	}
}
