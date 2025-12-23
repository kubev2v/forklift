package vantara

import (
	"testing"
)

// TestFindHostGroupIDs tests finding host group IDs from port details
func TestFindHostGroupIDs(t *testing.T) {
	jsonData := JSONData{
		Data: []DataEntry{
			{
				PortID: "CL1-A",
				WWN:    "50060E801234ABCD",
				Logins: []Logins{
					{
						HostGroupId: "CL1-A,1",
						Islogin:     "true",
						LoginWWN:    "21000024FF123456",
						WWNNickName: "ESXi-Host-1",
					},
					{
						HostGroupId: "CL1-A,2",
						Islogin:     "true",
						LoginWWN:    "21000024FF789ABC",
						WWNNickName: "ESXi-Host-2",
					},
				},
			},
			{
				PortID: "CL2-B",
				WWN:    "50060E805678EFGH",
				Logins: []Logins{
					{
						HostGroupId: "CL2-B,1",
						Islogin:     "true",
						LoginWWN:    "21000024FFABCDEF",
						WWNNickName: "ESXi-Host-3",
					},
				},
			},
		},
	}

	tests := []struct {
		name         string
		hbaUIDs      []string
		expectedLen  int
		expectedHGID string
	}{
		{
			name:         "Single FC HBA match",
			hbaUIDs:      []string{"fc.0:21000024FF123456"},
			expectedLen:  1,
			expectedHGID: "CL1-A,1",
		},
		{
			name:         "Multiple FC HBA matches",
			hbaUIDs:      []string{"fc.0:21000024FF123456", "fc.1:21000024FF789ABC"},
			expectedLen:  2,
			expectedHGID: "CL1-A,1", // First match
		},
		{
			name:        "No match",
			hbaUIDs:     []string{"fc.0:NONEXISTENT1234"},
			expectedLen: 0,
		},
		{
			name:         "Case insensitive match",
			hbaUIDs:      []string{"fc.0:21000024ff123456"}, // lowercase
			expectedLen:  1,
			expectedHGID: "CL1-A,1",
		},
		{
			name:        "iSCSI HBA (not implemented - skipped)",
			hbaUIDs:     []string{"iqn.1998-01.com.vmware:esxi-host"},
			expectedLen: 0,
		},
		{
			name:        "NVMe HBA (not implemented - skipped)",
			hbaUIDs:     []string{"nqn.2014-08.org.nvmexpress:uuid:12345"},
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := FindHostGroupIDs(jsonData, tt.hbaUIDs)

			if len(results) != tt.expectedLen {
				t.Errorf("Expected %d results, got %d", tt.expectedLen, len(results))
			}

			if tt.expectedLen > 0 && len(results) > 0 {
				if results[0].HostGroupId != tt.expectedHGID {
					t.Errorf("Expected first HostGroupId=%s, got %s", tt.expectedHGID, results[0].HostGroupId)
				}
			}
		})
	}
}

// TestFindHostGroupIDsInvalidFormat tests invalid FC WWN formats
func TestFindHostGroupIDsInvalidFormat(t *testing.T) {
	jsonData := JSONData{
		Data: []DataEntry{
			{
				PortID: "CL1-A",
				WWN:    "50060E801234ABCD",
				Logins: []Logins{
					{
						HostGroupId: "CL1-A,1",
						Islogin:     "true",
						LoginWWN:    "21000024FF123456",
					},
				},
			},
		},
	}

	invalidUIDs := []string{
		"fc.invalid",                 // Missing WWN part
		"fc.0",                       // Missing colon separator
		"fc.0:123",                   // Too short
		"unknown.0:21000024FF123456", // Unknown prefix
		"",                           // Empty string
	}

	for _, uid := range invalidUIDs {
		t.Run(uid, func(t *testing.T) {
			results := FindHostGroupIDs(jsonData, []string{uid})
			// Invalid formats should return empty results, not panic
			if len(results) != 0 {
				t.Errorf("Expected 0 results for invalid UID %s, got %d", uid, len(results))
			}
		})
	}
}

