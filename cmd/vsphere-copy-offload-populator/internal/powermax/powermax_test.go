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
	"k8s.io/klog/v2"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
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
			log:         klog.Background(),
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
			log:         klog.Background(),
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
			log:         klog.Background(),
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
			log:         klog.Background(),
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
			log:         klog.Background(),
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
			log:         klog.Background(),
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

func TestResolvePortGroupProtocol(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	log := klog.Background()

	t.Run("returns PortGroupProtocol when set (V4)", func(t *testing.T) {
		pg := &v100.PortGroup{PortGroupProtocol: "SCSI_FC", PortGroupType: "SCSI_FC"}
		protocol, err := resolvePortGroupProtocol(pg, log)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(protocol).To(gomega.Equal("SCSI_FC"))
	})

	t.Run("returns iSCSI PortGroupProtocol when set (V4)", func(t *testing.T) {
		pg := &v100.PortGroup{PortGroupProtocol: "iSCSI", PortGroupType: "iSCSI"}
		protocol, err := resolvePortGroupProtocol(pg, log)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(protocol).To(gomega.Equal("iSCSI"))
	})

	t.Run("maps Fibre type to SCSI_FC when PortGroupProtocol is empty (V3)", func(t *testing.T) {
		pg := &v100.PortGroup{PortGroupType: "Fibre"}
		protocol, err := resolvePortGroupProtocol(pg, log)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(protocol).To(gomega.Equal("SCSI_FC"))
	})

	t.Run("maps iSCSI type to iSCSI when PortGroupProtocol is empty (V3)", func(t *testing.T) {
		pg := &v100.PortGroup{PortGroupType: "iSCSI"}
		protocol, err := resolvePortGroupProtocol(pg, log)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(protocol).To(gomega.Equal("iSCSI"))
	})

	t.Run("returns error for unknown type when PortGroupProtocol is empty", func(t *testing.T) {
		pg := &v100.PortGroup{PortGroupType: "SomeNewType"}
		_, err := resolvePortGroupProtocol(pg, log)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("unable to determine port group protocol"))
		g.Expect(err.Error()).To(gomega.ContainSubstring("SomeNewType"))
	})

	t.Run("returns error when both fields are empty", func(t *testing.T) {
		pg := &v100.PortGroup{}
		_, err := resolvePortGroupProtocol(pg, log)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("unable to determine port group protocol"))
	})
}

func TestEnsureClonnerIgroupV3Fallback(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("V3 FC: resolves protocol from type=Fibre when port_group_protocol is absent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroup:   "v3-fc-pg",
			log:         klog.Background(),
		}

		initiatorGroup := "test-ig"
		clonnerIqn := []string{"10000000c9a12345:10000000c9a12346"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", gomock.Not(gomock.Nil())).Return(&v100.StorageGroup{}, nil)
		mockClient.EXPECT().GetPortGroupByID(context.TODO(), "123", "v3-fc-pg").Return(&v100.PortGroup{
			PortGroupType: "Fibre",
		}, nil)
		mockClient.EXPECT().GetInitiatorList(context.TODO(), "123", "10000000c9a12346", false, true).Return(&v100.InitiatorList{
			InitiatorIDs: []string{"FA-2D:0:10000000c9a12346"},
		}, nil)
		mockClient.EXPECT().GetInitiatorByID(context.TODO(), "123", "FA-2D:0:10000000c9a12346").Return(&v100.Initiator{
			InitiatorID: "FA-2D:0:10000000c9a12346",
			Host:        "v3-host",
		}, nil)

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mappingContext).ToNot(gomega.BeNil())
		g.Expect(clonner.hostID).To(gomega.Equal("v3-host"))
	})

	t.Run("V3 iSCSI: resolves protocol from type=iSCSI when port_group_protocol is absent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroup:   "v3-iscsi-pg",
			log:         klog.Background(),
		}

		initiatorGroup := "test-ig"
		clonnerIqn := []string{"iqn.1994-05.com.redhat:v3-host"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", gomock.Not(gomock.Nil())).Return(&v100.StorageGroup{}, nil)
		mockClient.EXPECT().GetPortGroupByID(context.TODO(), "123", "v3-iscsi-pg").Return(&v100.PortGroup{
			PortGroupType: "iSCSI",
		}, nil)
		mockClient.EXPECT().GetInitiatorByID(context.TODO(), "123", "iqn.1994-05.com.redhat:v3-host").Return(&v100.Initiator{
			InitiatorID: "iqn.1994-05.com.redhat:v3-host",
			Host:        "v3-iscsi-host",
		}, nil)

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mappingContext).ToNot(gomega.BeNil())
		g.Expect(clonner.hostID).To(gomega.Equal("v3-iscsi-host"))
	})
}

