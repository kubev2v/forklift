package vsphere

import (
	"context"
	"fmt"
	"testing"
	"time"

	vmware_mocks "github.com/kubev2v/forklift/pkg/lib/client/vsphere/vmware/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/cli/esx"
	"go.uber.org/mock/gomock"
)

func TestVSphere(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VSphere Client Suite")
}

var _ = Describe("ShouldSkipVIBCheck", func() {
	It("should not skip when lastTransitionTime is zero", func() {
		Expect(ShouldSkipVIBCheck(time.Time{}, 15*time.Minute)).To(BeFalse())
	})

	It("should skip when within cache duration", func() {
		recent := time.Now().Add(-5 * time.Minute)
		Expect(ShouldSkipVIBCheck(recent, 15*time.Minute)).To(BeTrue())
	})

	It("should not skip when cache duration has elapsed", func() {
		old := time.Now().Add(-20 * time.Minute)
		Expect(ShouldSkipVIBCheck(old, 15*time.Minute)).To(BeFalse())
	})

	It("should not skip when exactly at cache boundary", func() {
		boundary := time.Now().Add(-15 * time.Minute)
		Expect(ShouldSkipVIBCheck(boundary, 15*time.Minute)).To(BeFalse())
	})

	It("should handle zero cache duration", func() {
		recent := time.Now().Add(-1 * time.Second)
		Expect(ShouldSkipVIBCheck(recent, 0)).To(BeFalse())
	})
})

var _ = Describe("GetVIBVersion", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *vmware_mocks.MockClient
		ctx        context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = vmware_mocks.NewMockClient(ctrl)
		ctx = context.Background()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should return the version from esxcli response", func() {
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"software", "vib", "get", "-n", vibName}).
			Return([]esx.Values{{"Version": {"2.0.1"}}}, nil)

		version, err := GetVIBVersion(ctx, mockClient, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(Equal("2.0.1"))
	})

	It("should return empty string when VIB is not installed (NoMatchError)", func() {
		faultErr := &esx.Fault{
			Detail: `<obj xmlns="urn:vim25" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><fault xsi:type="NoMatchError"><errMsg>[NoMatchError] No matching VIBs</errMsg></fault></obj>`,
		}
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"software", "vib", "get", "-n", vibName}).
			Return(nil, faultErr)

		version, err := GetVIBVersion(ctx, mockClient, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(BeEmpty())
	})

	It("should propagate non-fault errors", func() {
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"software", "vib", "get", "-n", vibName}).
			Return(nil, fmt.Errorf("connection refused"))

		_, err := GetVIBVersion(ctx, mockClient, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connection refused"))
	})
})

var _ = Describe("GetLoadedVIBVersion", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *vmware_mocks.MockClient
		ctx        context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = vmware_mocks.NewMockClient(ctrl)
		ctx = context.Background()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should parse version from valid JSON response", func() {
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"vmkfstools", "version"}).
			Return([]esx.Values{{"message": {`{"version": "3.1.0"}`}, "status": {"0"}}}, nil)

		version, err := GetLoadedVIBVersion(ctx, mockClient, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(Equal("3.1.0"))
	})

	It("should return error on invalid JSON", func() {
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"vmkfstools", "version"}).
			Return([]esx.Values{{"message": {"not-json"}, "status": {"0"}}}, nil)

		_, err := GetLoadedVIBVersion(ctx, mockClient, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to parse VIB version response"))
	})

	It("should return error on empty message field", func() {
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"vmkfstools", "version"}).
			Return([]esx.Values{{"message": {""}}}, nil)

		_, err := GetLoadedVIBVersion(ctx, mockClient, nil)
		Expect(err).To(HaveOccurred())
	})

	It("should propagate RunEsxCommand errors", func() {
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"vmkfstools", "version"}).
			Return(nil, fmt.Errorf("host not reachable"))

		_, err := GetLoadedVIBVersion(ctx, mockClient, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("host not reachable"))
	})
})

var _ = Describe("DefaultVIBInstaller", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *vmware_mocks.MockClient
		ctx        context.Context
		installer  *DefaultVIBInstaller
		origVer    string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = vmware_mocks.NewMockClient(ctrl)
		ctx = context.Background()
		installer = &DefaultVIBInstaller{}
		origVer = VibVersion
		VibVersion = "1.0.0"
	})

	AfterEach(func() {
		VibVersion = origVer
		ctrl.Finish()
	})

	It("should skip install when loaded version matches desired", func() {
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"vmkfstools", "version"}).
			Return([]esx.Values{{"message": {`{"version": "1.0.0"}`}}}, nil)

		err := installer.InstallVib(ctx, mockClient, nil, "/path/to/vib")
		Expect(err).NotTo(HaveOccurred())
	})

	It("should skip install when on-disk version matches desired", func() {
		// Loaded version check fails
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"vmkfstools", "version"}).
			Return(nil, fmt.Errorf("not available"))

		// On-disk version matches
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"software", "vib", "get", "-n", vibName}).
			Return([]esx.Values{{"Version": {"1.0.0"}}}, nil)

		err := installer.InstallVib(ctx, mockClient, nil, "/path/to/vib")
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return error when on-disk version check fails after loaded check fails", func() {
		mockClient.EXPECT().
			RunEsxCommand(ctx, nil, []string{"software", "vib", "get", "-n", vibName}).
			Return(nil, fmt.Errorf("esxcli connection failed"))

		_, err := GetVIBVersion(ctx, mockClient, nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("esxcli connection failed"))
	})
})
