//nolint:errcheck
package inventory

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestFindOVAFiles(t *testing.T) {
	g := NewGomegaWithT(t)

	tests := []struct {
		name         string
		setup        func(directory string)
		expectedOVAs []string
		expectedOVFs []string
		expectError  bool
	}{
		{
			name: "basic structure",
			setup: func(directory string) {
				os.MkdirAll(filepath.Join(directory, "subdir1", "subdir2"), 0755)
				os.WriteFile(filepath.Join(directory, "test.ova"), []byte{}, 0644)
				os.WriteFile(filepath.Join(directory, "test.ovf"), []byte{}, 0644)
				os.WriteFile(filepath.Join(directory, "subdir1", "test1.ova"), []byte{}, 0644)
				os.WriteFile(filepath.Join(directory, "subdir1", "test1.ovf"), []byte{}, 0644)
				os.WriteFile(filepath.Join(directory, "subdir1", "subdir2", "test2.ovf"), []byte{}, 0644)
			},
			expectedOVAs: []string{"test.ova", "subdir1/test1.ova"},
			expectedOVFs: []string{"test.ovf", "subdir1/test1.ovf", "subdir1/subdir2/test2.ovf"},
			expectError:  false,
		},
		{
			name: "non-existent directory",
			setup: func(directory string) {
				os.RemoveAll(directory)
			},
			expectedOVAs: nil,
			expectedOVFs: nil,
			expectError:  true,
		},
		{
			name: "non-ova/ovf files",
			setup: func(directory string) {
				os.WriteFile(filepath.Join(directory, "test.txt"), []byte{}, 0644)
			},
			expectedOVAs: nil,
			expectedOVFs: nil,
			expectError:  false,
		},
		{
			name: "incorrect depth ova",
			setup: func(directory string) {
				os.MkdirAll(filepath.Join(directory, "subdir1", "subdir2"), 0755)
				os.WriteFile(filepath.Join(directory, "subdir1", "subdir2", "test3.ova"), []byte{}, 0644)
			},
			expectedOVAs: nil,
			expectedOVFs: nil,
			expectError:  false,
		},
		{
			name: "incorrect depth ovf",
			setup: func(directory string) {
				os.MkdirAll(filepath.Join(directory, "subdir1", "subdir2", "subdir3"), 0755)
				os.WriteFile(filepath.Join(directory, "subdir1", "subdir2", "subdir3", "test3.ovf"), []byte{}, 0644)
			},
			expectedOVAs: nil,
			expectedOVFs: nil,
			expectError:  false,
		},
		{
			name: "folder with extension",
			setup: func(directory string) {
				os.MkdirAll(filepath.Join(directory, "subdir1.ova"), 0755)
				os.MkdirAll(filepath.Join(directory, "subdir2.ovf"), 0755)
			},
			expectedOVAs: nil,
			expectedOVFs: nil,
			expectError:  false,
		},
		{
			name: "files inside folders with extension",
			setup: func(directory string) {
				os.MkdirAll(filepath.Join(directory, "subdir1.ova"), 0755)
				os.MkdirAll(filepath.Join(directory, "subdir2.ovf"), 0755)
				os.WriteFile(filepath.Join(directory, "subdir1.ova", "test.ova"), []byte{}, 0644)
				os.WriteFile(filepath.Join(directory, "subdir2.ovf", "test.ovf"), []byte{}, 0644)
			},
			expectedOVAs: []string{"subdir1.ova/test.ova"},
			expectedOVFs: []string{"subdir2.ovf/test.ovf"},
			expectError:  false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			testDir, err := os.MkdirTemp("", "ova_test")
			g.Expect(err).NotTo(HaveOccurred())

			testCase.setup(testDir)

			for i, relPath := range testCase.expectedOVAs {
				testCase.expectedOVAs[i] = filepath.Join(testDir, relPath)
			}
			for i, relPath := range testCase.expectedOVFs {
				testCase.expectedOVFs[i] = filepath.Join(testDir, relPath)
			}

			ovaFiles, ovfFiles, err := findOVAFiles(testDir)
			if testCase.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ovaFiles).To(ConsistOf(testCase.expectedOVAs))
				g.Expect(ovfFiles).To(ConsistOf(testCase.expectedOVFs))
			}
			os.RemoveAll(testDir)
		})
	}
}
