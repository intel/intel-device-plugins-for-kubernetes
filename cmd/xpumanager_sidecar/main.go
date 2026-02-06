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

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path"
	"reflect"
	"strconv"
	"syscall"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/pluginutils"
	"k8s.io/klog/v2"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

type invalidEntryErr struct{}

const (
	labelMaxLength        = 63
	xeLinkLabelName       = "xe-links"
	pureXeLinkMetricValue = 1
	labelControlChar      = "Z"
)

type xpuManagerTopologyMatrixCell struct {
	LocalDeviceID     int
	LocalSubdeviceID  int
	RemoteDeviceID    int
	RemoteSubdeviceID int
	LaneCount         int
}

type xpuManagerSidecar struct {
	getMetricsData          func() []byte
	tmpDirPrefix            string
	dstFilePath             string
	labelNamespace          string
	url                     string
	certFile                string
	interval                uint64
	startDelay              uint64
	xpumPort                uint64
	laneCount               uint64
	allowSubdevicelessLinks bool
}

func (e *invalidEntryErr) Error() string {
	return "metrics entry was invalid for our use"
}

func (xms *xpuManagerSidecar) getMetricsDataFromXPUM() []byte {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	if len(xms.certFile) > 0 {
		cert, err := os.ReadFile(xms.certFile)
		if err != nil {
			klog.Warning("Failed to read cert: ", err)

			return nil
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(cert) {
			klog.Warning("Adding server cert to pool failed")

			return nil
		}

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    certPool,
				ServerName: "127.0.0.1",
			},
		}

		client.Transport = tr
	}

	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, xms.url, http.NoBody)
	if err != nil {
		klog.Error(err.Error())

		return nil
	}

	res, err := client.Do(req)
	if err != nil {
		klog.Error(err.Error())

		return nil
	}

	resBody, err := io.ReadAll(res.Body)

	defer res.Body.Close()

	if err != nil {
		klog.Error(err.Error())

		return nil
	}

	// Seems /metrics doesn't add new-line at the end of the last entry
	// and without this there is an error from TextParser
	resBody = append(resBody, "\n"...)

	return resBody
}

func processMetricsLabels(labels []*io_prometheus_client.LabelPair, allowNonSubdeviceLinks bool) (xpuManagerTopologyMatrixCell, error) {
	cell := createInvalidTopologyCell()

	for _, label := range labels {
		name := label.GetName()
		strVal := label.GetValue()

		klog.V(5).Info(name, " ", strVal)

		// xelinks should always be on subdevices
		if !allowNonSubdeviceLinks && name == "local_on_subdevice" && strVal != "true" {
			return cell, &invalidEntryErr{}
		}

		valInt, err := strconv.Atoi(strVal)
		if err != nil {
			continue
		}

		switch name {
		case "local_device_id":
			cell.LocalDeviceID = valInt
		case "local_subdevice_id":
			cell.LocalSubdeviceID = valInt
		case "remote_device_id":
			cell.RemoteDeviceID = valInt
		case "remote_subdevice_id":
			cell.RemoteSubdeviceID = valInt
		case "lane_count":
			fallthrough
		case "lan_count":
			cell.LaneCount = valInt
		}
	}

	if !isValidTopologyCell(&cell) {
		return cell, &invalidEntryErr{}
	}

	return cell, nil
}

func (xms *xpuManagerSidecar) GetTopologyFromXPUMMetrics(data []byte) (topologyInfos []xpuManagerTopologyMatrixCell) {
	reader := bytes.NewReader(data)

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(reader)

	if err != nil {
		klog.Error(err.Error())

		return nil
	}

	for name, family := range families {
		klog.V(4).Info("parsing family: " + name)

		if name != "xpum_topology_link" {
			klog.V(5).Info("... skipping")

			continue
		}

		for _, metric := range family.Metric {
			value := -1.0

			klog.V(5).Info(metric)

			if metric.Gauge != nil {
				klog.V(5).Info("metric is of type gauge")

				value = *metric.Gauge.Value
			} else if metric.Untyped != nil {
				klog.V(5).Info("metric is of type untyped")

				value = *metric.Untyped.Value
			} else {
				klog.Warningf("Unknown/unsupported metric type: %v", metric)
			}

			if value != pureXeLinkMetricValue {
				klog.V(5).Info("... not xelink")

				continue
			}

			cell, err := processMetricsLabels(metric.Label, xms.allowSubdevicelessLinks)
			if err == nil {
				klog.V(5).Info("topology entry: ", cell)
				topologyInfos = append(topologyInfos, cell)
			}
		}
	}

	klog.V(5).Info("topology entries: ", len(topologyInfos))

	return topologyInfos
}

func (xms *xpuManagerSidecar) iterate() {
	metricsData := xms.getMetricsData()
	topologyInfos := xms.GetTopologyFromXPUMMetrics(metricsData)

	labels := xms.createLabels(topologyInfos)

	if !xms.compareLabels(labels) {
		xms.writeLabels(labels)
	} else {
		klog.V(2).Info("labels have not changed")
	}
}

// TODO: Move this function under internal/pluginutils.
func (xms *xpuManagerSidecar) writeLabels(labels []string) {
	root, err := os.MkdirTemp(xms.tmpDirPrefix, "xpumsidecar")
	if err != nil {
		klog.Errorf("can't create temporary directory: %+v", err)

		return
	}

	defer os.RemoveAll(root)

	filePath := path.Join(root, "xpum-sidecar-labels.txt")

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		klog.Error(err.Error())
	}

	writer := bufio.NewWriter(file)

	for _, label := range labels {
		_, _ = writer.WriteString(label + "\n")
	}

	writer.Flush()
	file.Close()

	// move tmp file to dst file
	err = os.Rename(filePath, xms.dstFilePath)
	if err != nil {
		klog.Errorf("Failed to rename tmp file to dst file: %+v", err)

		return
	}

	for _, label := range labels {
		klog.V(2).Infof("%v\n", label)
	}
}

