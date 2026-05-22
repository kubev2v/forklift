package iboxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/infinidat/infinibox-csi-driver/common"
)

type EventRequest struct {
	Data []EventRequestData `json:"data"`
	Code string             `json:"code"`
}
type EventRequestData struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type CreateEventResponse struct {
	Result   CreateEventResult `json:"result"`
	Error    Error             `json:"error"`
	Metadata Metadata          `json:"metadata"`
}

type CreateEventResult struct {
	AffectedEntityID    int    `json:"affected_entity_id"`
	Username            string `json:"username"`
	Code                string `json:"code"`
	Description         string `json:"description"`
	Timestamp           int64  `json:"timestamp"`
	Level               string `json:"level"`
	SeqNum              int    `json:"seq_num"`
	TenantID            int    `json:"tenant_id"`
	Reporter            string `json:"reporter"`
	Visibility          string `json:"visibility"`
	SystemVersion       string `json:"system_version"`
	SourceNodeID        int    `json:"source_node_id"`
	DescriptionTemplate string `json:"description_template"`
	Data                []any  `json:"data"`
	ID                  int    `json:"id"`
}

func (client *IboxClient) CreateEvent(ctx context.Context, eventRequest EventRequest) (err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/events")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url, "event", eventRequest)

	jsonBytes, err := json.Marshal(eventRequest)
	if err != nil {
		return common.Errorf("marshal - error: %w url: %s", err, url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return common.Errorf("newRequest - error: %w url: %s", err, url)
	}
	SetAuthHeader(request, client.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return common.Errorf("do - error: %w url: %s", err, url)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Error("error in Close()", "error", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return common.Errorf("readAll - error: %w url: %s", err, url)
	}

	var responseObject CreateEventResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return common.Errorf("unmarshal - error: %w url: %s", err, url)
	}
	slog.Debug("CreateEvent", "Event ID", responseObject.Result.ID)
	if responseObject.Error.Code != "" {
		return common.Errorf("ibox API - error: %v url: %s", responseObject.Error, url)
	}
	return nil
}
