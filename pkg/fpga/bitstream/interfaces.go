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
//
// +build linux
//

package bitstream

import "io"

// File defines interfaces that are common for all supported bitstream file formats
// It should provide mechanisms to get raw bitstream data as a reader or as a byte array
// as well as mechanisms to identify bitstreams
type File interface {
	io.Closer
	// RawBitstreamReader returns Reader for raw bitstream data
	RawBitstreamReader() io.ReadSeeker
	// RawBitstreamData returns raw bitstream byte array
	RawBitstreamData() ([]byte, error)
	// InterfaceUUID returns bitstream's Interface UUID
	InterfaceUUID() string
	// AcceleratorTypeUUID returns bitstream's AFU UUID
	AcceleratorTypeUUID() string
	// UniqueUUID returns UUID that uniquely identifies bitstream
	UniqueUUID() string
	// InstallPath returns unique filename for bitstream relative to given directory
	InstallPath(string) string
	// ExtraMetadata returns map of key/value with additional metadata that can be detected from bitstream
	ExtraMetadata() map[string]string
}
