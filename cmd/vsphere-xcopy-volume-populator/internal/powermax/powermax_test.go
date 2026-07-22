package powermax

import (
	"context"
	"fmt"
	"os"
	"testing"

	gopowermax "github.com/dell/gopowermax/v2"
	"github.com/dell/gopowermax/v2/types/v100"
	"github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestNewPowermaxClonner(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("should return error if POWERMAX_SYMMETRIX_ID is not set", func(t *testing.T) {
		os.Unsetenv("POWERMAX_SYMMETRIX_ID")
		_, err := NewPowermaxClonner("host", "user", "pass", true)
		g.Expect(err).To(gomega.HaveOccurred())
	})

	t.Run("should return error if POWERMAX_PORT_GROUP_NAME is not set", func(t *testing.T) {
		os.Setenv("POWERMAX_SYMMETRIX_ID", "123")
		os.Unsetenv("POWERMAX_PORT_GROUP_NAME")
		_, err := NewPowermaxClonner("host", "user", "pass", true)
		g.Expect(err).To(gomega.HaveOccurred())
	})

	t.Run("should return a clonner if all env vars are set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		os.Setenv("POWERMAX_SYMMETRIX_ID", "123")
		os.Setenv("POWERMAX_PORT_GROUP_NAME", "456")

		mockClient.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
		mockClient.EXPECT().GetSymmetrixByID(gomock.Any(), "123").Return(&v100.Symmetrix{
			SymmetrixID: "123",
			Model:       "PowerMax_2500",
			Ucode:       "6079.170.170",
		}, nil)
		// not testing the gopowermax constructor
		origNewClientWithArgs := newClientWithArgs
		newClientWithArgs = func(string, string, bool, bool, string) (gopowermax.Pmax, error) {
			return mockClient, nil
		}
		defer func() { newClientWithArgs = origNewClientWithArgs }()

		clonner, err := NewPowermaxClonner("host", "user", "pass", true)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(clonner).ToNot(gomega.BeNil())
		g.Expect(clonner.symmetrixID).To(gomega.Equal("123"))
		g.Expect(clonner.portGroup).To(gomega.Equal("456"))
		g.Expect(clonner.arrayInfo.Vendor).To(gomega.Equal("Dell"))
		g.Expect(clonner.arrayInfo.Product).To(gomega.Equal("PowerMax"))
		g.Expect(clonner.arrayInfo.Model).To(gomega.Equal("PowerMax_2500"))
		g.Expect(clonner.arrayInfo.Version).To(gomega.Equal("6079.170.170"))
	})
}

