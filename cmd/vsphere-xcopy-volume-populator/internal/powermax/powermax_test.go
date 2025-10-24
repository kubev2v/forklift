package powermax

import (
	"context"
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

	t.Run("should return error if POWERMAX_PORT_GROUP_ID is not set", func(t *testing.T) {
		os.Setenv("POWERMAX_SYMMETRIX_ID", "123")
		os.Unsetenv("POWERMAX_PORT_GROUP_ID")
		_, err := NewPowermaxClonner("host", "user", "pass", true)
		g.Expect(err).To(gomega.HaveOccurred())
	})

	t.Run("should return a clonner if all env vars are set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		os.Setenv("POWERMAX_SYMMETRIX_ID", "123")
		os.Setenv("POWERMAX_PORT_GROUP_ID", "456")

		mockClient.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Return(nil)
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
		g.Expect(clonner.portGroupID).To(gomega.Equal("456"))
	})
}

func TestEnsureClonnerIgroup(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("should return a mapping context with the port group id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockClient := NewMockPmax(ctrl)

		clonner := PowermaxClonner{
			client:      mockClient,
			symmetrixID: "123",
			portGroupID: "456",
		}

		initiatorGroup := "test-ig"
		clonnerIqn := []string{"iqn.1994-05.com.redhat:rhv-host"}

		mockClient.EXPECT().GetStorageGroup(context.TODO(), "123", initiatorGroup).Return(&v100.StorageGroup{}, nil)
		mockClient.EXPECT().GetHostList(context.TODO(), "123").Return(&v100.HostList{HostIDs: []string{"host1"}}, nil)
		mockClient.EXPECT().GetHostByID(context.TODO(), "123", "host1").Return(&v100.Host{Initiators: []string{"iqn.1994-05.com.redhat:rhv-host"}}, nil)
		mockClient.EXPECT().GetHostGroupByID(context.TODO(), "123", initiatorGroup).Return(&v100.HostGroup{}, nil)

		mappingContext, err := clonner.EnsureClonnerIgroup(initiatorGroup, clonnerIqn)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mappingContext).ToNot(gomega.BeNil())
		g.Expect(mappingContext[portGroupIDKey]).To(gomega.Equal("456"))
	})
}