// compareLabels returns true, if the labels at dstFilePath are equal to given labels.
func (xms *xpuManagerSidecar) compareLabels(labels []string) bool {
	file, err := os.OpenFile(xms.dstFilePath, os.O_RDONLY, 0644)
	if err != nil {
		return false
	}

	fileLabels := []string{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		fileLabels = append(fileLabels, scanner.Text())
	}

	return reflect.DeepEqual(fileLabels, labels)
}

func createInvalidTopologyCell() xpuManagerTopologyMatrixCell {
	cell := xpuManagerTopologyMatrixCell{}

	cell.LaneCount = -1
	cell.LocalDeviceID = -1
	cell.LocalSubdeviceID = -1
	cell.RemoteDeviceID = -1
	cell.RemoteSubdeviceID = -1

	return cell
}

func isValidTopologyCell(cell *xpuManagerTopologyMatrixCell) bool {
	return (cell.LaneCount >= 0 && cell.LocalDeviceID >= 0 &&
		cell.LocalSubdeviceID >= 0 && cell.RemoteDeviceID >= 0 &&
		cell.RemoteSubdeviceID >= 0)
}

func (xms *xpuManagerSidecar) createLabels(topologyInfos []xpuManagerTopologyMatrixCell) []string {
	links := ""
	separator := ""

	submitted := map[string]int{}

	cellToString := func(cell xpuManagerTopologyMatrixCell) string {
		if cell.LocalDeviceID < cell.RemoteDeviceID {
			return strconv.Itoa(cell.LocalDeviceID) + "." + strconv.Itoa(cell.LocalSubdeviceID) + "-" +
				strconv.Itoa(cell.RemoteDeviceID) + "." + strconv.Itoa(cell.RemoteSubdeviceID)
		}

		return strconv.Itoa(cell.RemoteDeviceID) + "." + strconv.Itoa(cell.RemoteSubdeviceID) + "-" +
			strconv.Itoa(cell.LocalDeviceID) + "." + strconv.Itoa(cell.LocalSubdeviceID)
	}

	for _, ti := range topologyInfos {
		if ti.LaneCount < int(xms.laneCount) {
			continue
		}

		linkString := cellToString(ti)

		count, found := submitted[linkString]
		if !found {
			links += separator + linkString
			separator = "_"
		}

		count++

		if count > 2 {
			klog.Warningf("Duplicate links found for: %s (lane count: %d)", linkString, ti.LaneCount)
		}

		submitted[linkString] = count
	}

	splitLinks := pluginutils.SplitAtLastAlphaNum(links, labelMaxLength, labelControlChar)

	labels := []string{}

	if len(splitLinks) == 0 {
		return labels
	}

	labels = append(labels, xms.labelNamespace+"/"+xeLinkLabelName+"="+splitLinks[0])
	for i := 1; i < len(splitLinks); i++ {
		labels = append(labels, xms.labelNamespace+"/"+xeLinkLabelName+strconv.FormatInt(int64(i+1), 10)+"="+splitLinks[i])
	}

	return labels
}

func createXPUManagerSidecar() *xpuManagerSidecar {
	xms := xpuManagerSidecar{}

	xms.getMetricsData = xms.getMetricsDataFromXPUM

	return &xms
}

func main() {
	xms := createXPUManagerSidecar()

	flag.Uint64Var(&xms.interval, "interval", 10, "interval for topology fetching and label writing (seconds, >= 1)")
	flag.Uint64Var(&xms.startDelay, "startup-delay", 10, "startup delay for first topology fetching and label writing (seconds, >= 0)")
	flag.Uint64Var(&xms.xpumPort, "xpum-port", 29999, "xpumanager port number")
	flag.StringVar(&xms.tmpDirPrefix, "tmp-dir-prefix", "/etc/kubernetes/node-feature-discovery/features.d/", "location prefix for a temporary directory (used to store in-flight label file)")
	flag.StringVar(&xms.dstFilePath, "dst-file-path", "/etc/kubernetes/node-feature-discovery/features.d/xpum-sidecar-labels.txt", "label file destination")
	flag.Uint64Var(&xms.laneCount, "lane-count", 4, "minimum lane count for xelink")
	flag.StringVar(&xms.labelNamespace, "label-namespace", "gpu.intel.com", "namespace for the labels")
	flag.BoolVar(&xms.allowSubdevicelessLinks, "allow-subdeviceless-links", false, "allow xelinks that are not tied to subdevices (=1 tile GPUs)")
	flag.StringVar(&xms.certFile, "cert", "", "Use HTTPS and verify server's endpoint")
	klog.InitFlags(nil)

	flag.Parse()

	if xms.interval == 0 {
		klog.Fatal("zero interval won't work, set it to at least 1")
	}

	protocol := "http"

	if len(xms.certFile) > 0 {
		protocol = "https"
	}

	xms.url = fmt.Sprintf("%s://127.0.0.1:%d/metrics", protocol, xms.xpumPort)

	keepIterating := true

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	time.Sleep(time.Duration(xms.startDelay) * time.Second)

	for keepIterating {
		xms.iterate()

		timeout := time.After(time.Duration(xms.interval) * time.Second)

		select {
		case <-timeout:
			continue
		case <-c:
			klog.V(2).Info("Interrupt received")

			keepIterating = false
		}
	}

	klog.V(2).Info("Removing label file")

	err := os.Remove(xms.dstFilePath)
	if err != nil {
		klog.Errorf("Failed to cleanup label file: %+v", err)
	}

	klog.V(2).Info("Stopping sidecar")
}
