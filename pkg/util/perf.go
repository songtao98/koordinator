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
	"log"

	"golang.org/x/sys/unix"

	"github.com/hodgesds/perf-utils"
)

const (
	AnyCPU = -1
)

func computeCPIWithHardwareProfiler(cgroupFd int) (float64, error) {
	hwProfiler, err := perf.NewHardwareProfiler(cgroupFd, AnyCPU, perf.AllHardwareProfilers, unix.PERF_FLAG_PID_CGROUP)
	if err != nil && !hwProfiler.HasProfilers() {
		log.Fatal(err)
	}
	defer func() {
		if err := hwProfiler.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if err := hwProfiler.Start(); err != nil {
		log.Fatal(err)
	}

	profile := &perf.HardwareProfile{}
	err = hwProfiler.Profile(profile)
	if err != nil {
		log.Fatal(err)
	}
	cycles := profile.RefCPUCycles
	instructions := profile.Instructions
	// todo: maybe some other handle methods
	if *instructions == 0 {
		log.Fatal("get 0 instructions, cannot compute CPI in this time window")
		return float64(0), err
	}
	CPI := float64(*cycles) / float64(*instructions)
	fmt.Printf("%d instructions, %d CPU cycles: %f CPI", instructions, cycles, CPI)
	if err := hwProfiler.Stop(); err != nil {
		log.Fatal(err)
	}
	return CPI, nil
}
