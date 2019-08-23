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
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	bitstreamGUID1   uint64 = 0x414750466e6f6558
	bitstreamGUID2   uint64 = 0x31303076534247b7
	fileHeaderLength        = 20
	fileExtensionGBS        = ".gbs"
)

// Header represents header struct of the GBS file
type Header struct {
	GUID1          uint64
	GUID2          uint64
	MetadataLength uint32
}

// FileGBS represents an open GBS file.
type FileGBS struct {
	Header
	Metadata  Metadata
	Bitstream *Bitstream
	closer    io.Closer
}

// Metadata represents parsed JSON metadata of GBS file
type Metadata struct {
	Version      int    `json:"version"`
	PlatformName string `json:"platform-name,omitempty"`
	AfuImage     struct {
		MagicNo         int    `json:"magic-no,omitempty"`
		InterfaceUUID   string `json:"interface-uuid,omitempty"`
		AfuTopInterface struct {
			Class       string `json:"class"`
			ModulePorts []struct {
				Params struct {
					Clock string `json:"clock,omitempty"`
				} `json:"params"`
				Optional bool   `json:"optional,omitempty"`
				Class    string `json:"class,omitempty"`
			} `json:"module-ports,omitempty"`
		} `json:"afu-top-interface"`
		Power               int         `json:"power"`
		ClockFrequencyHigh  interface{} `json:"clock-frequency-high,omitempty"`
		ClockFrequencyLow   interface{} `json:"clock-frequency-low,omitempty"`
		AcceleratorClusters []struct {
			AcceleratorTypeUUID string `json:"accelerator-type-uuid"`
			Name                string `json:"name"`
			TotalContexts       int    `json:"total-contexts"`
		} `json:"accelerator-clusters"`
	} `json:"afu-image"`
}

// A Bitstream represents a raw bitsream data (RBF) in the GBS binary
type Bitstream struct {
	Size uint64
	// Embed ReaderAt for ReadAt method.
	// Do not embed SectionReader directly
	// to avoid having Read and Seek.
	// If a client wants Read and Seek it must use
	// Open() to avoid fighting over the seek offset
	// with other clients.
	io.ReaderAt
	sr *io.SectionReader
	// embed common bitstream interfaces
	File
}

// Open returns a new ReadSeeker reading the bitsream body.
func (b *Bitstream) Open() io.ReadSeeker { return io.NewSectionReader(b.sr, 0, 1<<63-1) }

// Data reads and returns the contents of the bitstream.
func (b *Bitstream) Data() ([]byte, error) {
	dat := make([]byte, b.Size)
	n, err := io.ReadFull(b.Open(), dat)
	return dat[0:n], err
}

// OpenGBS opens the named file using os.Open and prepares it for use as GBS.
func OpenGBS(name string) (*FileGBS, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ff, err := NewFileGBS(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	ff.closer = f
	return ff, nil
}

// Close closes the FileGBS.
// If the FileGBS was created using NewFileGBS directly instead of Open,
// Close has no effect.
func (f *FileGBS) Close() (err error) {
	if f.closer != nil {
		err = f.closer.Close()
		f.closer = nil
	}
	return
}

// InterfaceUUID returns normalized Metadata.AfuImage.InterfaceUUID
func (f *FileGBS) InterfaceUUID() string {
	return strings.ToLower(strings.Replace(f.Metadata.AfuImage.InterfaceUUID, "-", "", -1))
}

// AcceleratorTypeUUID returns list of normalized AFU UUID from the metadata.
// Empty string returned in case of errors in Metadata
func (f *FileGBS) AcceleratorTypeUUID() (ret string) {
	if len(f.Metadata.AfuImage.AcceleratorClusters) == 1 {
		ret = strings.ToLower(strings.Replace(f.Metadata.AfuImage.AcceleratorClusters[0].AcceleratorTypeUUID, "-", "", -1))
	}
	return
}

// We need both Seek and ReadAt
type bitstreamReader interface {
	io.ReadSeeker
	io.ReaderAt
}

// NewFileGBS creates a new FileGBS for accessing an ELF binary in an underlying reader.
// The ELF binary is expected to start at position 0 in the ReaderAt.
func NewFileGBS(r bitstreamReader) (*FileGBS, error) {
	sr := io.NewSectionReader(r, 0, 1<<63-1)

	f := new(FileGBS)
	// 1. Read file header
	sr.Seek(0, io.SeekStart)
	if err := binary.Read(sr, binary.LittleEndian, &f.Header); err != nil {
		return nil, errors.Wrap(err, "unable to read header")
	}
	// 2. Validate Magic/GUIDs
	if f.GUID1 != bitstreamGUID1 || f.GUID2 != bitstreamGUID2 {
		return nil, errors.Errorf("wrong magic in GBS file: %#x %#x Expected %#x %#x", f.GUID1, f.GUID2, bitstreamGUID1, bitstreamGUID2)
	}
	// 3. Read/unmarshal metadata JSON
	if f.MetadataLength == 0 || f.MetadataLength >= 4096 {
		return nil, errors.Errorf("incorrect length of GBS metadata %d", f.MetadataLength)
	}
	dec := json.NewDecoder(io.NewSectionReader(r, fileHeaderLength, int64(f.MetadataLength)))
	if err := dec.Decode(&f.Metadata); err != nil {
		return nil, errors.Wrap(err, "unable to parse GBS metadata")
	}
	if afus := len(f.Metadata.AfuImage.AcceleratorClusters); afus != 1 {
		return nil, errors.Errorf("incorect length of AcceleratorClusters in GBS metadata: %d", afus)
	}
	// 4. Create bitsream struct
	b := new(Bitstream)
	// 4.1. calculate offest/size
	last, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, errors.Wrap(err, "unable to determine file size")
	}
	b.Size = uint64(last - fileHeaderLength - int64(f.MetadataLength))
	// 4.2. assign internal sr
	b.sr = io.NewSectionReader(r, int64(fileHeaderLength+f.MetadataLength), int64(b.Size))
	b.ReaderAt = b.sr
	f.Bitstream = b
	return f, nil
}

// File interfaces implementations

// RawBitstreamReader returns Reader for raw bitstream data
func (f *FileGBS) RawBitstreamReader() io.ReadSeeker {
	return f.Bitstream.Open()
}

// RawBitstreamData returns raw bitstream data
func (f *FileGBS) RawBitstreamData() ([]byte, error) {
	return f.Bitstream.Data()
}

// UniqueUUID represents the unique field that identifies bitstream.
// For GBS it is the AFU ID
func (f *FileGBS) UniqueUUID() string {
	return f.AcceleratorTypeUUID()
}

// InstallPath returns unique filename for bitstream relative to given directory
func (f *FileGBS) InstallPath(root string) (ret string) {
	interfaceID := f.InterfaceUUID()
	uniqID := f.UniqueUUID()
	if interfaceID != "" && uniqID != "" {
		ret = filepath.Join(root, interfaceID, uniqID+fileExtensionGBS)
	}
	return
}

// ExtraMetadata returns map of key/value with additional metadata that can be detected from bitstream
func (f *FileGBS) ExtraMetadata() map[string]string {
	return map[string]string{"Size": strconv.FormatUint(f.Bitstream.Size, 10)}
}
