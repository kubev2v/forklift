package iboxapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/infinidat/infinibox-csi-driver/common"
)

type GetPoolByIDResponse struct {
	Metadata Metadata   `json:"metadata"`
	Result   PoolResult `json:"result"`
	Error    Error      `json:"error"`
}

type GetPoolByNameResponse struct {
	Metadata Metadata     `json:"metadata"`
	Result   []PoolResult `json:"result"`
	Error    Error        `json:"error"`
}
type PoolResult struct {
	VolumesCount                     int     `json:"volumes_count"`
	StandardEntitiesCount            int     `json:"standard_entities_count"`
	UpdatedAt                        int64   `json:"updated_at"`
	StandardFilesystemSnapshotsCount int     `json:"standard_filesystem_snapshots_count"`
	StandardSnapshotsCount           int     `json:"standard_snapshots_count"`
	MaxExtend                        int     `json:"max_extend"`
	AllocatedPhysicalSpace           int     `json:"allocated_physical_space"`
	FreeVirtualSpace                 int64   `json:"free_virtual_space"`
	StandardFilesystemsCount         int     `json:"standard_filesystems_count"`
	ID                               int     `json:"id"`
	ReservedCapacity                 int64   `json:"reserved_capacity"`
	FilesystemsCount                 int     `json:"filesystems_count"`
	SsdEnabled                       bool    `json:"ssd_enabled"`
	VvolEntitiesCount                int     `json:"vvol_entities_count"`
	SnapshotsCount                   int     `json:"snapshots_count"`
	State                            string  `json:"state"`
	VvolVolumesCount                 int     `json:"vvol_volumes_count"`
	Type                             string  `json:"type"`
	FreePhysicalSpace                int64   `json:"free_physical_space"`
	DataReductionRatio               float64 `json:"data_reduction_ratio"`
	TotalDiskUsage                   any     `json:"total_disk_usage"`
	VvolSnapshotsCount               int     `json:"vvol_snapshots_count"`
	ThinCapacitySavings              any     `json:"thin_capacity_savings"`
	EntitiesCount                    int     `json:"entities_count"`
	PhysicalCapacityCritical         int     `json:"physical_capacity_critical"`
	StandardVolumesCount             int     `json:"standard_volumes_count"`
	Owners                           []any   `json:"owners"`
	CapacitySavings                  any     `json:"capacity_savings"`
	Name                             string  `json:"name"`
	VirtualCapacity                  int64   `json:"virtual_capacity"`
	TenantID                         int     `json:"tenant_id"`
	CreatedAt                        int64   `json:"created_at"`
	FilesystemSnapshotsCount         int     `json:"filesystem_snapshots_count"`
	CompressionEnabled               bool    `json:"compression_enabled"`
	QosPolicies                      []any   `json:"qos_policies"`
	PhysicalCapacityWarning          int     `json:"physical_capacity_warning"`
	PhysicalCapacity                 int64   `json:"physical_capacity"`
	ThickCapacitySavings             any     `json:"thick_capacity_savings"`
}

func (iboxClient *IboxClient) GetPoolByName(name string) (pool *PoolResult, err error) {
	const functionName = "GetPoolByName"
	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/pools")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "name", name)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	values := req.URL.Query()
	values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(common.IBOXDefaultQueryPageSize))
	values.Add(PARAMETER_PAGE, strconv.Itoa(1))
	values.Add("name", name)
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject GetPoolByNameResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}

	if len(responseObject.Result) > 0 {
		pool = &responseObject.Result[0]
	} else {
		return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - pool '%s' not found", functionName, name)}
	}

	return pool, nil
}

func (iboxClient *IboxClient) GetPoolByID(poolID int) (pool *PoolResult, err error) {
	const functionName = "GetPoolByID"
	url := fmt.Sprintf("%s%s/%d", iboxClient.Creds.URL, "api/rest/pools", poolID)
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "id", poolID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}

	SetAuthHeader(req, iboxClient.Creds)

	resp, err := iboxClient.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}
	var responseObject GetPoolByIDResponse
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		return nil, fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}

	if responseObject.Error.Code == "POOL_NOT_FOUND" {
		return nil, &APIError{Code: IBOXAPI_RESOURCE_NOT_FOUND_ERROR, Err: fmt.Errorf("%s - pool '%d' not found", functionName, poolID)}
	}

	if responseObject.Error.Code != "" {
		return nil, fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}

	return &responseObject.Result, nil
}
