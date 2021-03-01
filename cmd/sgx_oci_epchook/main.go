// Copyright 2021 Intel Corporation. All Rights Reserved.
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

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cilium/ebpf"
	v2 "github.com/containers/common/pkg/cgroupv2"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

const (
	configJSON    = "config.json"
	sgxEpcSizeEnv = "SGX_EPC_SIZE"

	annotationName  = "com.intel.sgx.map"
	annotationValue = "container_sgx_epc_limit"

	cgroupFsPath  = "/sys/fs/cgroup"
	bpfFsPath     = "/sys/fs/bpf"
	kubepodsSlice = "kubepods.slice"
)

// Stdin defines structure for standard JSONed input of the OCI platform hook.
type Stdin struct {
	Annotations struct {
		ComIntelSgxBpfMap string `json:"com.intel.sgx.map"`
	} `json:"annotations"`
	Bundle string `json:"bundle"`
}

// Config defines structure of OCI hook configuration.
type Config struct {
	Process struct {
		Env []string `json:"env"`
	} `json:"process"`
	Linux struct {
		CgroupsPath string `json:"cgroupsPath"`
	} `json:"linux"`
}

func decodeJSONStream(reader io.Reader, dest interface{}) error {
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&dest)
	return errors.WithStack(err)
}

type hookEnv struct {
	config  string
	cleanup bool
}

func newHookEnv(config string, cleanup bool) *hookEnv {
	return &hookEnv{
		config:  config,
		cleanup: cleanup,
	}
}

func getID(path string) uint64 {
	h, _, err := unix.NameToHandleAt(unix.AT_FDCWD, path, 0)
	if err != nil {
		return 0
	}

	return binary.LittleEndian.Uint64(h.Bytes())
}

func (he *hookEnv) getConfig(stdinJ *Stdin) (*Config, error) {
	configPath := filepath.Join(stdinJ.Bundle, he.config)
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer configFile.Close()

	var config Config
	err = decodeJSONStream(configFile, &config)
	if err != nil {
		return nil, errors.WithMessage(err, "can't decode "+configPath)
	}

	if len(config.Process.Env) == 0 {
		return nil, errors.Errorf("%s: process.env is empty", configPath)
	}

	if config.Linux.CgroupsPath == "" {
		return nil, errors.Errorf("%s: linux.cgroupsPath is not set", configPath)
	}
	return &config, nil
}

func (he *hookEnv) getSgxMapParams(config *Config) (uint64, uint64, []byte, error) {
	cgroupV2Path := cgroupFsPath

	if enabled, err := v2.Enabled(); err == nil {
		if !enabled {
			cgroupV2Path = filepath.Join(cgroupFsPath, "unified")
		}
	}

	podQOSClassRE := regexp.MustCompile(`^kubepods-(besteffort)|(burstable)|(guaranteed)`)

	QoS := podQOSClassRE.FindAllString(config.Linux.CgroupsPath, 1)

	var dir string

	if len(QoS) != 0 {
		dir = fmt.Sprintf("kubepods-%s.slice", QoS[0])
	}

	containerInfo := strings.SplitN(config.Linux.CgroupsPath, ":", 3)

	containerPath := filepath.Join(
		cgroupV2Path,
		kubepodsSlice,
		dir,
		containerInfo[0],
		fmt.Sprintf("%s-%s.scope", containerInfo[1], containerInfo[2]),
	)

	id := getID(containerPath)
	if id == 0 && !he.cleanup {
		return 0, 0, nil, errors.Errorf("failed to get container ID for cgroupsPath (%s)", containerPath)
	}

	b := bytes.NewBufferString(containerInfo[2])

	// parse SGX_EPC_SIZE environment variable
	for _, env := range config.Process.Env {
		splitted := strings.SplitN(env, "=", 2)
		if strings.HasPrefix(splitted[0], sgxEpcSizeEnv) {
			q := resource.MustParse(splitted[1])
			if s, ok := q.AsInt64(); ok {
				return id, uint64(s), b.Bytes()[:64], nil
			}
		}
	}

	return 0, 0, nil, errors.Errorf("%s* environment variable is not set", sgxEpcSizeEnv)
}

func getStdin(reader io.Reader) (*Stdin, error) {
	var stdinJ Stdin
	err := decodeJSONStream(reader, &stdinJ)
	if err != nil {
		return nil, err
	}

	// Check if device plugin annotation is set
	if stdinJ.Annotations.ComIntelSgxBpfMap == "" {
		return nil, fmt.Errorf("annotation is not set")
	}

	// Check if device plugin annotation is set
	if stdinJ.Annotations.ComIntelSgxBpfMap != annotationValue {
		return nil, fmt.Errorf("annotation %s has incorrect value '%s'", annotationName, stdinJ.Annotations.ComIntelSgxBpfMap)
	}

	if stdinJ.Bundle == "" {
		return nil, errors.New("'bundle' field is not set in the stdin JSON")
	}

	if _, err := os.Stat(stdinJ.Bundle); err != nil {
		return nil, fmt.Errorf("bundle directory %s: stat error: %+v", stdinJ.Bundle, err)
	}

	return &stdinJ, nil
}

func (he *hookEnv) process(reader io.Reader) error {
	stdin, err := getStdin(reader)
	if err != nil {
		return err
	}

	config, err := he.getConfig(stdin)
	if err != nil {
		return err
	}

	key, value, sha, err := he.getSgxMapParams(config)
	if err != nil {
		return errors.WithMessage(err, "couldn't get SGX BPF Map parameters")
	}

	m, err := ebpf.LoadPinnedMap(filepath.Join(bpfFsPath, annotationValue), &ebpf.LoadPinOptions{})
	if err != nil {
		return errors.WithMessage(err, "couldn't load pinned SGX EPC limits map")
	}

	id, err := ebpf.LoadPinnedMap(filepath.Join(bpfFsPath, "container_id_hash"), &ebpf.LoadPinOptions{})
	if err != nil {
		return errors.WithMessage(err, "couldn't load pinned container ID hash")
	}

	if he.cleanup {
		var containerID uint64

		err = id.Lookup(sha, &containerID)
		if err != nil {
			return errors.WithMessage(err, "couldn't lookup and delete container ID key")
		}
		err = m.Delete(containerID)
		if err != nil {
			return errors.WithMessage(err, "couldn't delete SGX EPC limits key")
		}
		err = id.Delete(sha)
		if err != nil {
			return errors.WithMessage(err, "couldn't delete container ID key")
		}
		klog.Info(fmt.Sprintf("Updated BPF MAP %s: key %d (delete)\n", annotationValue, key))
	} else {
		err = m.Update(key, value, ebpf.UpdateNoExist)
		if err != nil {
			return errors.WithMessage(err, "couldn't update SGX EPC limits map")
		}
		err = id.Update(sha, key, ebpf.UpdateNoExist)
		if err != nil {
			return errors.WithMessage(err, "couldn't update container ID hash map")
		}
		klog.Info(fmt.Sprintf("Add to BPF MAP %s: key %d, value: %d\n", annotationValue, key, value))
	}

	return nil
}

func init() {
	klog.InitFlags(nil)
}

func main() {
	var cleanup bool

	if os.Getenv("PATH") == "" { // runc doesn't set PATH when runs hooks
		os.Setenv("PATH", "/sbin:/usr/sbin:/usr/local/sbin:/usr/local/bin:/usr/bin:/bin")
	}
	flag.BoolVar(&cleanup, "cleanup", false, "cleanup eBPF maps")
	flag.Parse()

	he := newHookEnv(configJSON, cleanup)

	if err := he.process(os.Stdin); err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}
}
