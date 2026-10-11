/*
 *
 * Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *      http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package gopowerstore

import (
	"net/url"
	"strings"
)

// BuildMIPURL constructs a PowerStore REST API base URL from a management IP
// or hostname. It handles IPv6 addresses correctly by wrapping them in brackets
// as required by RFC 3986 §3.2.2.
//
// For IPv6 MIPs (e.g. "2001:db8::1") it produces "https://[2001:db8::1]/api/rest".
// For IPv4 MIPs (e.g. "192.168.1.1") it produces "https://192.168.1.1/api/rest".
// For FQDNs (e.g. "powerstore.example.com") it passes the hostname through unchanged.
// Handles pre-bracketed IPv6 input (e.g. "[2001:db8::1]") by stripping brackets first
// to prevent double-bracketing.
func BuildMIPURL(mip string) string {
	// Strip any pre-existing brackets so we never produce double brackets
	// (e.g. if the caller passes "[2001:db8::1]" we must not emit "[[…]]").
	bare := strings.Trim(mip, "[]")
	host := bare
	// IPv6 addresses contain colons; RFC 3986 requires them to be wrapped in
	// brackets when used as the host component of a URI.
	if strings.Contains(bare, ":") {
		host = "[" + bare + "]"
	}
	u := &url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/api/rest",
	}
	return u.String()
}
