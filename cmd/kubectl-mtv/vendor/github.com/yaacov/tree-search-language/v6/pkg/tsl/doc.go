// Copyright 2018 Yaacov Zamir <kobi.zamir@gmail.com>
// and other contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tsl describe and parse the Tree Search Language (TSL),
// TSL is a wonderful search language, With similar grammar to SQL's
// where part.
//
// TSL grammar examples:
//
//	"name = 'joe' or name = 'jane'"
//	"city in ('paris', 'rome', 'milan') and state != 'spane'"
//	"pages between 100 and 200 and author.name ~= 'Hilbert'"
//
// The package provide the ParseTSL method to convert TSL string into TSL tree.
//
// Usage:
//
//	// Parse input string into a TSL tree.
//	tree, err := tsl.ParseTSL("user.name like 'Joe'")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// TSL tree can be used to generate SQL and MongoDB query filters, see the walkers
// package for TSL tree walking examples.
//
// https://pkg.go.dev/github.com/yaacov/tree-search-language/v6/pkg/walkers
package tsl
