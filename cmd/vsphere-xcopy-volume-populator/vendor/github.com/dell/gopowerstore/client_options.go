/*
 *
 * Copyright © 2020-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"github.com/dell/gopowerstore/api"
)

// ClientOptions defaults
const (
	clientOptionsDefaultInsecure     = false
	clientOptionsDefaultTimeout      = 120
	clientOptionsDefaultRateLimit    = 60
	clientOptionsDefaultRequestIDKey = "csi.requestid"
)

// NewClientOptions returns pointer to a new ClientOptions struct
func NewClientOptions() *ClientOptions {
	return &ClientOptions{}
}

// ClientOptions struct provide additional options for api client configuration
type ClientOptions struct {
	insecure       *bool // skip https cert check
	defaultTimeout *int64
	rateLimit      *int
	// define field name in context which will be used for tracing
	requestIDKey *api.ContextKey
}

// Insecure returns insecure client option
func (co *ClientOptions) Insecure() bool {
	if co.insecure == nil {
		return clientOptionsDefaultInsecure
	}
	return *co.insecure
}

// DefaultTimeout returns http client default timeout
func (co *ClientOptions) DefaultTimeout() int64 {
	if co.defaultTimeout == nil {
		return clientOptionsDefaultTimeout
	}
	return *co.defaultTimeout
}

// RateLimit returns http client rate limit
func (co *ClientOptions) RateLimit() int {
	if co.rateLimit == nil {
		return clientOptionsDefaultRateLimit
	}
	return *co.rateLimit
}

// RequestIDKey returns client requestIDKey
func (co *ClientOptions) RequestIDKey() api.ContextKey {
	if co.requestIDKey == nil {
		return clientOptionsDefaultRequestIDKey
	}
	return *co.requestIDKey
}

// SetInsecure sets insecure value
func (co *ClientOptions) SetInsecure(value bool) *ClientOptions {
	co.insecure = &value
	return co
}

// SetDefaultTimeout sets default http client timeout value
func (co *ClientOptions) SetDefaultTimeout(value int64) *ClientOptions {
	co.defaultTimeout = &value
	return co
}

// SetRateLimit returns http client rate limit
func (co *ClientOptions) SetRateLimit(value int) *ClientOptions {
	co.rateLimit = &value
	return co
}

// SetRequestIDKey sets requestIdKey value
func (co *ClientOptions) SetRequestIDKey(value api.ContextKey) *ClientOptions {
	co.requestIDKey = &value
	return co
}
