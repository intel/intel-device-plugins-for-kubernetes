// Copyright 2020-2021 Intel Corporation. All Rights Reserved.
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
	"testing"
)

func TestSplitAlphaNumeric(t *testing.T) {
	type testData struct {
		label      string
		prefix     string
		expStrings []string
		maxLength  int
	}

	tds := []testData{
		{
			"0.0-1.0_0.1-1.1_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.1-1.1_1.0-0.0_1.1-0.1_1.0-0.0_1.1-0.1",
			"Z",
			[]string{
				"0.0-1.0_0.1-1.1_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0",
				"Z_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.1-1.1_1.0-0.0_1.1-0",
				"Z.1_1.0-0.0_1.1-0.1",
			},
			63,
		},
		{
			"0.0-1.0_0.1-1.1_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.1-1.1_1.0-0.0_1.1-0.1_1.0-0.0_1.1-0.1",
			"ZZZ",
			[]string{
				"0.0-1.0_0.1",
				"ZZZ-1.1_0.1",
				"ZZZ-1.1_0.0",
				"ZZZ-1.0_0.1",
				"ZZZ-1.1_0.0",
				"ZZZ-1.0_0.1",
				"ZZZ-1.1_0.0",
				"ZZZ-1.0_0.1",
				"ZZZ-1.1_0.0",
				"ZZZ-1.0_0.1",
				"ZZZ-1.1_0.0",
				"ZZZ-1.0_0.1",
				"ZZZ-1.1_0.1",
				"ZZZ-1.1_1.0",
				"ZZZ-0.0_1.1",
				"ZZZ-0.1_1.0",
				"ZZZ-0.0_1.1",
				"ZZZ-0.1",
			},
			12,
		},
		{
			"0.0-1.0_0.1-1.1_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-15.0_0.1",
			"X",
			[]string{
				"0.0-1.0_0.1-1.1_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1.1_0.0-15",
				"X.0_0.1",
			},
			63,
		},
		{
			"0.0-1.0_0.1-1.1_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1._-_._-_..0_0.1",
			"XYZ",
			[]string{
				"0.0-1.0_0.1-1.1_0.1-1.1_0.0-1.0_0.1-1.1_0.0-1.0_0.1-1",
				"XYZ._-_._-_..0_0.1",
			},
			63,
		},
		{
			"A___B____C",
			"Z",
			[]string{},
			4,
		},
		{
			"A___B____C",
			"ZYYYYYYZZZZZ",
			[]string{},
			4,
		},
	}

	for _, td := range tds {
		res := SplitAtLastAlphaNum(td.label, uint(td.maxLength), td.prefix)

		if len(res) != len(td.expStrings) {
			t.Errorf("Got invalid amount of string chunks: %d", len(res))
		}

		for i, s := range td.expStrings {
			if res[i] != s {
				t.Errorf("Invalid chunk from split %s (vs. %s)", res[i], s)
			}

			if len(res[i]) > td.maxLength {
				t.Errorf("Chunk is too long %d (vs. %d)", len(res[i]), td.maxLength)
			}
		}
	}

	for _, td := range tds[:4] {
		res := ConcatAlphaNumSplitChunks(td.expStrings, td.prefix)

		if res != td.label {
			t.Errorf("Invalid concatenated string: %s vs. %s", res, td.label)
		}
	}
}
