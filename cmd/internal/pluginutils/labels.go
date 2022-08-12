// Copyright 2022 Intel Corporation. All Rights Reserved.
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

package pluginutils

// Split returns the given string cut to chunks of size up to maxLength size.
// maxLength refers to the max length of the strings in the returned slice.
// If the whole input string fits under maxLength, it is not split.
// Split("foo_bar", 4) returns []string{"foo_", "bar"}.
func Split(str string, maxLength uint) []string {
	remainingString := str
	results := []string{}

	for len(remainingString) >= 0 {
		if uint(len(remainingString)) <= maxLength {
			results = append(results, remainingString)
			return results
		}

		results = append(results, remainingString[:maxLength])
		remainingString = remainingString[maxLength:]
	}

	return results
}
