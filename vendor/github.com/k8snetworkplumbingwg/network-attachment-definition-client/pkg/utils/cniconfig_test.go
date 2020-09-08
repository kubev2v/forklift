// Copyright (c) 2019 Kubernetes Network Plumbing Working Group
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

package utils

import (
	v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCNIConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CNI Config")
}

var _ = Describe("CNI config manipulations", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "multus-tmp")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tmpDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Valid case", func() {
		It("test net-attach-def with valid config", func() {
			cniConfig := `
		{
			"type": "test",
			"name": "testname",
			"version": "0.3.1"
		}
		`
			netattachdef := v1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testnamespace",
				},
				Spec: v1.NetworkAttachmentDefinitionSpec{
					Config: cniConfig,
				},
			}
			config, err := GetCNIConfig(&netattachdef, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(config).To(Equal([]byte(cniConfig)))
		})

		It("test net-attach-def with conf file", func() {
			tmpConfFilePath := filepath.Join(tmpDir, "testCNI.conf")
			cniConfig := `{
			"type": "test",
			"name": "test-net-attach-def",
			"version": "0.3.1"
		}`
			ioutil.WriteFile(tmpConfFilePath, []byte(cniConfig), 0644)

			netattachdef := v1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-net-attach-def",
					Namespace: "testnamespace",
				},
			}
			config, err := GetCNIConfig(&netattachdef, tmpDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).To(Equal([]byte(cniConfig)))
		})

		It("test net-attach-def with conflist file", func() {
			tmpConfFilePath := filepath.Join(tmpDir, "testCNI.conflist")
			cniConfig := `{
			"name": "test-net-attach-def",
			"version": "0.3.1",
			"plugins": [
			{
				"type": "test"
			}]
		}`
			ioutil.WriteFile(tmpConfFilePath, []byte(cniConfig), 0644)

			netattachdef := v1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-net-attach-def",
					Namespace: "testnamespace",
				},
			}
			config, err := GetCNIConfig(&netattachdef, tmpDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).To(Equal([]byte(cniConfig)))
		})
	})

	Context("Invalid case", func() {
		It("test net-attach-def with invalid config", func() {
			cniConfig := `***invalid json file***`
			netattachdef := v1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-net-attach-def",
					Namespace: "testnamespace",
				},
				Spec: v1.NetworkAttachmentDefinitionSpec{
					Config: cniConfig,
				},
			}
			_, err := GetCNIConfig(&netattachdef, "")
			Expect(err).To(HaveOccurred())
		})

		It("test net-attach-def with invalid conf file", func() {
			tmpConfFilePath := filepath.Join(tmpDir, "testCNI.conf")
			cniConfig := `***invalid json file***`
			ioutil.WriteFile(tmpConfFilePath, []byte(cniConfig), 0644)

			netattachdef := v1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-net-attach-def",
					Namespace: "testnamespace",
				},
			}
			_, err := GetCNIConfig(&netattachdef, tmpDir)
			Expect(err).To(HaveOccurred())
		})

		It("test net-attach-def with invalid conflist file", func() {
			tmpConfFilePath := filepath.Join(tmpDir, "testCNI.conflist")
			cniConfig := `***invalid json file***`
			ioutil.WriteFile(tmpConfFilePath, []byte(cniConfig), 0644)

			netattachdef := v1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-net-attach-def",
					Namespace: "testnamespace",
				},
			}
			_, err := GetCNIConfig(&netattachdef, tmpDir)
			Expect(err).To(HaveOccurred())
		})
	})

})
