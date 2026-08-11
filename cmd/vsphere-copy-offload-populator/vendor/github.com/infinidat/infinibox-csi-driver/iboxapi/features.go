package iboxapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/infinidat/infinibox-csi-driver/common"
)

type Features struct {
	Result   []FeatureResult `json:"result"`
	Error    Error           `json:"error"`
	Metadata Metadata        `json:"metadata"`
}
type FeatureResult struct {
	Name         string `json:"name"`
	Version      int    `json:"version"`
	Experimental bool   `json:"experimental"`
	Enabled      bool   `json:"enabled"`
}

func (client *IboxClient) GetFeatures(ctx context.Context) (result []FeatureResult, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/_features")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url)

	parameters := make(map[string]string)

	bodyBytes, err := commonGetLogic(ctx, url, client, parameters)
	if err != nil {
		return nil, common.Errorf("commonGetLogic - error: %w url: %s", err, url)
	}

	var response Features
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

	return response.Result, nil
}
