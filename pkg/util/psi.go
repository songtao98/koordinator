/*
Copyright 2022 The Koordinator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"k8s.io/klog/v2"
)

const psiLineFormat = "avg10=%f avg60=%f avg300=%f total=%d"

type PSILine struct {
	Avg10  float64
	Avg60  float64
	Avg300 float64
	Total  uint64
}

type PSIStats struct {
	Some *PSILine
	Full *PSILine
}

func ReadPSI(pressurePath string) (map[string]float64, error) {
	defer func() {
		if e := recover(); e != nil {
			klog.Warningf("ReadPSI panic: %v, filePath: %s", e, pressurePath)
		}
	}()
	result := map[string]float64{}
	cpuPressure, err := os.ReadFile(path.Join(pressurePath, "/cpu.pressure"))
	if err != nil {
		return result, err
	}
	cpuStats, err := parsePSIStats(bytes.NewReader(cpuPressure))
	if err != nil {
		return result, err
	}
	result["SomeCPUAvg10"] = (*cpuStats.Some).Avg10
	result["FullMemAvg10"] = 0
	if cpuStats.Full != nil {
		result["FullCPUAvg10"] = (*cpuStats.Full).Avg10
	}
	memPressure, err := os.ReadFile(path.Join(pressurePath, "/memory.pressure"))
	if err != nil {
		return result, err
	}
	memStats, err := parsePSIStats(bytes.NewReader(memPressure))
	if err != nil {
		return result, err
	}
	result["SomeMemAvg10"] = (*memStats.Some).Avg10
	result["FullMemAvg10"] = (*memStats.Full).Avg10
	ioPressure, err := os.ReadFile(path.Join(pressurePath, "/io.pressure"))
	if err != nil {
		return result, err
	}
	ioStats, err := parsePSIStats(bytes.NewReader(ioPressure))
	if err != nil {
		return result, err
	}
	result["SomeIOAvg10"] = (*ioStats.Some).Avg10
	result["FullIOAvg10"] = (*ioStats.Full).Avg10

	klog.V(4).Infof("Pressure file contents from : %s, cpu %s, mem %s, io %s", pressurePath, string(cpuPressure), string(memPressure), string(ioPressure))
	return result, nil
}

// parsePSIStats parses the specified file for pressure stall information.
func parsePSIStats(r io.Reader) (PSIStats, error) {
	psiStats := PSIStats{}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l := scanner.Text()
		prefix := strings.Split(l, " ")[0]
		switch prefix {
		case "some":
			psi := PSILine{}
			_, err := fmt.Sscanf(l, fmt.Sprintf("some %s", psiLineFormat), &psi.Avg10, &psi.Avg60, &psi.Avg300, &psi.Total)
			if err != nil {
				return PSIStats{}, err
			}
			psiStats.Some = &psi
		case "full":
			psi := PSILine{}
			_, err := fmt.Sscanf(l, fmt.Sprintf("full %s", psiLineFormat), &psi.Avg10, &psi.Avg60, &psi.Avg300, &psi.Total)
			if err != nil {
				return PSIStats{}, err
			}
			psiStats.Full = &psi
		default:
			return PSIStats{}, fmt.Errorf("parsePSIStats failed with wrong prefix")
		}
	}

	return psiStats, nil
}
