/*
Copyright 2026 Infinidat
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

package common

import (
	"fmt"
	"runtime"
)

func Errorf(format string, a ...any) error {
	_, file, line, ok := runtime.Caller(1) // '1' skips the current function
	if !ok {
		file = "???"
		line = 0
	}
	// Format the message with location info
	format = fmt.Sprintf("%s:%d: %s", file, line, format)
	return fmt.Errorf(format, a...)
}
