// Copyright 2019 Intel Corporation. All Rights Reserved.
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

package bitstream

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// GetFPGABitstream scans bitstream storage and returns first found bitstream by region and afu id
func GetFPGABitstream(bitstreamDir, region, afu string) (File, error) {
	bitstreamPath := ""
	// Temporarily only support gbs bitstreams
	// for _, ext := range []string{".gbs", ".aocx"} {
	for _, ext := range []string{".gbs", ".aocx"} {
		bitstreamPath = filepath.Join(bitstreamDir, region, afu+ext)

		_, err := os.Stat(bitstreamPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.Errorf("%s: stat error: %v", bitstreamPath, err)
		}

		return Open(bitstreamPath)
	}
	return nil, errors.Errorf("%s/%s: bitstream not found", region, afu)
}

// Open bitstream file, detecting type based on the filename extension.
func Open(fname string) (File, error) {
	switch filepath.Ext(fname) {
	case ".gbs":
		return OpenGBS(fname)
	case ".aocx":
		return OpenAOCX(fname)
	}
	return nil, errors.Errorf("unsupported file format %s", fname)
}
