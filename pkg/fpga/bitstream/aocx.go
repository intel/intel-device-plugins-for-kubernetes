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
	"bytes"
	"compress/gzip"
	"debug/elf"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	// OpenCLUUID is a special AFU UUID that is used for all OpenCL BSP based FPGA bitstreams
	OpenCLUUID        = "18b79ffa2ee54aa096ef4230dafacb5f"
	fileExtensionAOCX = ".aocx"
)

// A FileAOCX represents an open AOCX file.
type FileAOCX struct {
	AutoDiscovery          string
	AutoDiscoveryXML       string
	Board                  string
	BoardPackage           string
	BoardSpecXML           string
	CompilationEnvironment string
	Hash                   string
	KernelArgInfoXML       string
	QuartusInputHash       string
	QuartusReport          string
	Target                 string
	Version                string
	GBS                    *FileGBS
	closer                 io.Closer
	// embed common bitstream interfaces
	File
}

// OpenAOCX opens the named file using os.Open and prepares it for use as GBS.
func OpenAOCX(name string) (*FileAOCX, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ff, err := NewFileAOCX(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	ff.closer = f
	return ff, nil
}

// Close closes the FileAOCX.
// If the FileAOCX was created using NewFileAOCX directly instead of Open,
// Close has no effect.
func (f *FileAOCX) Close() (err error) {
	if f.closer != nil {
		err = f.closer.Close()
		f.closer = nil
	}
	return
}

func setSection(f *FileAOCX, section *elf.Section) error {
	name := section.SectionHeader.Name
	if name == ".acl.fpga.bin" {
		data, err := section.Data()
		if err != nil {
			return errors.Wrap(err, "unable to read .acl.fpga.bin")
		}
		f.GBS, err = parseFpgaBin(data)
		if err != nil {
			return errors.Wrap(err, "unable to parse gbs")
		}
		return nil
	}

	fieldMap := map[string]*string{
		".acl.autodiscovery":       &f.AutoDiscovery,
		".acl.autodiscovery.xml":   &f.AutoDiscoveryXML,
		".acl.board":               &f.Board,
		".acl.board_package":       &f.BoardPackage,
		".acl.board_spec.xml":      &f.BoardSpecXML,
		".acl.compilation_env":     &f.CompilationEnvironment,
		".acl.rand_hash":           &f.Hash,
		".acl.kernel_arg_info.xml": &f.KernelArgInfoXML,
		".acl.quartus_input_hash":  &f.QuartusInputHash,
		".acl.quartus_report":      &f.QuartusReport,
		".acl.target":              &f.Target,
		".acl.version":             &f.Version,
	}

	if field, ok := fieldMap[name]; ok {
		data, err := section.Data()
		if err != nil {
			return errors.Wrapf(err, "%s: unable to get section data", name)
		}
		*field = strings.TrimSpace(string(data))
	}
	return nil
}

// NewFileAOCX creates a new File for accessing an ELF binary in an underlying reader.
// The ELF binary is expected to start at position 0 in the ReaderAt.
func NewFileAOCX(r io.ReaderAt) (*FileAOCX, error) {
	el, err := elf.NewFile(r)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read header")
	}
	f := new(FileAOCX)
	for _, section := range el.Sections {
		err = setSection(f, section)
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

func parseFpgaBin(d []byte) (*FileGBS, error) {
	gb, err := elf.NewFile(bytes.NewReader(d))
	gz := gb.Section(".acl.gbs.gz")
	if gz == nil {
		return nil, errors.New("no .acl.gbs.gz section in .acl.fgpa.bin")
	}
	gzr, err := gzip.NewReader(gz.Open())
	if err != nil {
		return nil, errors.Wrap(err, "unable to open gzip reader for .acl.gbs.gz")
	}
	b, err := ioutil.ReadAll(gzr)
	if err != nil {
		return nil, errors.Wrap(err, "unable to uncompress .acl.gbs.gz")
	}
	g, err := NewFileGBS(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if afuUUID := g.AcceleratorTypeUUID(); afuUUID != OpenCLUUID {
		g.Close()
		return nil, errors.Errorf("incorrect OpenCL BSP AFU UUID (%s)", afuUUID)
	}
	return g, nil
}

// RawBitstreamReader returns Reader for raw bitstream data
func (f *FileAOCX) RawBitstreamReader() io.ReadSeeker {
	if f.GBS != nil {
		return f.GBS.Bitstream.Open()
	}
	return nil
}

// RawBitstreamData returns raw bitstream data
func (f *FileAOCX) RawBitstreamData() ([]byte, error) {
	if f.GBS != nil {
		return f.GBS.Bitstream.Data()
	}
	return nil, errors.Errorf("GBS section not found")
}

// UniqueUUID represents the unique field that identifies bitstream.
// For AOCX it is the unique Hash in the header.
func (f *FileAOCX) UniqueUUID() string {
	return f.Hash
}

// InterfaceUUID returns underlying GBS InterfaceUUID
func (f *FileAOCX) InterfaceUUID() (ret string) {
	if f.GBS != nil {
		ret = f.GBS.InterfaceUUID()
	}
	return
}

// AcceleratorTypeUUID returns underlying GBS AFU ID
func (f *FileAOCX) AcceleratorTypeUUID() (ret string) {
	if f.GBS != nil {
		ret = f.GBS.AcceleratorTypeUUID()
	}
	return
}

// InstallPath returns unique filename for bitstream relative to given directory
func (f *FileAOCX) InstallPath(root string) (ret string) {
	interfaceID := f.InterfaceUUID()
	uniqID := f.UniqueUUID()
	if interfaceID != "" && uniqID != "" {
		ret = filepath.Join(root, interfaceID, uniqID+fileExtensionAOCX)
	}
	return
}

// ExtraMetadata returns map of key/value with additional metadata that can be detected from bitstream
func (f *FileAOCX) ExtraMetadata() map[string]string {
	return map[string]string{
		"Board":   f.Board,
		"Target":  f.Target,
		"Hash":    f.Hash,
		"Version": f.Version,
		"Size":    strconv.FormatUint(f.GBS.Bitstream.Size, 10),
	}
}