// TestFindHostGroupIDsMultiplePorts tests matching across multiple ports
func TestFindHostGroupIDsMultiplePorts(t *testing.T) {
	jsonData := JSONData{
		Data: []DataEntry{
			{
				PortID: "CL1-A",
				WWN:    "50060E801234ABCD",
				Logins: []Logins{
					{
						HostGroupId: "CL1-A,1",
						Islogin:     "true",
						LoginWWN:    "21000024FF111111",
					},
				},
			},
			{
				PortID: "CL2-B",
				WWN:    "50060E805678EFGH",
				Logins: []Logins{
					{
						HostGroupId: "CL2-B,1",
						Islogin:     "true",
						LoginWWN:    "21000024FF222222",
					},
				},
			},
			{
				PortID: "CL3-C",
				WWN:    "50060E809ABC1234",
				Logins: []Logins{
					{
						HostGroupId: "CL3-C,1",
						Islogin:     "true",
						LoginWWN:    "21000024FF333333",
					},
				},
			},
		},
	}

	hbaUIDs := []string{
		"fc.0:21000024FF111111",
		"fc.1:21000024FF222222",
		"fc.2:21000024FF333333",
	}

	results := FindHostGroupIDs(jsonData, hbaUIDs)

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	expectedHGIDs := []string{"CL1-A,1", "CL2-B,1", "CL3-C,1"}
	for i, expected := range expectedHGIDs {
		found := false
		for _, result := range results {
			if result.HostGroupId == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find HostGroupId=%s in results", expected)
		}
		_ = i
	}
}

// TestFindHostGroupIDsEmptyData tests with empty input
func TestFindHostGroupIDsEmptyData(t *testing.T) {
	tests := []struct {
		name     string
		jsonData JSONData
		hbaUIDs  []string
	}{
		{
			name:     "Empty data entries",
			jsonData: JSONData{Data: []DataEntry{}},
			hbaUIDs:  []string{"fc.0:21000024FF123456"},
		},
		{
			name: "Empty logins",
			jsonData: JSONData{
				Data: []DataEntry{
					{
						PortID: "CL1-A",
						WWN:    "50060E801234ABCD",
						Logins: []Logins{},
					},
				},
			},
			hbaUIDs: []string{"fc.0:21000024FF123456"},
		},
		{
			name: "Empty HBA UIDs",
			jsonData: JSONData{
				Data: []DataEntry{
					{
						PortID: "CL1-A",
						WWN:    "50060E801234ABCD",
						Logins: []Logins{
							{
								HostGroupId: "CL1-A,1",
								LoginWWN:    "21000024FF123456",
							},
						},
					},
				},
			},
			hbaUIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := FindHostGroupIDs(tt.jsonData, tt.hbaUIDs)
			if len(results) != 0 {
				t.Errorf("Expected 0 results for %s, got %d", tt.name, len(results))
			}
		})
	}
}

// TestFindHostGroupIDsLoginFields tests that all login fields are preserved
func TestFindHostGroupIDsLoginFields(t *testing.T) {
	jsonData := JSONData{
		Data: []DataEntry{
			{
				PortID: "CL1-A",
				WWN:    "50060E801234ABCD",
				Logins: []Logins{
					{
						HostGroupId:     "CL1-A,1",
						Islogin:         "true",
						LoginWWN:        "21000024FF123456",
						WWNNickName:     "ESXi-Host-1",
						IscsiNickName:   "",
						IscsiTargetName: "",
						LoginIscsiName:  "",
					},
				},
			},
		},
	}

	results := FindHostGroupIDs(jsonData, []string{"fc.0:21000024FF123456"})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.HostGroupId != "CL1-A,1" {
		t.Errorf("Expected HostGroupId=CL1-A,1, got %s", result.HostGroupId)
	}
	if result.Islogin != "true" {
		t.Errorf("Expected Islogin=true, got %s", result.Islogin)
	}
	if result.LoginWWN != "21000024FF123456" {
		t.Errorf("Expected LoginWWN=21000024FF123456, got %s", result.LoginWWN)
	}
	if result.WWNNickName != "ESXi-Host-1" {
		t.Errorf("Expected WWNNickName=ESXi-Host-1, got %s", result.WWNNickName)
	}
	// iSCSI fields should be empty for FC logins
	if result.IscsiNickName != "" {
		t.Errorf("Expected empty IscsiNickName, got %s", result.IscsiNickName)
	}
}
