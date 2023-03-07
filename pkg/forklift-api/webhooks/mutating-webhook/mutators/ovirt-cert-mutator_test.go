package mutators

import (
	"io/ioutil"
	"testing"

	. "github.com/onsi/gomega"
)

func TestCertAppending(t *testing.T) {
	g := NewGomegaWithT(t)

	providedCa, err := ioutil.ReadFile("completeCerts.pem")
	g.Expect(err).ToNot(HaveOccurred())

	newCa, err := ioutil.ReadFile("engineCert.pem")
	g.Expect(err).ToNot(HaveOccurred())

	//Test the case where two certificates are identical but have a different line count due to new lines.
	g.Expect(contains(providedCa, newCa)).To(BeTrue())

	//Test the case where the original certificate does not have a new line at the end.
	g.Expect(appendCerts(newCa, newCa)).To(ContainSubstring("-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----"))

	//Test the case where the original certificate has a new line at the end and verify a redundant new line was not added.
	newCa = append(newCa, 0x0a)
	g.Expect(appendCerts(newCa, newCa)).To(ContainSubstring("-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----"))

	//Test the case when the certificate is changed by one byte to verify that "contains" returns false.
	newCa = append(newCa, 0x01)
	g.Expect(contains(providedCa, newCa)).To(BeFalse())
}
