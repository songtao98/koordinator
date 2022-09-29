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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hodgesds/perf-utils"
	"golang.org/x/sys/unix"
)

type perfCollector struct {
	cgroupFd          int
	cpus              []int
	cpuHwProfilersMap map[int]*perf.HardwareProfiler
	// todo: cpuSwProfilers map[int]*perf.SoftwareProfiler
}

func NewPerfCollector(cgroupFd int, cpus []int) (*perfCollector, error) {
	collector := &perfCollector{
		cgroupFd:          cgroupFd,
		cpus:              cpus,
		cpuHwProfilersMap: map[int]*perf.HardwareProfiler{},
	}
	for _, cpu := range cpus {
		cpiProfiler, err := perf.NewHardwareProfiler(cgroupFd, cpu, perf.RefCpuCyclesProfiler&perf.CpuInstrProfiler, unix.PERF_FLAG_PID_CGROUP)
		if err != nil && !cpiProfiler.HasProfilers() {
			return nil, err
		}

		// todo: NewSoftwareProfiler, etc.

		collector.cpuHwProfilersMap[cpu] = &cpiProfiler
	}
	return collector, nil
}

type collectResult struct {
	cycles       uint64
	instructions uint64

	// todo: context-switches, etc.
}

func (c *perfCollector) startAndCollect() (result collectResult, err error) {
	for _, cpu := range c.cpus {
		if err = (*c.cpuHwProfilersMap[cpu]).Start(); err != nil {
			return
		}
	}
	// todo: does software metrics collect fits the 10 seconds ticker logic? notice that this may leads to different function structure
	timer := time.NewTicker(10 * time.Second)
	defer timer.Stop()

	// do profile only once per Run()
	select {
	case <-timer.C:
		for _, cpu := range c.cpus {
			// todo: c.swProfile, etc.
			profile, err := c.hwProfileOnSingleCPU(cpu)
			if err != nil {
				continue
			}
			// skip not counted cases
			if profile.RefCPUCycles != nil {
				result.cycles += *profile.RefCPUCycles
			}
			if profile.Instructions != nil {
				result.instructions += *profile.Instructions
			}
		}
	}
	return
}

func (c *perfCollector) hwProfileOnSingleCPU(cpu int) (*perf.HardwareProfile, error) {
	profile := &perf.HardwareProfile{}
	if err := (*c.cpuHwProfilersMap[cpu]).Profile(profile); err != nil {
		return nil, err
	}
	if err := (*c.cpuHwProfilersMap[cpu]).Stop(); err != nil {
		return profile, err
	}
	if err := (*c.cpuHwProfilersMap[cpu]).Close(); err != nil {
		return profile, err
	}
	return profile, nil
}

// todo: call collect() to get all metrics at the same time instead of put it inside getContainerCyclesAndInstructions
func getContainerCyclesAndInstructions(cgroupFd int, cpus []int) (uint64, uint64, error) {
	collector, err := NewPerfCollector(cgroupFd, cpus)
	if err != nil {
		return 0, 0, err
	}
	result, err := collector.startAndCollect()
	if err != nil {
		return 0, 0, err
	}
	return result.instructions, result.cycles, nil
}

// perfCPUFlagToCPUs returns a set of CPUs for the perf collectors to monitor.
func perfCPUFlagToCPUs(cpuFlag string) ([]int, error) {
	var err error
	cpus := []int{}
	for _, subset := range strings.Split(cpuFlag, ",") {
		// First parse a single CPU.
		if !strings.Contains(subset, "-") {
			cpu, err := strconv.Atoi(subset)
			if err != nil {
				return nil, err
			}
			cpus = append(cpus, cpu)
			continue
		}

		stride := 1
		// Handle strides, ie 1-10:5 should yield 1,5,10
		strideSet := strings.Split(subset, ":")
		if len(strideSet) == 2 {
			stride, err = strconv.Atoi(strideSet[1])
			if err != nil {
				return nil, err
			}
		}

		rangeSet := strings.Split(strideSet[0], "-")
		if len(rangeSet) != 2 {
			return nil, fmt.Errorf("invalid flag value %q", cpuFlag)
		}
		start, err := strconv.Atoi(rangeSet[0])
		if err != nil {
			return nil, err
		}
		end, err := strconv.Atoi(rangeSet[1])
		if err != nil {
			return nil, err
		}
		for i := start; i <= end; i += stride {
			cpus = append(cpus, i)
		}
	}

	return cpus, nil
}
