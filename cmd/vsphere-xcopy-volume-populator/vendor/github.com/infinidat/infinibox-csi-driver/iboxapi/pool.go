package iboxapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

func (client *IboxClient) GetPoolByName(ctx context.Context, name string) (pool *PoolResult, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/pools")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "name", name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	values := req.URL.Query()
	values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(common.IBOXDefaultQueryPageSize))
	values.Add(PARAMETER_PAGE, strconv.Itoa(1))
	values.Add("name", name)
	req.URL.RawQuery = values.Encode()

	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var response GetPoolByNameResponse
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url: %s", err, url)
	}

	if response.Error.Code != "" {
		return nil, common.Errorf("ibox API - error: %v url: %s", response.Error, url)
	}

	if len(response.Result) == 0 {
		return nil, ErrNotFound
	}
	pool = &response.Result[0]

	return pool, nil
}

func (client *IboxClient) GetPoolByID(ctx context.Context, poolID int) (pool *PoolResult, err error) {
	url := fmt.Sprintf("%s%s/%d", client.Creds.URL, "api/rest/pools", poolID)
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "id", poolID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, common.Errorf("newRequest - error: %w url: %s", err, url)
	}

	SetAuthHeader(req, client.Creds)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.Errorf("readAll - error: %w url: %s", err, url)
	}
	var response GetPoolByIDResponse
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, common.Errorf("unmarshal - error: %w url; %s", err, url)
	}

	if response.Error.Code != "" {
		if response.Error.Code == "POOL_NOT_FOUND" {
			return nil, common.Errorf("errorCode: %s - error: %w url: %s", response.Error.Code, ErrNotFound, url)
		}
		return nil, common.Errorf("ibox API - error: %v", response.Error)
	}

	return &response.Result, nil
}
