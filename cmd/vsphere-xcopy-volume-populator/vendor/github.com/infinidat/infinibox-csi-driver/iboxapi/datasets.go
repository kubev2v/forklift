package iboxapi

/*
Copyright 2025 Infinidat
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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

type GetAllSnapshotsResponse struct {
	Metadata Metadata `json:"metadata"`
	Result   []Volume `json:"result"`
	Error    Error    `json:"error"`
}

func (client *IboxClient) GetAllSnapshots(ctx context.Context) (results []Volume, err error) {
	url := fmt.Sprintf("%s%s", client.Creds.URL, "api/rest/datasets")
	slog.Log(ctx, common.LevelTrace, "info", "URL", url)

	pageSize := common.IBOXDefaultQueryPageSize
	totalPages := 1 // start with 1, update after first query.
	for page := 1; page <= totalPages; page++ {
		slog.Log(ctx, common.LevelTrace, "info", "page", page, "totalPages", totalPages)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return results, common.Errorf("newRequest - error: %w url: %s", err, url)
		}

		values := req.URL.Query()
		values.Add(PARAMETER_PAGE_SIZE, strconv.Itoa(pageSize))
		values.Add(PARAMETER_PAGE, strconv.Itoa(page))
		values.Add("type", "SNAPSHOT")
		req.URL.RawQuery = values.Encode()

		SetAuthHeader(req, client.Creds)

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return results, common.Errorf("do - error: %w url: %s", err, url)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Error("error in Close()", "error", err.Error())
			}
		}()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return results, common.Errorf("readAll - error: %w url: %s", err, url)
		}
		var responseObject GetAllSnapshotsResponse
		err = json.Unmarshal(bodyBytes, &responseObject)
		if err != nil {
			return results, common.Errorf("unmarshal - error: %w url: %s", err, url)
		}
		results = append(results, responseObject.Result...)

		if page == 1 {
			totalPages = responseObject.Metadata.PagesTotal
		}
	}

	return results, nil
}
