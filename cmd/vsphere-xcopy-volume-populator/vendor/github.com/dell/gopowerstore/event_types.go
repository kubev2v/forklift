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

// Event represents a PowerStore API "event".
type Event struct {
	ID        string `json:"id"`
	EventCode string `json:"event_code"`

	// One of "None", "Info", "Minor", "Major", "Critical"
	Severity     string `json:"severity"`
	ResourceName string `json:"resource_name"`
	ResourceType string `json:"resource_type"`
	Description  string `json:"description_l10n"`

	// Format: "2006-01-02T15:04:05.000000+00:00"
	Timestamp string `json:"generated_timestamp"`
}

type Events []Event

type EventsResponseMeta struct {
	api.RespMeta
}

type GetEventsResponse struct {
	EventsResponseMeta
	Events
}

type EventsClient interface {
	// GetEvents returns a list of events. Provide optional opts to enable various filters
	// for the queried events.
	GetEvents(ctx context.Context, opts GetEventsOpts) (*GetEventsResponse, error)
}

// GetEventsOpts is a set of optional options that may be provided as an argument
// to the (*ClientIMPL).GetEvents function to add filters to the query.
type GetEventsOpts struct {
	// RequestPagination provides options for paginating results by specifying the
	// page size and starting index.
	// Default page size is controlled by the PowerStore API server, and
	// default page index is 0.
	RequestPagination
	// Queries is a list of key-value pairs that can be used to filter the results.
	// Each key-value pair will be appended to the URL in the format, key1=value1&key2=value2, etc.
	// +optional
	Queries map[string]string
}

// Fields returns a list of fields that must be included in the REST request to fill the Event struct.
// Satisfies the FieldProvider interface.
func (e *Event) Fields() []string {
	return []string{"id", "event_code", "severity", "resource_name", "description_l10n", "generated_timestamp"}
}
