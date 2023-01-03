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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"k8s.io/klog/v2"
)

// GpuFatalErrors returns (value, name) of first GPU fatal_* error counter
// file with non-zero value, or (0, "") if there were no fatal errors.
func GpuFatalErrors(syspath string) (uint64, string) {
	for tile := 0; ; tile++ {
		path := path.Join(syspath, fmt.Sprintf("gt/gt%d", tile))

		ok, count, fname := tileFatalErrors(path)
		if count > 0 {
			return count, fname
		}

		if !ok {
			return 0, ""
		}
	}
}

// tileFatalErrors returns (true, counter value, file name) for first >0 tile
// '*fatal*' error counter value, (true, 0, "") if all counters were zero,
// and (false, 0, "") if there was an error, or no counter files matched.
func tileFatalErrors(tilepath string) (bool, uint64, string) {
	// match files like 'fatal_guc' and 'sgunit_fatal'
	paths, err := filepath.Glob(path.Join(tilepath, "error_counter/*fatal*"))
	if err != nil {
		klog.Error("Error counter glob failed: ", err)
		return false, 0, ""
	}

	if len(paths) == 0 {
		return false, 0, ""
	}

	for _, f := range paths {
		dat, err := os.ReadFile(f)
		if err != nil {
			klog.Warning("Failed to read:", f)
			return false, 0, ""
		}

		value, err := strconv.ParseUint(string(dat), 10, 64)
		if err != nil {
			klog.Warning("Failed to parse:", f)
			return false, 0, ""
		}

		if value > 0 {
			// first >0 fatal counter value
			return true, value, path.Base(f)
		}
	}

	return true, 0, ""
}
