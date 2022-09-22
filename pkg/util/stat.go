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
	"os"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/koordinator-sh/koordinator/pkg/util/system"
)

var (
	CpuacctUsageTypeStat = sets.NewString("user", "nice", "system", "irq", "softirq")
)

func readTotalCPUStat(statPath string) (uint64, error) {
	// stat usage: $user + $nice + $system + $irq + $softirq
	rawStats, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}
	stats := strings.Split(string(rawStats), "\n")
	for _, stat := range stats {
		fieldStat := strings.Fields(stat)
		if fieldStat[0] == "cpu" {
			if len(fieldStat) <= 7 {
				return 0, fmt.Errorf("%s is illegally formatted", statPath)
			}
			var total uint64 = 0
			// format: cpu $user $nice $system $idle $iowait $irq $softirq
			for _, i := range []int{1, 2, 3, 6, 7} {
				v, err := strconv.ParseUint(fieldStat[i], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("failed to parse node stat %s, err: %s", stat, err)
				}
				total += v
			}
			return total, nil
		}
	}
	return 0, fmt.Errorf("%s is illegally formatted", statPath)
}

// GetCPUStatUsageTicks returns the node's CPU usage ticks
func GetCPUStatUsageTicks() (uint64, error) {
	return readTotalCPUStat(system.ProcStatFile.File)
}

func readCPUAcctUsage(usagePath string) (uint64, error) {
	v, err := os.ReadFile(usagePath)
	if err != nil {
		return 0, err
	}

	r, err1 := strconv.ParseUint(strings.TrimSpace(string(v)), 10, 64)
	if err1 != nil {
		return 0, err1
	}
	return r, nil
}

// GetPodCPUUsage returns the pod's CPU usage in nanosecond
func GetPodCPUUsageNanoseconds(podCgroupDir string) (uint64, error) {
	podStatPath := GetPodCgroupCPUAcctProcUsagePath(podCgroupDir)
	return readCPUAcctUsage(podStatPath)
}

func GetContainerCPUUsageNanoseconds(podCgroupDir string, c *corev1.ContainerStatus) (uint64, error) {
	containerStatPath, err := GetContainerCgroupCPUAcctUsagePath(podCgroupDir, c)
	if err != nil {
		return 0, err
	}
	return readCPUAcctUsage(containerStatPath)
}

func GetRootCgroupCPUUsageNanoseconds(qosClass corev1.PodQOSClass) (uint64, error) {
	rootCgroupParentDir := GetKubeQosRelativePath(qosClass)
	statPath := system.GetCgroupFilePath(rootCgroupParentDir, system.CpuacctUsage)
	return readCPUAcctUsage(statPath)
}

func GetPodCPI(podCgroupDir string, pod *corev1.Pod) (float64, error) {
	var totalCycles, totalInstructions uint64

	for i := range pod.Status.ContainerStatuses {
		containerStat := &pod.Status.ContainerStatuses[i]
		containerCgroupFilePath, err := GetContainerCgroupPathWithKube(podCgroupDir, containerStat)
		if err != nil {
			klog.Fatal(err)
			continue
		}
		f, err := os.OpenFile(containerCgroupFilePath, os.O_RDONLY, 0755)
		if err != nil {
			klog.Fatal(err)
		}
		fd := f.Fd()
		cycles, instructions, err := getContainerCyclesAndInstructions(int(fd))
		if err != nil {
			klog.Fatal(err)
		}
		totalCycles += cycles
		totalInstructions += instructions
	}
	if totalInstructions == 0 {
		return float64(0), fmt.Errorf("get 0 instructions, cannot compute CPI in this time window")
	}

	CPI := float64(totalCycles) / float64(totalInstructions)
	klog.V(5).Infof("%d instructions, %d CPU cycles: %f CPI", totalInstructions, totalCycles, CPI)
	return CPI, nil
}

func GetContainerCyclesAndInstructions(podCgroupDir string, c *corev1.ContainerStatus) (uint64, uint64, error) {
	containerCgroupFilePath, err := GetContainerCgroupPathWithKube(podCgroupDir, c)
	if err != nil {
		return 0, 0, err
	}
	// get file descriptor for cgroup mode perf_event_open
	f, err := os.OpenFile(containerCgroupFilePath, os.O_RDONLY, 0755)
	if err != nil {
		klog.Fatal(err)
	}
	fd := f.Fd()
	cycles, instructions, err := getContainerCyclesAndInstructions(int(fd))
	if err != nil {
		klog.Fatal(err)
	}
	klog.V(5).Infof("%d instructions, %d CPU cycles", instructions, cycles)
	return cycles, instructions, nil
}
