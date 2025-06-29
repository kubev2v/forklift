package primera3par

import (
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/stretchr/testify/assert"
)

func TestPrimera3ParClonner(t *testing.T) {
	mockClient := NewMockPrimera3ParClient()
	clonner := Primera3ParClonner{client: mockClient}

	targetLUN := populator.LUN{
		Name: "TestVolume",
		IQN:  "iqn.1993-08.org.debian:01:test1234",
	}
	initiatorGroup := "TestInitiatorGroup"

	mockClient.Volumes[targetLUN.Name] = targetLUN

	t.Run("Ensure Clonner Igroup", func(t *testing.T) {
		_, err := clonner.EnsureClonnerIgroup(initiatorGroup, []string{targetLUN.IQN})
		assert.NoError(t, err, "Expected no error when ensuring Clonner Igroup")
		_, hostExists := mockClient.Hosts["mock-host-"+targetLUN.IQN]
		assert.True(t, hostExists, "Expected host to exist")
		_, hostSetExists := mockClient.HostSets[initiatorGroup]
		assert.True(t, hostSetExists, "Expected host set to exist")
	})

	t.Run("Map LUN", func(t *testing.T) {
		_, err := clonner.Map(initiatorGroup, targetLUN, nil)
		assert.NoError(t, err, "Expected no error when mapping LUN")
	})

	t.Run("Current Mapped Groups", func(t *testing.T) {
		groups, err := clonner.CurrentMappedGroups(targetLUN, nil)
		assert.NoError(t, err, "Expected no error when fetching mapped groups")
		assert.Contains(t, groups, initiatorGroup, "Expected initiator group to be mapped")
	})

	t.Run("Resolve Volume Handle to LUN", func(t *testing.T) {
		_, err := clonner.ResolvePVToLUN(populator.PersistentVolume{Name: targetLUN.Name})
		assert.NoError(t, err, "Expected no error when resolving LUN details")
	})

	t.Run("Unmap LUN", func(t *testing.T) {
		err := clonner.UnMap(initiatorGroup, targetLUN, nil)
		assert.NoError(t, err, "Expected no error when unmapping LUN")
	})
}
