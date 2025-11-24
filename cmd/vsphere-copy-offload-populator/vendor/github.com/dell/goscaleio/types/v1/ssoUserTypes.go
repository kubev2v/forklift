// Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goscaleio

// SSOUserDetails represents the details of an SSO user.
type SSOUserDetails struct {
	ID               string     `json:"id"`
	Username         string     `json:"username"`
	CreatedTimestamp string     `json:"created_timestamp"`
	IsEnabled        bool       `json:"is_enabled"`
	FirstName        string     `json:"first_name"`
	LastName         string     `json:"last_name"`
	EmailAddress     string     `json:"email_address"`
	IsBuiltin        bool       `json:"is_builtin"`
	Type             string     `json:"type"`
	Permission       Permission `json:"permission"`
}

// SSOUserList represents the details of an SSO users.
type SSOUserList struct {
	SSOUsers []SSOUserDetails `json:"users"`
}

// Permission represents a permission that can be granted to an SSO user.
type Permission struct {
	Role   string  `json:"role"`
	Scopes []Scope `json:"scopes"`
}

// Scope represents a scope that can be granted to an SSO user.
type Scope struct {
	ScopeID   string `json:"scope_id"`
	ScopeType string `json:"scope_type"`
}

// SSOUserCreateParam represents the parameters for creating an SSO user.
type SSOUserCreateParam struct {
	UserName  string `json:"username"`
	Role      string `json:"role"`
	Password  string `json:"password"`
	Type      string `json:"type"`
	IsEnabled bool   `json:"is_enabled,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// SSOUserModifyParam represents the parameters for modifying an SSO user.
type SSOUserModifyParam struct {
	UserName  string `json:"username,omitempty"`
	Role      string `json:"role,omitempty"`
	Password  string `json:"password,omitempty"`
	IsEnabled bool   `json:"is_enabled,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}
