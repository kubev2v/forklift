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
	"context"
	"net"
	"testing"

	cnitypes "github.com/containernetworking/cni/pkg/types"
	cnicurrent "github.com/containernetworking/cni/pkg/types/current"

	v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// EnsureCIDR parses/verify CIDR ip string and convert to net.IPNet
func EnsureCIDR(cidr string) *net.IPNet {
    ip, net, err := net.ParseCIDR(cidr)
    Expect(err).NotTo(HaveOccurred())
    net.IP = ip
    return net
}

func TestNetworkAttachmentDefinition(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Attachment Definition utils")
}

var _ = Describe("Netwok Attachment Definition manipulations", func() {

	It("test convertDNS", func() {
		cniDNS := cnitypes.DNS{
			Nameservers: []string{ "aaa", "bbb" },
			Domain: "testDomain",
			Search: []string{ "1.example.com", "2.example.com" },
			Options: []string{ "debug", "inet6" },
		}

		v1DNS := convertDNS(cniDNS)
		Expect(v1DNS.Nameservers).To(Equal(cniDNS.Nameservers))
		Expect(v1DNS.Domain).To(Equal(cniDNS.Domain))
		Expect(v1DNS.Search).To(Equal(cniDNS.Search))
		Expect(v1DNS.Options).To(Equal(cniDNS.Options))
	})

	It("set network status into pod", func() {
		fakePod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fakePod1",
				Namespace: "fakeNamespace1",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "fakeContainer",
						Image: "fakeImage",
					},
				},
			},
		}
		fakeStatus := []v1.NetworkStatus{
			v1.NetworkStatus{
				Name: "cbr0",
				Interface: "eth0",
				IPs: []string{ "10.244.1.2" },
				Mac: "92:79:27:01:7c:ce",
			},
			v1.NetworkStatus{
				Name: "test-net-attach-def-1",
				Interface: "net1",
				IPs: []string{ "1.1.1.1" },
				Mac: "ea:0e:fa:63:95:f9",
			},
		}

		clientSet := fake.NewSimpleClientset(fakePod)
		pod, err:= clientSet.CoreV1().Pods("fakeNamespace1").Get(context.TODO(), "fakePod1", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		err = SetNetworkStatus(clientSet, pod, fakeStatus)
		Expect(err).NotTo(HaveOccurred())

		getStatuses, err := GetNetworkStatus(pod)
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeStatus).To(Equal(getStatuses))
	})

	It("create network status from cni result", func() {
		cniResult := &cnicurrent.Result{
			CNIVersion: "0.3.1",
			Interfaces: []*cnicurrent.Interface{
				&cnicurrent.Interface{
					Name: "net1",
					Mac: "92:79:27:01:7c:cf",
					Sandbox: "/proc/1123/ns/net",
				},
			},
			IPs: []*cnicurrent.IPConfig{
				{
					Version: "4",
					Address: *EnsureCIDR("1.1.1.3/24"),
				},
				{
					Version: "6",
					Address: *EnsureCIDR("2001::1/64"),
				},
			},
		}
		status, err := CreateNetworkStatus(cniResult, "test-net-attach-def", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(status.Name).To(Equal("test-net-attach-def"))
		Expect(status.Interface).To(Equal("net1"))
		Expect(status.Mac).To(Equal("92:79:27:01:7c:cf"))
		Expect(status.IPs).To(Equal([]string{ "1.1.1.3", "2001::1" }))
	})

	It("parse network selection element in pod", func() {
		selectionElement := `
		[{
			"name": "test-net-attach-def",
			"interface": "test1"
		}]`
		expectedElement := []*v1.NetworkSelectionElement{
			&v1.NetworkSelectionElement{
				Name: "test-net-attach-def",
				InterfaceRequest: "test1",
				Namespace: "fakeNamespace1",
			},
		}

		fakePod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fakePod1",
				Namespace: "fakeNamespace1",
				Annotations: map[string]string{
					"k8s.v1.cni.cncf.io/networks": selectionElement,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "fakeContainer",
						Image: "fakeImage",
					},
				},
			},
		}
		elem, err := ParsePodNetworkAnnotation(fakePod)
		Expect(err).NotTo(HaveOccurred())
		Expect(elem).To(Equal(expectedElement))
	})
})
