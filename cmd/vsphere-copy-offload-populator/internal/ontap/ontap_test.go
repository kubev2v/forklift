package ontap

import (
	"fmt"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/netapp/trident/storage_drivers/ontap/api"
	"go.uber.org/mock/gomock"
)

func TestNetappClonner_Map(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockOntapAPI(ctrl)
	clonner := &NetappClonner{api: mockAPI}

	igroup := "test-igroup"
	lun := populator.LUN{Name: "test-lun"}

	mockAPI.EXPECT().EnsureLunMapped(gomock.Any(), igroup, lun.Name).Return(1, nil)

	mappedLUN, err := clonner.Map(igroup, lun, nil)
	if err != nil {
		t.Errorf("Map() error = %v, wantErr %v", err, false)
	}
	if mappedLUN.Name != lun.Name {
		t.Errorf("Map() = %v, want %v", mappedLUN.Name, lun.Name)
	}
}

func TestNetappClonner_UnMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockOntapAPI(ctrl)
	clonner := &NetappClonner{api: mockAPI}

	igroup := "test-igroup"
	lun := populator.LUN{Name: "test-lun"}

	mockAPI.EXPECT().LunUnmap(gomock.Any(), igroup, lun.Name).Return(nil)

	err := clonner.UnMap(igroup, lun, nil)
	if err != nil {
		t.Errorf("UnMap() error = %v, wantErr %v", err, false)
	}
}

func TestNetappClonner_EnsureClonnerIgroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockOntapAPI(ctrl)
	clonner := &NetappClonner{api: mockAPI}

	igroup := "test-igroup"
	adapterIDs := []string{"adapter1", "adapter2"}

	mockAPI.EXPECT().IgroupCreate(gomock.Any(), igroup, "mixed", "vmware").Return(nil)
	mockAPI.EXPECT().EnsureIgroupAdded(gomock.Any(), igroup, "adapter1").Return(nil)
	mockAPI.EXPECT().EnsureIgroupAdded(gomock.Any(), igroup, "adapter2").Return(nil)

	_, err := clonner.EnsureClonnerIgroup(igroup, adapterIDs)
	if err != nil {
		t.Errorf("EnsureClonnerIgroup() error = %v, wantErr %v", err, false)
	}
}

func TestNetappClonner_ResolvePVToLUN(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockOntapAPI(ctrl)
	clonner := &NetappClonner{api: mockAPI}

	pv := populator.PersistentVolume{
		Name: "test-pv",
		VolumeAttributes: map[string]string{
			"internalName": "test-internal-name",
		},
	}
	lunPath := fmt.Sprintf("/vol/%s/lun0", pv.VolumeAttributes["internalName"])
	expectedLUN := &api.Lun{
		Name:         lunPath,
		SerialNumber: "test-serial",
	}

	mockAPI.EXPECT().LunGetByName(gomock.Any(), lunPath).Return(expectedLUN, nil)

	lun, err := clonner.ResolvePVToLUN(pv)
	if err != nil {
		t.Errorf("ResolvePVToLUN() error = %v, wantErr %v", err, false)
	}
	if lun.Name != expectedLUN.Name {
		t.Errorf("ResolvePVToLUN() = %v, want %v", lun.Name, expectedLUN.Name)
	}
}

func TestNetappClonner_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockOntapAPI(ctrl)
	clonner := &NetappClonner{api: mockAPI}

	expectedIPs := []string{"192.0.2.1", "192.0.2.2"}
	mockAPI.EXPECT().NetInterfaceGetDataLIFs(gomock.Any(), "iscsi").Return(expectedIPs, nil)

	ip, err := clonner.Get(populator.LUN{}, nil)
	if err != nil {
		t.Errorf("Get() error = %v, wantErr %v", err, false)
	}
	if ip != expectedIPs[0] {
		t.Errorf("Get() = %v, want %v", ip, expectedIPs[0])
	}
}

func TestNetappClonner_CurrentMappedGroups(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockOntapAPI(ctrl)
	clonner := &NetappClonner{api: mockAPI}

	lun := populator.LUN{Name: "test-lun"}
	expectedGroups := []string{"group1", "group2"}

	mockAPI.EXPECT().LunListIgroupsMapped(gomock.Any(), lun.Name).Return(expectedGroups, nil)

	groups, err := clonner.CurrentMappedGroups(lun, nil)
	if err != nil {
		t.Errorf("CurrentMappedGroups() error = %v, wantErr %v", err, false)
	}
	if len(groups) != len(expectedGroups) {
		t.Errorf("CurrentMappedGroups() = %v, want %v", groups, expectedGroups)
	}
}

func TestNewNetappClonner(t *testing.T) {
	_, err := NewNetappClonner("invalid-hostname", "username", "password")
	if err == nil {
		t.Errorf("NewNetappClonner() error = %v, wantErr %v", err, true)
	}
}