func TestEnsureClonnerIgroup(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("should find host via direct initiator lookup (iSCSI)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroup:   "456",
		}

		initiatorGroup := "test-ig"
		clonnerIqn := []string{"iqn.1994-05.com.redhat:rhv-host"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", gomock.Not(gomock.Nil())).Return(&v100.StorageGroup{}, nil)
		mockClient.EXPECT().GetPortGroupByID(context.TODO(), "123", "456").Return(&v100.PortGroup{PortGroupProtocol: "iSCSI"}, nil)
		// Direct initiator lookup succeeds and returns the host
		mockClient.EXPECT().GetInitiatorByID(context.TODO(), "123", "iqn.1994-05.com.redhat:rhv-host").Return(&v100.Initiator{
			InitiatorID: "iqn.1994-05.com.redhat:rhv-host",
			Host:        "host1",
		}, nil)

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mappingContext).ToNot(gomega.BeNil())
		g.Expect(clonner.hostID).To(gomega.Equal("host1"))
	})

	t.Run("should fail when initiator lookup returns 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroup:   "456",
		}

		initiatorGroup := "test-ig"
		clonnerIqn := []string{"iqn.1994-05.com.redhat:rhv-host"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", gomock.Not(gomock.Nil())).Return(&v100.StorageGroup{}, nil)
		mockClient.EXPECT().GetPortGroupByID(context.TODO(), "123", "456").Return(&v100.PortGroup{PortGroupProtocol: "iSCSI"}, nil)
		mockClient.EXPECT().GetInitiatorByID(context.TODO(), "123", "iqn.1994-05.com.redhat:rhv-host").Return(nil, &v100.Error{HTTPStatusCode: 404, Message: "not found"})

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("failed to look up initiator"))
		g.Expect(mappingContext).To(gomega.BeNil())
	})

	t.Run("should fail when port group protocol is SCSI_FC but only iSCSI initiators are provided", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroup:   "fc-port-group",
		}

		initiatorGroup := "test-ig"
		// Only iSCSI initiators provided
		clonnerIqn := []string{"iqn.1994-05.com.redhat:rhv-host"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", gomock.Not(gomock.Nil())).Return(&v100.StorageGroup{}, nil)
		// Port group is configured for Fibre Channel
		mockClient.EXPECT().GetPortGroupByID(context.TODO(), "123", "fc-port-group").Return(&v100.PortGroup{PortGroupProtocol: "SCSI_FC"}, nil)

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("no initiators matching protocol SCSI_FC"))
		g.Expect(mappingContext).To(gomega.BeNil())
	})

	t.Run("should find host via direct initiator lookup (FC)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroup:   "fc-port-group",
		}

		initiatorGroup := "test-ig"
		// FC initiators in WWNN:WWPN format
		clonnerIqn := []string{"10000000c9a12345:10000000c9a12346"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", gomock.Not(gomock.Nil())).Return(&v100.StorageGroup{}, nil)
		mockClient.EXPECT().GetPortGroupByID(context.TODO(), "123", "fc-port-group").Return(&v100.PortGroup{PortGroupProtocol: "SCSI_FC"}, nil)
		// FC lookup: first list by WWPN, then get by PowerMax initiator ID
		mockClient.EXPECT().GetInitiatorList(context.TODO(), "123", "10000000c9a12346", false, true).Return(&v100.InitiatorList{
			InitiatorIDs: []string{"OR-2C:0:10000000c9a12346"},
		}, nil)
		mockClient.EXPECT().GetInitiatorByID(context.TODO(), "123", "OR-2C:0:10000000c9a12346").Return(&v100.Initiator{
			InitiatorID: "OR-2C:0:10000000c9a12346",
			Host:        "host1",
		}, nil)

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mappingContext).ToNot(gomega.BeNil())
		g.Expect(clonner.hostID).To(gomega.Equal("host1"))
	})

	t.Run("should strip fc. prefix for direct initiator lookup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroup:   "fc-port-group",
		}

		initiatorGroup := "test-ig"
		// FC initiators with fc. prefix (as seen from ESXi)
		clonnerIqn := []string{"fc.200000109b985703:100000109b985703"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", gomock.Not(gomock.Nil())).Return(&v100.StorageGroup{}, nil)
		mockClient.EXPECT().GetPortGroupByID(context.TODO(), "123", "fc-port-group").Return(&v100.PortGroup{PortGroupProtocol: "SCSI_FC"}, nil)
		// FC lookup: strip fc. prefix, extract WWPN, list by WWPN, then get by PowerMax ID
		mockClient.EXPECT().GetInitiatorList(context.TODO(), "123", "100000109b985703", false, true).Return(&v100.InitiatorList{
			InitiatorIDs: []string{"FA-2D:1:100000109b985703"},
		}, nil)
		mockClient.EXPECT().GetInitiatorByID(context.TODO(), "123", "FA-2D:1:100000109b985703").Return(&v100.Initiator{
			InitiatorID: "FA-2D:1:100000109b985703",
			Host:        "esx-host-42",
		}, nil)

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mappingContext).ToNot(gomega.BeNil())
		g.Expect(clonner.hostID).To(gomega.Equal("esx-host-42"))
	})

	t.Run("should retry on transient 503 error during direct initiator lookup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroup:   "456",
		}

		initiatorGroup := "test-ig"
		clonnerIqn := []string{"iqn.1994-05.com.redhat:rhv-host"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", gomock.Not(gomock.Nil())).Return(&v100.StorageGroup{}, nil)
		mockClient.EXPECT().GetPortGroupByID(context.TODO(), "123", "456").Return(&v100.PortGroup{PortGroupProtocol: "iSCSI"}, nil)
		// First call returns 503, second succeeds
		gomock.InOrder(
			mockClient.EXPECT().GetInitiatorByID(context.TODO(), "123", "iqn.1994-05.com.redhat:rhv-host").Return(nil, &v100.Error{HTTPStatusCode: 503, Message: "Service Unavailable"}),
			mockClient.EXPECT().GetInitiatorByID(context.TODO(), "123", "iqn.1994-05.com.redhat:rhv-host").Return(&v100.Initiator{
				InitiatorID: "iqn.1994-05.com.redhat:rhv-host",
				Host:        "host1",
			}, nil),
		)

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mappingContext).ToNot(gomega.BeNil())
		g.Expect(clonner.hostID).To(gomega.Equal("host1"))
	})
}

func TestRetryOnTransient(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ctx := context.Background()

	t.Run("should not retry on non-503 errors", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, "test", func() error {
			callCount++
			return &v100.Error{HTTPStatusCode: 404, Message: "not found"}
		})
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(callCount).To(gomega.Equal(1))
	})

	t.Run("should retry on 503 and succeed", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, "test", func() error {
			callCount++
			if callCount < 3 {
				return &v100.Error{HTTPStatusCode: 503, Message: "Service Unavailable"}
			}
			return nil
		})
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(callCount).To(gomega.Equal(3))
	})

	t.Run("should pass through non-pmxtypes errors", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, "test", func() error {
			callCount++
			return fmt.Errorf("some other error")
		})
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("some other error"))
		g.Expect(callCount).To(gomega.Equal(1))
	})

	t.Run("should succeed on first try", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, "test", func() error {
			callCount++
			return nil
		})
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(callCount).To(gomega.Equal(1))
	})

	t.Run("should preserve last error when retries exhausted", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, "test", func() error {
			callCount++
			return &v100.Error{HTTPStatusCode: 503, Message: "Service Unavailable"}
		})
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("Service Unavailable"))
		g.Expect(callCount).To(gomega.Equal(5)) // 5 steps in backoff
	})
}

func TestInitiatorToLookupID(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	g.Expect(initiatorToLookupID("fc.200000109b985703:100000109b985703")).To(gomega.Equal("200000109b985703:100000109b985703"))
	g.Expect(initiatorToLookupID("iqn.1994-05.com.redhat:rhv-host")).To(gomega.Equal("iqn.1994-05.com.redhat:rhv-host"))
	g.Expect(initiatorToLookupID("10000000c9a12345:10000000c9a12346")).To(gomega.Equal("10000000c9a12345:10000000c9a12346"))
}

func TestExtractWWPN(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	g.Expect(extractWWPN("200000109b985703:100000109b985703")).To(gomega.Equal("100000109b985703"))
	g.Expect(extractWWPN("10000000c9a12345:10000000c9a12346")).To(gomega.Equal("10000000c9a12346"))
	g.Expect(extractWWPN("100000109b985703")).To(gomega.Equal("100000109b985703"))
}