func TestRetryOnTransient(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	log := klog.Background()
	ctx := context.Background()

	t.Run("should not retry on non-transient errors", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, log, "test", func() error {
			callCount++
			return &v100.Error{HTTPStatusCode: 404, Message: "not found"}
		})
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(callCount).To(gomega.Equal(1))
	})

	t.Run("should retry on 500 and succeed", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, log, "test", func() error {
			callCount++
			if callCount < 2 {
				return &v100.Error{HTTPStatusCode: 500, Message: "auto provisioning operation is already in progress"}
			}
			return nil
		})
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(callCount).To(gomega.Equal(2))
	})

	t.Run("should retry on 503 and succeed", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, log, "test", func() error {
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
		err := retryOnTransient(ctx, log, "test", func() error {
			callCount++
			return fmt.Errorf("some other error")
		})
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("some other error"))
		g.Expect(callCount).To(gomega.Equal(1))
	})

	t.Run("should succeed on first try", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, log, "test", func() error {
			callCount++
			return nil
		})
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(callCount).To(gomega.Equal(1))
	})

	t.Run("should preserve last error when retries exhausted", func(t *testing.T) {
		callCount := 0
		err := retryOnTransient(ctx, log, "test", func() error {
			callCount++
			return &v100.Error{HTTPStatusCode: 503, Message: "Service Unavailable"}
		})
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("Service Unavailable"))
		g.Expect(callCount).To(gomega.Equal(7)) // 7 steps in backoff
	})
}

func TestUnMap(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	lun := populator.LUN{Name: "test-vol", ProviderID: "vol-001", NAA: "naa.abc123"}
	ctx := populator.MappingContext{}

	t.Run("deletes masking view, removes volume, deletes SG, then zeros fields", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "mv-xcopy",
			storageGroupID: "sg-xcopy",
		}

		gomock.InOrder(
			mockClient.EXPECT().DeleteMaskingView(context.TODO(), "sym123", "mv-xcopy").Return(nil),
			mockClient.EXPECT().RemoveVolumesFromStorageGroup(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil, nil),
			mockClient.EXPECT().DeleteStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return(nil),
		)

		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(clonner.maskingViewID).To(gomega.BeEmpty())
		g.Expect(clonner.storageGroupID).To(gomega.BeEmpty())
	})

	t.Run("skips DeleteMaskingView when maskingViewID is empty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "",
			storageGroupID: "sg-xcopy",
		}

		gomock.InOrder(
			mockClient.EXPECT().RemoveVolumesFromStorageGroup(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil, nil),
			mockClient.EXPECT().DeleteStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return(nil),
		)

		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(clonner.storageGroupID).To(gomega.BeEmpty())
	})

	t.Run("DeleteMaskingView failure is blocking: returns error immediately, skips SG cleanup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "mv-xcopy",
			storageGroupID: "sg-xcopy",
		}

		mvErr := fmt.Errorf("masking view delete failed")
		// gomock strict mode: any unexpected SG call fails the test
		mockClient.EXPECT().DeleteMaskingView(context.TODO(), "sym123", "mv-xcopy").Return(mvErr)

		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("masking view delete failed"))
		g.Expect(clonner.maskingViewID).To(gomega.BeEmpty())
		g.Expect(clonner.storageGroupID).To(gomega.BeEmpty())
	})

	t.Run("makes no API calls when both fields are empty (idempotent second call)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "",
			storageGroupID: "",
		}

		// gomock will fail the test if any unexpected call is made
		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
	})

	t.Run("returns error when RemoveVolumesFromStorageGroup fails, still calls DeleteStorageGroup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "mv-xcopy",
			storageGroupID: "sg-xcopy",
		}

		removeErr := fmt.Errorf("remove volume failed")
		gomock.InOrder(
			mockClient.EXPECT().DeleteMaskingView(context.TODO(), "sym123", "mv-xcopy").Return(nil),
			mockClient.EXPECT().RemoveVolumesFromStorageGroup(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil, removeErr),
			mockClient.EXPECT().DeleteStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return(nil),
		)

		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("remove volume failed"))
		g.Expect(clonner.maskingViewID).To(gomega.BeEmpty())
		g.Expect(clonner.storageGroupID).To(gomega.BeEmpty())
	})

	t.Run("returns error when DeleteStorageGroup fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "",
			storageGroupID: "sg-xcopy",
		}

		sgErr := fmt.Errorf("delete storage group failed")
		gomock.InOrder(
			mockClient.EXPECT().RemoveVolumesFromStorageGroup(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil, nil),
			mockClient.EXPECT().DeleteStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return(sgErr),
		)

		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("delete storage group failed"))
		g.Expect(clonner.storageGroupID).To(gomega.BeEmpty())
	})

	t.Run("DeleteMaskingView 404 treated as already deleted, SG cleanup proceeds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "mv-xcopy",
			storageGroupID: "sg-xcopy",
		}

		notFound := &v100.Error{HTTPStatusCode: 404, Message: "not found"}
		gomock.InOrder(
			mockClient.EXPECT().DeleteMaskingView(context.TODO(), "sym123", "mv-xcopy").Return(notFound),
			mockClient.EXPECT().RemoveVolumesFromStorageGroup(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil, nil),
			mockClient.EXPECT().DeleteStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return(nil),
		)

		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(clonner.maskingViewID).To(gomega.BeEmpty())
		g.Expect(clonner.storageGroupID).To(gomega.BeEmpty())
	})

	t.Run("returns both SG errors joined when RemoveVolumes and DeleteSG both fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "mv-xcopy",
			storageGroupID: "sg-xcopy",
		}

		removeErr := fmt.Errorf("remove volume failed")
		sgErr := fmt.Errorf("storage group delete failed")
		gomock.InOrder(
			mockClient.EXPECT().DeleteMaskingView(context.TODO(), "sym123", "mv-xcopy").Return(nil),
			mockClient.EXPECT().RemoveVolumesFromStorageGroup(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil, removeErr),
			mockClient.EXPECT().DeleteStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return(sgErr),
		)

		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("remove volume failed"))
		g.Expect(err.Error()).To(gomega.ContainSubstring("storage group delete failed"))
		g.Expect(clonner.maskingViewID).To(gomega.BeEmpty())
		g.Expect(clonner.storageGroupID).To(gomega.BeEmpty())
	})

	t.Run("only calls DeleteMaskingView when maskingViewID is set but storageGroupID is empty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			maskingViewID:  "mv-xcopy",
			storageGroupID: "",
		}

		mockClient.EXPECT().DeleteMaskingView(context.TODO(), "sym123", "mv-xcopy").Return(nil)

		err := clonner.UnMap("", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(clonner.maskingViewID).To(gomega.BeEmpty())
	})
}

