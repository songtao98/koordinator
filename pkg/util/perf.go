//go:build linux
// +build linux

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
	"time"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"

	"github.com/hodgesds/perf-utils"
)

const (
	AnyCPU = -1
)

func getContainerInstructionsAndCycles(cgroupFd int) (cycles, instructions uint64, err error) {
	hwProfiler, err := perf.NewHardwareProfiler(cgroupFd, AnyCPU, perf.AllHardwareProfilers, unix.PERF_FLAG_PID_CGROUP)
	if err != nil && !hwProfiler.HasProfilers() {
		klog.Fatal(err)
	}
	defer func() {
		if err := hwProfiler.Close(); err != nil {
			klog.Fatal(err)
		}
	}()

	if err := hwProfiler.Start(); err != nil {
		klog.Fatal(err)
	}

	<-time.After(10 * time.Second)
	if err := hwProfiler.Stop(); err != nil {
		klog.Fatal(err)
	}

	profile := &perf.HardwareProfile{}
	err = hwProfiler.Profile(profile)
	if err != nil {
		klog.Fatal(err)
	}
	cycles = *profile.RefCPUCycles
	instructions = *profile.Instructions
	return cycles, instructions, nil
}
