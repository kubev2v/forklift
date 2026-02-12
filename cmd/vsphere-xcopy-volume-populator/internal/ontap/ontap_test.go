package ontap

import (
	"fmt"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
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

func TestParseInternalIDToLunPath(t *testing.T) {
	tests := []struct {
		name       string
		internalID string
		want       string
		wantErr    bool
	}{
		{
			name:       "valid ontap-san-economy internalID",
			internalID: "/svm/vserver-ecosystem-mtv/flexvol/trident_lun_pool_copy_offload_cluster_ASVRVZBSPB/lun/copy_offload_cluster_pvc_5c9d270a_11c8_4c36_9f12_f0d5f69870cf",
			want:       "/vol/trident_lun_pool_copy_offload_cluster_ASVRVZBSPB/copy_offload_cluster_pvc_5c9d270a_11c8_4c36_9f12_f0d5f69870cf",
			wantErr:    false,
		},
		{
			name:       "simple valid internalID",
			internalID: "/svm/mysvm/flexvol/myflexvol/lun/mylun",
			want:       "/vol/myflexvol/mylun",
			wantErr:    false,
		},
		{
			name:       "invalid internalID - missing flexvol",
			internalID: "/svm/mysvm/lun/mylun",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "invalid internalID - missing lun",
			internalID: "/svm/mysvm/flexvol/myflexvol",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "invalid internalID - empty",
			internalID: "",
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInternalIDToLunPath(tt.internalID)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInternalIDToLunPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseInternalIDToLunPath() = %v, want %v", got, tt.want)
			}
		})
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

func TestNetappClonner_ResolvePVToLUN_EconomyStorageClass(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockOntapAPI(ctrl)
	clonner := &NetappClonner{api: mockAPI}

	pv := populator.PersistentVolume{
		Name: "test-pv-economy",
		VolumeAttributes: map[string]string{
			"internalName": "copy_offload_cluster_pvc_5c9d270a_11c8_4c36_9f12_f0d5f69870cf",
			"internalID":   "/svm/vserver-ecosystem-mtv/flexvol/trident_lun_pool_copy_offload_cluster_ASVRVZBSPB/lun/copy_offload_cluster_pvc_5c9d270a_11c8_4c36_9f12_f0d5f69870cf",
		},
	}
	expectedPath := "/vol/trident_lun_pool_copy_offload_cluster_ASVRVZBSPB/copy_offload_cluster_pvc_5c9d270a_11c8_4c36_9f12_f0d5f69870cf"
	expectedLUN := &api.Lun{
		Name:         expectedPath,
		SerialNumber: "test-serial-economy",
	}

	mockAPI.EXPECT().LunGetByName(gomock.Any(), expectedPath).Return(expectedLUN, nil)

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
	// NOSONAR - fake test credentials for unit testing, not real values
	_, err := NewNetappClonner("fake-hostname.invalid", "fake-user", "fake-pass")
	if err == nil {
		t.Errorf("NewNetappClonner() error = %v, wantErr %v", err, true)
	}
}

func TestNetappClonner_EnsureClonnerIgroup_WithFCConversion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAPI := NewMockOntapAPI(ctrl)
	clonner := &NetappClonner{api: mockAPI}

	igroup := "test-igroup"
	// Test data: fake FC and iSCSI adapter IDs for testing format conversion
	// These are Fibre Channel WWNs (World Wide Names), NOT IP addresses
	fakeFC := "fc.2000000000000001:2100000000000002" // fake FC adapter (WWNN:WWPN format)
	fakeIQN := "iqn.2099-01.com.fake:testhost:99"    // fake iSCSI IQN
	expectedWWPN := "21:00:00:00:00:00:00:02"        // nosonar
	adapterIDs := []string{fakeFC, fakeIQN}

	mockAPI.EXPECT().IgroupCreate(gomock.Any(), igroup, "mixed", "vmware").Return(nil)
	// FC adapter should be converted from fc.WWNN:WWPN to colon-separated WWPN
	mockAPI.EXPECT().EnsureIgroupAdded(gomock.Any(), igroup, expectedWWPN).Return(nil)
	// iSCSI adapter should pass through unchanged
	mockAPI.EXPECT().EnsureIgroupAdded(gomock.Any(), igroup, fakeIQN).Return(nil)

	_, err := clonner.EnsureClonnerIgroup(igroup, adapterIDs)
	if err != nil {
		t.Errorf("EnsureClonnerIgroup() error = %v, wantErr %v", err, false)
	}
}
