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

import (
	"strings"

	"k8s.io/klog/v2"
)

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
			break
		}

		results = append(results, remainingString[:maxLength])
		remainingString = remainingString[maxLength:]
	}

	return results
}

// SplitAtLastAlphaNum returns the given string cut to chunks of size up to maxLength.
// Difference to the Split above, this cuts the string at the last alpha numeric character
// (a-z0-9A-Z) and adds concatChars at the beginning of the next string chunk.
func SplitAtLastAlphaNum(str string, maxLength uint, concatChars string) []string {
	remainingString := str
	results := []string{}

	if maxLength <= uint(len(concatChars)) {
		klog.Errorf("SplitAtLastAlphaNum: maxLength cannot be smaller than concatChars: %d vs %d", maxLength, uint(len(concatChars)))

		results = []string{}

		return results
	}

	isAlphaNum := func(c byte) bool {
		return c >= 'a' && c <= 'z' ||
			c >= 'A' && c <= 'Z' ||
			c >= '0' && c <= '9'
	}

	strPrefix := ""

	for len(remainingString) >= 0 {
		if uint(len(remainingString)) <= maxLength {
			results = append(results, (strPrefix + remainingString))
			break
		}

		alphaNumIndex := int(maxLength) - 1
		for alphaNumIndex >= 0 && !isAlphaNum(remainingString[alphaNumIndex]) {
			alphaNumIndex--
		}

		if alphaNumIndex < 0 {
			klog.Errorf("SplitAtLastAlphaNum: chunk without any alpha numeric characters: %s", remainingString)

			results = []string{}

			return results
		}

		// increase by one to get the actual cut index
		alphaNumIndex++

		results = append(results, strPrefix+remainingString[:alphaNumIndex])
		remainingString = remainingString[alphaNumIndex:]

		if strPrefix == "" {
			maxLength -= uint(len(concatChars))
			strPrefix = concatChars
		}
	}

	return results
}

func ConcatAlphaNumSplitChunks(chunks []string, concatChars string) string {
	if len(chunks) == 1 {
		return chunks[0]
	}

	s := chunks[0]

	for _, chunk := range chunks[1:] {
		if !strings.HasPrefix(chunk, concatChars) {
			klog.Warningf("Chunk has invalid prefix: %s (should have %s)", chunk[:len(concatChars)], concatChars)
		}

		s += chunk[len(concatChars):]
	}

	return s
}
