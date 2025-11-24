package iboxapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

/**
log levels
logr.V(0) - Info level logging in zerolog
logr.V(1) - Debug level logging in zerolog
logr.V(2) - Trace level logging in zerolog
*/

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

func (iboxClient *IboxClient) CreateEvent(eventRequest EventRequest) (err error) {
	const functionName = "CreateEvent"

	url := fmt.Sprintf("%s%s", iboxClient.Creds.URL, "api/rest/events")
	iboxClient.Log.V(TRACE_LEVEL).Info(functionName, "URL", url, "event", eventRequest)

	jsonBytes, err := json.Marshal(eventRequest)
	if err != nil {
		return fmt.Errorf("%s - Marshal - error %w", functionName, err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("%s - NewRequest - error %w", functionName, err)
	}
	SetAuthHeader(request, iboxClient.Creds)
	request.Header.Set(CONTENT_TYPE, JSON_CONTENT_TYPE)

	response, err := iboxClient.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("%s - Do - error %w", functionName, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			iboxClient.Log.V(INFO_LEVEL).Error(err, functionName, "error in Close()", err.Error())
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("%s - ReadAll - error %w", functionName, err)
	}

	var responseObject CreateEventResponse
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return fmt.Errorf("%s - Unmarshal - error %w", functionName, err)
	}
	iboxClient.Log.V(DEBUG_LEVEL).Info("CreateEvent", "Event ID", responseObject.Result.ID)
	if responseObject.Error.Code != "" {
		return fmt.Errorf("%s - ibox API - error:  code: %s message: %s", functionName, responseObject.Error.Code, responseObject.Error.Message)
	}
	return nil
}
