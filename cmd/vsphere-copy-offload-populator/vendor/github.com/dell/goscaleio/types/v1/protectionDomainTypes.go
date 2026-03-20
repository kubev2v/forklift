package goscaleio

import (
	"encoding/json"
	"strconv"
)

// PDRfCacheParams is used to manipulate Read Flash Cache settings of a protection domain
type PDRfCacheParams struct {
	RfCacheOperationalMode PDRfCacheOpMode
	RfCachePageSizeKb      int
	RfCacheMaxIoSizeKb     int
}

// MarshalJSON implements a custom json marshalling
func (params PDRfCacheParams) MarshalJSON() ([]byte, error) {
	m := make(map[string]string)
	if params.RfCachePageSizeKb != 0 {
		m["pageSizeKb"] = strconv.Itoa(params.RfCachePageSizeKb)
	}
	if params.RfCacheMaxIoSizeKb != 0 {
		m["maxIOSizeKb"] = strconv.Itoa(params.RfCacheMaxIoSizeKb)
	}
	if params.RfCacheOperationalMode != "" {
		m["rfcacheOperationMode"] = string(params.RfCacheOperationalMode)
	}
	return json.Marshal(m)
}

// GetRfCacheParams is a function to extract RF cache params from a protection domain
func (pd *ProtectionDomain) GetRfCacheParams() PDRfCacheParams {
	return PDRfCacheParams{
		RfCacheOperationalMode: pd.RfCacheOperationalMode,
		RfCachePageSizeKb:      pd.RfCachePageSizeKb,
		RfCacheMaxIoSizeKb:     pd.RfCacheMaxIoSizeKb,
	}
}

// SdsNetworkLimitParams is used to set IOPS limits on all SDS of a protection domain
type SdsNetworkLimitParams struct {
	RebuildNetworkThrottlingInKbps                  *int
	RebalanceNetworkThrottlingInKbps                *int
	VTreeMigrationNetworkThrottlingInKbps           *int
	ProtectedMaintenanceModeNetworkThrottlingInKbps *int
	OverallIoNetworkThrottlingInKbps                *int
}

// MarshalJSON implements a custom json marshalling
func (params SdsNetworkLimitParams) MarshalJSON() ([]byte, error) {
	m := make(map[string]string)
	if size := params.RebuildNetworkThrottlingInKbps; size != nil {
		m["rebuildLimitInKbps"] = strconv.Itoa(*size)
	}
	if size := params.RebalanceNetworkThrottlingInKbps; size != nil {
		m["rebalanceLimitInKbps"] = strconv.Itoa(*size)
	}
	if size := params.VTreeMigrationNetworkThrottlingInKbps; size != nil {
		m["vtreeMigrationLimitInKbps"] = strconv.Itoa(*size)
	}
	if size := params.ProtectedMaintenanceModeNetworkThrottlingInKbps; size != nil {
		m["protectedMaintenanceModeLimitInKbps"] = strconv.Itoa(*size)
	}
	if size := params.OverallIoNetworkThrottlingInKbps; size != nil {
		m["overallLimitInKbps"] = strconv.Itoa(*size)
	}
	return json.Marshal(m)
}

// GetNwLimitParams is a function to extract IOPS limit params from a protection domain
func (pd *ProtectionDomain) GetNwLimitParams() SdsNetworkLimitParams {
	return SdsNetworkLimitParams{
		RebuildNetworkThrottlingInKbps:                  &(pd.RebuildNetworkThrottlingInKbps),
		RebalanceNetworkThrottlingInKbps:                &(pd.RebalanceNetworkThrottlingInKbps),
		VTreeMigrationNetworkThrottlingInKbps:           &(pd.VTreeMigrationNetworkThrottlingInKbps),
		ProtectedMaintenanceModeNetworkThrottlingInKbps: &(pd.ProtectedMaintenanceModeNetworkThrottlingInKbps),
		OverallIoNetworkThrottlingInKbps:                &(pd.OverallIoNetworkThrottlingInKbps),
	}
}
