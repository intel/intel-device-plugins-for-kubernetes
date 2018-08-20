// Copyright 2018 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package debug

import (
	"fmt"
	"runtime"
	"strings"
)

const (
	prefix = "DEBUG"
)

var (
	isEnabled bool
)

func getFileAndLine() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "???:0"
	}
	parts := strings.Split(file, "/")
	return fmt.Sprintf("%s:%d", parts[len(parts)-1], line)
}

// Activate activates debugging output
func Activate() {
	isEnabled = true
}

// Print prints its arguments with fmt.Println() if debug output is activated
func Print(obj ...interface{}) {
	if isEnabled {
		fmt.Println(append([]interface{}{prefix, getFileAndLine()}, obj...)...)
	}
}

// Printf prints its arguments with fmt.Printf() if debug output is activated
func Printf(pattern string, obj ...interface{}) {
	if isEnabled {
		fmt.Printf(strings.Join([]string{prefix, getFileAndLine(), pattern + "\n"}, " "), obj...)
	}
}
