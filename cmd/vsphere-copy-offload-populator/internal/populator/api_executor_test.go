package populator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("toDatastorePath", func() {
	DescribeTable("converts a vmkfstools path to the datastore path the vSphere API requires",
		func(datastore, path, expected string) {
			result, err := toDatastorePath(datastore, path)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
		Entry("vm home dir",
			"eco-iscsi-ds1",
			"/vmfs/volumes/eco-iscsi-ds1/aap_test_vm/aap_test_vm.vmdk",
			"[eco-iscsi-ds1] aap_test_vm/aap_test_vm.vmdk"),
		Entry("datastore name containing spaces",
			"my datastore",
			"/vmfs/volumes/my datastore/vm/vm.vmdk",
			"[my datastore] vm/vm.vmdk"),
		Entry("nested path",
			"ds1",
			"/vmfs/volumes/ds1/a/b/c.vmdk",
			"[ds1] a/b/c.vmdk"),
	)

	DescribeTable("rejects paths that are not under the datastore",
		func(datastore, path string) {
			_, err := toDatastorePath(datastore, path)
			Expect(err).To(HaveOccurred())
		},
		Entry("different datastore", "ds1", "/vmfs/volumes/ds2/vm/vm.vmdk"),
		Entry("datastore root with no file", "ds1", "/vmfs/volumes/ds1/"),
		Entry("already a datastore path", "ds1", "[ds1] vm/vm.vmdk"),
		Entry("empty path", "ds1", ""),
	)
})
