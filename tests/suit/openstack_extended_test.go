//nolint:errcheck
package suit_test

import (
	"github.com/kubev2v/forklift/pkg/lib/client/openstack"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/tests/suit/framework"
	"github.com/kubev2v/forklift/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	keystoneSecureURL = "https://packstack.konveyor-forklift:30051/v3"
)

var _ = Describe("[level:component]Migration Extended tests for OpenStack provider", func() {
	f := framework.NewFramework("migration-func-test")

	It("[extended] should connect to openstack using https/ssl with CA", func() {
		namespace := f.Namespace.Name

		err := f.Clients.OpenStackClient.SetupClient("cirros-volume", "net-int", "nfs")
		Expect(err).ToNot(HaveOccurred())

		By("Load Source VM Details from OpenStack")

		packstackCA, err := f.Clients.OpenStackClient.LoadCA(f, packstackNameSpace, "packstack")
		Expect(err).ToNot(HaveOccurred())

		err = utils.TestHttpsCA(keystoneSecureURL, packstackCA, false)
		Expect(err).ToNot(HaveOccurred())

		By("Create Secret from Definition")
		clientOpts := map[string]string{
			"username":    "admin",
			"password":    "12e2f14739194a6c",
			"domainName":  "default",
			"projectName": "admin",
			"regionName":  "RegionOne",
			"cacert":      packstackCA,
		}
		_, err = utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, clientOpts, nil, namespace, "os-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		client := openstack.Client{
			URL:     keystoneSecureURL,
			Options: clientOpts,
			Log:     logging.WithName("test"),
		}
		err = client.Connect()
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test] should connect to openstack using https/ssl insecure", func() {
		namespace := f.Namespace.Name
		err := f.Clients.OpenStackClient.SetupClient("cirros-volume", "net-int", "nfs")
		Expect(err).ToNot(HaveOccurred())

		By("Create Secret from Definition")
		clientOpts := map[string]string{
			"username":           "admin",
			"password":           "12e2f14739194a6c",
			"domainName":         "default",
			"projectName":        "admin",
			"regionName":         "RegionOne",
			"insecureSkipVerify": "true",
			"cacert":             "",
		}
		_, err = utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, clientOpts, nil, namespace, "os-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		client := openstack.Client{
			URL:     keystoneSecureURL,
			Options: clientOpts,
			Log:     logging.WithName("test"),
		}
		err = client.Connect()
		Expect(err).ToNot(HaveOccurred())
	})

	It("[test] should connect to openstack using https/ssl with system CA", func() {
		namespace := f.Namespace.Name

		err := f.Clients.OpenStackClient.SetupClient("cirros-volume", "net-int", "nfs")
		Expect(err).ToNot(HaveOccurred())

		By("Load Source VM Details from OpenStack")

		packstackCA, err := f.Clients.OpenStackClient.LoadCA(f, packstackNameSpace, "packstack")
		Expect(err).ToNot(HaveOccurred())

		err = utils.UpdateLocalCA(packstackCA)
		Expect(err).ToNot(HaveOccurred())

		err = utils.TestHttpsCA(keystoneSecureURL, packstackCA, false)
		Expect(err).ToNot(HaveOccurred())

		By("Create Secret from Definition")
		clientOpts := map[string]string{
			"username":    "admin",
			"password":    "12e2f14739194a6c",
			"domainName":  "default",
			"projectName": "admin",
			"regionName":  "RegionOne",
			"cacert":      "",
		}
		_, err = utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, clientOpts, nil, namespace, "os-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		client := openstack.Client{
			URL:     keystoneSecureURL,
			Options: clientOpts,
			Log:     logging.WithName("test"),
		}
		err = client.Connect()
		Expect(err).ToNot(HaveOccurred())

		utils.RemoveLocalCA()
	})

	It("[test] should not connect with invalid CA and not fallback to system", func() {
		namespace := f.Namespace.Name

		err := f.Clients.OpenStackClient.SetupClient("cirros-volume", "net-int", "nfs")
		Expect(err).ToNot(HaveOccurred())

		By("Load Source VM Details from OpenStack")

		packstackCA, err := f.Clients.OpenStackClient.LoadCA(f, packstackNameSpace, "packstack")
		Expect(err).ToNot(HaveOccurred())

		err = utils.UpdateLocalCA(packstackCA)
		Expect(err).ToNot(HaveOccurred())

		err = utils.TestHttpsCA(keystoneSecureURL, packstackCA, false)
		Expect(err).ToNot(HaveOccurred())

		By("Create Secret from Definition")
		clientOpts := map[string]string{
			"username":    "admin",
			"password":    "12e2f14739194a6c",
			"domainName":  "default",
			"projectName": "admin",
			"regionName":  "RegionOne",
			"cacert":      packstackCA + "bad",
		}
		_, err = utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, clientOpts, nil, namespace, "os-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		client := openstack.Client{
			URL:     keystoneSecureURL,
			Options: clientOpts,
			Log:     logging.WithName("test"),
		}
		err = client.Connect()
		Expect(err).To(HaveOccurred())

		utils.RemoveLocalCA()
	})

})
