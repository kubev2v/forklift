/*
 *
 * Copyright © 2025 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

import (
	"context"

	"github.com/dell/gopowerstore/api"
)

// Alert represents a PowerStore API "alert_instance", or more familiarly,
// an alert from the PowerStore UI.
type Alert struct {
	ID        string `json:"id"`
	EventCode string `json:"event_code"`

	// Severity is one of "None", "Info", "Major", "Minor", or "Critical".
	Severity     string `json:"severity"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	Description  string `json:"description_l10n"`

	// Format: "2006-01-02T15:04:05.000000+00:00"
	GeneratedTimestamp string `json:"generated_timestamp"`

	// Format: "2006-01-02T15:04:05.000000+00:00"
	RaisedTimestamp string `json:"raised_timestamp"`

	// Format: "2006-01-02T15:04:05.000000+00:00"
	ClearedTimestamp string `json:"cleared_timestamp"`

	// State represents the active status of the alert.
	// Should be one of "ACTIVE" or "CLEARED".
	State          string  `json:"state"`
	IsAcknowledged bool    `json:"is_acknowledged"`
	Events         []Event `json:"events"`
}

type Alerts []Alert

type GetAlertsResponse struct {
	AlertsResponseMeta `json:",inline"`
	Alerts             `json:",inline"`
}

type AlertsClient interface {
	// GetAlerts returns a list of alerts. Provide optional opts to enable
	// various filters for the queried alerts.
	GetAlerts(ctx context.Context, opts GetAlertsOpts) (*GetAlertsResponse, error)
}

// GetAlertsOpts is a set of optional options that can be provided as an argument
// to the (*ClientIMPL).GetAlert function to add filters to the query.
type GetAlertsOpts struct {
	// RequestPagination provides options for paginating results by specifying the
	// page size and starting index.
	// Default page size is controlled by the PowerStore API server, and
	// default page index is 0.
	// +optional
	RequestPagination

	// Queries is a list of key-value pairs that can be used to filter the results.
	// Each key-value pair will be appended to the URL in the format, key1=value1&key2=value2, etc.
	// +optional
	Queries map[string]string
}

type AlertsResponseMeta struct {
	api.RespMeta
}

// Fields returns a list of fields that must be included in the REST request to fill the Event struct.
// Satisfies the FieldProvider interface.
func (e *Alert) Fields() []string {
	return []string{
		"id", "event_code", "severity", "resource_type", "resource_name", "description_l10n",
		"generated_timestamp", "raised_timestamp", "cleared_timestamp", "state", "is_acknowledged",
		"events",
	}
}
