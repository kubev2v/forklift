package powerstore

import (
	"context"
	"testing"

	"github.com/dell/gopowerstore"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/onsi/gomega"
	"k8s.io/klog/v2"
)

type mockClient struct {
	gopowerstore.Client

	hosts             []gopowerstore.Host
	hostGroups        []gopowerstore.HostGroup
	volumeMappings    []gopowerstore.HostVolumeMapping
	attachedToHost    []string
	attachedToGroup   []string
	detachedFromHost  []string
	detachedFromGroup []string
}

func (m *mockClient) GetHosts(_ context.Context) ([]gopowerstore.Host, error) {
	return m.hosts, nil
}

func (m *mockClient) GetHostGroups(_ context.Context) ([]gopowerstore.HostGroup, error) {
	return m.hostGroups, nil
}

func (m *mockClient) GetHostByName(_ context.Context, name string) (gopowerstore.Host, error) {
	for _, h := range m.hosts {
		if h.Name == name {
			return h, nil
		}
	}
	return gopowerstore.Host{}, gopowerstore.NewHostIsNotExistError()
}

func (m *mockClient) GetHostVolumeMappingByVolumeID(_ context.Context, _ string) ([]gopowerstore.HostVolumeMapping, error) {
	return m.volumeMappings, nil
}

func (m *mockClient) AttachVolumeToHost(_ context.Context, hostID string, _ *gopowerstore.HostVolumeAttach) (gopowerstore.EmptyResponse, error) {
	m.attachedToHost = append(m.attachedToHost, hostID)
	return "", nil
}

func (m *mockClient) AttachVolumeToHostGroup(_ context.Context, groupID string, _ *gopowerstore.HostVolumeAttach) (gopowerstore.EmptyResponse, error) {
	m.attachedToGroup = append(m.attachedToGroup, groupID)
	return "", nil
}

func (m *mockClient) DetachVolumeFromHost(_ context.Context, hostID string, _ *gopowerstore.HostVolumeDetach) (gopowerstore.EmptyResponse, error) {
	m.detachedFromHost = append(m.detachedFromHost, hostID)
	return "", nil
}

func (m *mockClient) DetachVolumeFromHostGroup(_ context.Context, groupID string, _ *gopowerstore.HostVolumeDetach) (gopowerstore.EmptyResponse, error) {
	m.detachedFromGroup = append(m.detachedFromGroup, groupID)
	return "", nil
}

func newTestClonner(mc *mockClient) PowerstoreClonner {
	return PowerstoreClonner{Client: mc, log: klog.Background()}
}

func newMappingContext(hostID, logicalName, realName string) populator.MappingContext {
	return populator.MappingContext{
		hostIDContextKey:      hostID,
		esxLogicalHostNameKey: logicalName,
		esxRealHostNameKey:    realName,
	}
}

func TestMap(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("should attach volume to individual host when host has no host group", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{
				{ID: "host-1", Name: "esxi-host-1"},
			},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "esxi-host-1")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		_, err := clonner.Map("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mc.attachedToHost).To(gomega.Equal([]string{"host-1"}))
		g.Expect(mc.attachedToGroup).To(gomega.BeEmpty())
	})

	t.Run("should attach volume to host group when host belongs to a group", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{
				{ID: "host-1", Name: "esxi-host-1", HostGroupID: "hg-1"},
			},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "esxi-host-1")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		_, err := clonner.Map("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mc.attachedToGroup).To(gomega.Equal([]string{"hg-1"}))
		g.Expect(mc.attachedToHost).To(gomega.BeEmpty())
	})

	t.Run("should skip attach when volume already mapped to host group", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{
				{ID: "host-1", Name: "esxi-host-1", HostGroupID: "hg-1"},
			},
			volumeMappings: []gopowerstore.HostVolumeMapping{
				{HostGroupID: "hg-1", HostID: "host-1"},
			},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "esxi-host-1")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		_, err := clonner.Map("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mc.attachedToGroup).To(gomega.BeEmpty())
		g.Expect(mc.attachedToHost).To(gomega.BeEmpty())
	})

	t.Run("should skip attach when volume already mapped to individual host", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{
				{ID: "host-1", Name: "esxi-host-1"},
			},
			volumeMappings: []gopowerstore.HostVolumeMapping{
				{HostID: "host-1"},
			},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "esxi-host-1")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		_, err := clonner.Map("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mc.attachedToHost).To(gomega.BeEmpty())
		g.Expect(mc.attachedToGroup).To(gomega.BeEmpty())
	})

	t.Run("should return error when IQN is empty", func(t *testing.T) {
		clonner := PowerstoreClonner{}
		lun := populator.LUN{Name: "vol-1", IQN: ""}
		_, err := clonner.Map("ig", lun, newMappingContext("h", "l", "r"))
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("IQN is required"))
	})

	t.Run("should return error when mapping context is nil", func(t *testing.T) {
		clonner := PowerstoreClonner{}
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}
		_, err := clonner.Map("ig", lun, nil)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("mapping context is required"))
	})

	t.Run("should return error when host is not found", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "nonexistent-host")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		_, err := clonner.Map("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("failed to find host"))
	})

	t.Run("should attach to host group even when volume is mapped to a different group", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{
				{ID: "host-1", Name: "esxi-host-1", HostGroupID: "hg-1"},
			},
			volumeMappings: []gopowerstore.HostVolumeMapping{
				{HostGroupID: "hg-other", HostID: "host-99"},
			},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "esxi-host-1")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		_, err := clonner.Map("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mc.attachedToGroup).To(gomega.Equal([]string{"hg-1"}))
	})
}

func TestUnMap(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	t.Run("should detach volume from individual host when host has no host group", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{
				{ID: "host-1", Name: "esxi-host-1"},
			},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "esxi-host-1")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		err := clonner.UnMap("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mc.detachedFromHost).To(gomega.Equal([]string{"host-1"}))
		g.Expect(mc.detachedFromGroup).To(gomega.BeEmpty())
	})

	t.Run("should detach volume from host group when host belongs to a group", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{
				{ID: "host-1", Name: "esxi-host-1", HostGroupID: "hg-1"},
			},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "esxi-host-1")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		err := clonner.UnMap("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(mc.detachedFromGroup).To(gomega.Equal([]string{"hg-1"}))
		g.Expect(mc.detachedFromHost).To(gomega.BeEmpty())
	})

	t.Run("should return error when IQN is empty", func(t *testing.T) {
		clonner := PowerstoreClonner{}
		lun := populator.LUN{Name: "vol-1", IQN: ""}
		err := clonner.UnMap("ig", lun, newMappingContext("h", "l", "r"))
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("IQN is required"))
	})

	t.Run("should return error when mapping context is nil", func(t *testing.T) {
		clonner := PowerstoreClonner{}
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}
		err := clonner.UnMap("ig", lun, nil)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("mapping context is required"))
	})

	t.Run("should return error when host is not found", func(t *testing.T) {
		mc := &mockClient{
			hosts: []gopowerstore.Host{},
		}
		clonner := newTestClonner(mc)
		ctx := newMappingContext("host-1", "xcopy-hostsystem-ha-host", "nonexistent-host")
		lun := populator.LUN{Name: "vol-1", IQN: "vol-iqn-1"}

		err := clonner.UnMap("xcopy-hostsystem-ha-host", lun, ctx)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(err.Error()).To(gomega.ContainSubstring("failed to find host"))
	})
}