func TestMap(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	lun := populator.LUN{Name: "test-vol", ProviderID: "vol-001", NAA: "naa.abc123"}
	ctx := populator.MappingContext{}

	t.Run("is a no-op when storageGroupID is empty (remap guard after cleanup)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "",
		}

		// gomock will fail the test if any API call is made
		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(result).To(gomega.Equal(lun))
	})

	t.Run("returns error when GetVolumeIDListInStorageGroup fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
		}

		listErr := fmt.Errorf("list volumes failed")
		mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return(nil, listErr)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("list volumes failed"))
		g.Expect(result).To(gomega.Equal(lun))
	})

	t.Run("returns lun unchanged when volume already in storage group", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
		}

		mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return([]string{"vol-001", "vol-002"}, nil)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(result).To(gomega.Equal(lun))
	})

	t.Run("returns error when AddVolumesToStorageGroupS fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
		}

		addErr := fmt.Errorf("add volumes failed")
		gomock.InOrder(
			mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return([]string{}, nil),
			mockClient.EXPECT().AddVolumesToStorageGroupS(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(addErr),
		)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("add volumes failed"))
		g.Expect(result).To(gomega.Equal(lun))
	})

	t.Run("returns error when GetMaskingViewByID returns non-404 error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
		}

		mvErr := &v100.Error{HTTPStatusCode: 400, Message: "bad request"}
		gomock.InOrder(
			mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return([]string{}, nil),
			mockClient.EXPECT().AddVolumesToStorageGroupS(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil),
			mockClient.EXPECT().GetMaskingViewByID(context.TODO(), "sym123", "mv-xcopy").Return(nil, mvErr),
		)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(result).To(gomega.Equal(populator.LUN{}))
	})

	t.Run("happy path: masking view already exists, sets maskingViewID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
		}

		existingMV := &v100.MaskingView{MaskingViewID: "mv-xcopy"}
		gomock.InOrder(
			mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return([]string{}, nil),
			mockClient.EXPECT().AddVolumesToStorageGroupS(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil),
			mockClient.EXPECT().GetMaskingViewByID(context.TODO(), "sym123", "mv-xcopy").Return(existingMV, nil),
		)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(result).To(gomega.Equal(lun))
		g.Expect(clonner.maskingViewID).To(gomega.Equal("mv-xcopy"))
	})

	t.Run("happy path: masking view not found (404), creates new one", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
			hostID:         "host1",
			portGroup:      "pg1",
		}

		notFoundErr := &v100.Error{HTTPStatusCode: 404, Message: "not found"}
		createdMV := &v100.MaskingView{MaskingViewID: "mv-xcopy"}
		gomock.InOrder(
			mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return([]string{}, nil),
			mockClient.EXPECT().AddVolumesToStorageGroupS(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil),
			mockClient.EXPECT().GetMaskingViewByID(context.TODO(), "sym123", "mv-xcopy").Return(nil, notFoundErr),
			mockClient.EXPECT().CreateMaskingView(context.TODO(), "sym123", "mv-xcopy", "sg-xcopy", "host1", false, "pg1").Return(createdMV, nil),
		)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(result).To(gomega.Equal(lun))
		g.Expect(clonner.maskingViewID).To(gomega.Equal("mv-xcopy"))
	})

	t.Run("returns error when CreateMaskingView fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
			hostID:         "host1",
			portGroup:      "pg1",
		}

		notFoundErr := &v100.Error{HTTPStatusCode: 404, Message: "not found"}
		createErr := fmt.Errorf("create masking view failed")
		gomock.InOrder(
			mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return([]string{}, nil),
			mockClient.EXPECT().AddVolumesToStorageGroupS(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil),
			mockClient.EXPECT().GetMaskingViewByID(context.TODO(), "sym123", "mv-xcopy").Return(nil, notFoundErr),
			mockClient.EXPECT().CreateMaskingView(context.TODO(), "sym123", "mv-xcopy", "sg-xcopy", "host1", false, "pg1").Return(nil, createErr),
		)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("create masking view failed"))
		g.Expect(result).To(gomega.Equal(populator.LUN{}))
	})

	t.Run("409 idempotent path: CreateMaskingView returns nil mv, fetches it on retry", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
			hostID:         "host1",
			portGroup:      "pg1",
		}

		notFoundErr := &v100.Error{HTTPStatusCode: 404, Message: "not found"}
		fetchedMV := &v100.MaskingView{MaskingViewID: "mv-xcopy"}
		gomock.InOrder(
			mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return([]string{}, nil),
			mockClient.EXPECT().AddVolumesToStorageGroupS(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil),
			// First GetMaskingViewByID: 404, triggers create path
			mockClient.EXPECT().GetMaskingViewByID(context.TODO(), "sym123", "mv-xcopy").Return(nil, notFoundErr),
			// CreateMaskingView returns nil mv (409 idempotent)
			mockClient.EXPECT().CreateMaskingView(context.TODO(), "sym123", "mv-xcopy", "sg-xcopy", "host1", false, "pg1").Return(nil, nil),
			// Second GetMaskingViewByID: fetches the existing one
			mockClient.EXPECT().GetMaskingViewByID(context.TODO(), "sym123", "mv-xcopy").Return(fetchedMV, nil),
		)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(result).To(gomega.Equal(lun))
		g.Expect(clonner.maskingViewID).To(gomega.Equal("mv-xcopy"))
	})

	t.Run("409 idempotent path: second GetMaskingViewByID fails after nil CreateMaskingView", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:         mockClient,
			symmetrixID:    "sym123",
			log:            klog.Background(),
			storageGroupID: "sg-xcopy",
			initiatorID:    "mv-xcopy",
			hostID:         "host1",
			portGroup:      "pg1",
		}

		notFoundErr := &v100.Error{HTTPStatusCode: 404, Message: "not found"}
		fetchErr := fmt.Errorf("fetch after 409 failed")
		gomock.InOrder(
			mockClient.EXPECT().GetVolumeIDListInStorageGroup(context.TODO(), "sym123", "sg-xcopy").Return([]string{}, nil),
			mockClient.EXPECT().AddVolumesToStorageGroupS(context.TODO(), "sym123", "sg-xcopy", false, "vol-001").Return(nil),
			mockClient.EXPECT().GetMaskingViewByID(context.TODO(), "sym123", "mv-xcopy").Return(nil, notFoundErr),
			mockClient.EXPECT().CreateMaskingView(context.TODO(), "sym123", "mv-xcopy", "sg-xcopy", "host1", false, "pg1").Return(nil, nil),
			mockClient.EXPECT().GetMaskingViewByID(context.TODO(), "sym123", "mv-xcopy").Return(nil, fetchErr),
		)

		result, err := clonner.Map("ignored-group", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("fetch after 409 failed"))
		g.Expect(result).To(gomega.Equal(populator.LUN{}))
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
