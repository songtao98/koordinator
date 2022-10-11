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

// todo: add readme
package performance

import (
	"time"

	"github.com/hodgesds/perf-utils"
	"golang.org/x/sys/unix"
)

type perfCollector struct {
	collectTimeWindow int
	cgroupFd          int
	cpus              []int
	cpuHwProfilersMap map[int]*perf.HardwareProfiler
	// todo: cpuSwProfilers map[int]*performance.SoftwareProfiler
}

func NewPerfCollector(cgroupFd int, cpus []int, collectTimeWindow int) (*perfCollector, error) {
	collector := &perfCollector{
		collectTimeWindow: collectTimeWindow,
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
	timer := time.NewTicker(time.Duration(c.collectTimeWindow) * time.Second)
	defer timer.Stop()

	// do profile only once per Run()
	select {
	case <-timer.C:
		for _, cpu := range c.cpus {
			// todo: c.swProfile, etc.
			profile, err := c.hwProfileOnSingleCPU(cpu)
			if profile == nil {
				return result, err
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

// todo: call collect() to get all metrics at the same time instead of put it inside GetContainerCyclesAndInstructions
func GetContainerCyclesAndInstructions(cgroupFd int, cpus []int, collectTimeWindow int) (uint64, uint64, error) {
	collector, err := NewPerfCollector(cgroupFd, cpus, collectTimeWindow)
	if err != nil {
		return 0, 0, err
	}
	result, err := collector.startAndCollect()
	if err != nil {
		return 0, 0, err
	}
	return result.instructions, result.cycles, nil
}
