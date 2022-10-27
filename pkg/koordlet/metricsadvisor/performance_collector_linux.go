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

package metricsadvisor

import (
	"sync"
	"time"

	"golang.org/x/sys/unix"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/koordinator-sh/koordinator/pkg/koordlet/metriccache"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	"github.com/koordinator-sh/koordinator/pkg/util"
	"github.com/koordinator-sh/koordinator/pkg/util/perf"
)

type performanceCollector struct {
	statesInformer    statesinformer.StatesInformer
	metricCache       metriccache.MetricCache
	collectTimeWindow int
}

func NewPerformanceCollector(statesInformer statesinformer.StatesInformer, metricCache metriccache.MetricCache, collectTimeWindow int) *performanceCollector {
	return &performanceCollector{
		statesInformer:    statesInformer,
		metricCache:       metricCache,
		collectTimeWindow: collectTimeWindow,
	}
}

func (c *performanceCollector) collectContainerMetrics() {
	klog.V(6).Infof("start collectContainerMetrics")
	timeWindow := time.Now()
	containerStatusesMap := map[*corev1.ContainerStatus]*statesinformer.PodMeta{}
	podMetas := c.statesInformer.GetAllPods()
	for _, meta := range podMetas {
		pod := meta.Pod
		for i := range pod.Status.ContainerStatuses {
			containerStat := &pod.Status.ContainerStatuses[i]
			containerStatusesMap[containerStat] = meta
		}
	}
	// get container CPI collectors for each container
	collectors := sync.Map{}
	var wg sync.WaitGroup
	wg.Add(len(containerStatusesMap))
	nodeCpuInfo, err := c.metricCache.GetNodeCPUInfo(&metriccache.QueryParam{})
	if err != nil {
		klog.Errorf("failed to get node cpu info : %v", err)
		return
	}
	cpuNumber := nodeCpuInfo.TotalInfo.NumberCPUs
	for containerStatus, parentPod := range containerStatusesMap {
		go func(status *corev1.ContainerStatus, parent string) {
			defer wg.Done()
			collectorOnSingleContainer, err := c.getAndStartCollectorOnSingleContainer(parent, status, cpuNumber)
			if err != nil {
				return
			}
			collectors.Store(status, collectorOnSingleContainer)
		}(containerStatus, parentPod.CgroupDir)
	}
	wg.Wait()

	time.Sleep(time.Duration(c.collectTimeWindow) * time.Second)

	// collect cpi, psi for each container at the same time
	var wg1 sync.WaitGroup
	wg1.Add(len(containerStatusesMap))
	for containerStatus, podMeta := range containerStatusesMap {
		podUid := podMeta.Pod.UID
		cgroupDir := podMeta.CgroupDir
		go func(parentDir string, status *corev1.ContainerStatus, podUid string) {
			defer wg1.Done()
			// collect container cpi
			oneCollector, ok := collectors.Load(status)
			if !ok {
				return
			}
			c.profilePerfOnSingleContainer(status, oneCollector.(*perf.PerfCollector), podUid)
			err1 := unix.Close(oneCollector.(*perf.PerfCollector).CgroupFd)
			if err1 != nil {
				klog.Errorf("close CgroupFd %v, err : %v", oneCollector.(*perf.PerfCollector).CgroupFd, err1)
			}
			// collect container psi
			c.collectSingleContainerPSI(parentDir, status, podUid)
		}(cgroupDir, containerStatus, string(podUid))
	}
	wg1.Wait()
	klog.V(5).Infof("collectContainerMetrics for time window %s finished at %s, container num %d",
		timeWindow, time.Now(), len(containerStatusesMap))
}

func (c *performanceCollector) getAndStartCollectorOnSingleContainer(podParentCgroupDir string, containerStatus *corev1.ContainerStatus, number int32) (*perf.PerfCollector, error) {
	perfCollector, err := util.GetContainerPerfCollector(podParentCgroupDir, containerStatus, number)
	if err != nil {
		klog.Errorf("get and start container %s collector err: %v", containerStatus.Name, err)
		return nil, err
	}
	return perfCollector, nil
}

func (c *performanceCollector) profilePerfOnSingleContainer(containerStatus *corev1.ContainerStatus, collector *perf.PerfCollector, podUid string) {
	collectTime := time.Now()
	cycles, instructions, err := util.GetContainerCyclesAndInstructions(collector)
	if err != nil {
		klog.Errorf("collect container %s cpi err: %v", containerStatus.Name, err)
		return
	}
	containerCpiMetric := &metriccache.ContainerInterferenceMetric{
		MetricName:  metriccache.MetricNameContainerCPI,
		PodUID:      podUid,
		ContainerID: containerStatus.ContainerID,
		MetricValue: &metriccache.CPIMetric{
			Cycles:       cycles,
			Instructions: instructions,
		},
	}
	err = c.metricCache.InsertContainerInterferenceMetrics(collectTime, containerCpiMetric)
	if err != nil {
		klog.Errorf("insert container cpi metrics failed, err %v", err)
	}
}

func (c *performanceCollector) collectSingleContainerPSI(podParentCgroupDir string, containerStatus *corev1.ContainerStatus, podUid string) {
	collectTime := time.Now()
	psiMap, err := util.GetContainerPSI(podParentCgroupDir, containerStatus)
	if err != nil {
		klog.Errorf("collect container %s psi err: %v", containerStatus.Name, err)
		return
	}
	containerPsiMetric := &metriccache.ContainerInterferenceMetric{
		MetricName:  metriccache.MetricNameContainerPSI,
		PodUID:      podUid,
		ContainerID: containerStatus.ContainerID,
		MetricValue: &metriccache.PSIMetric{
			SomeCPUAvg10: psiMap["SomeCPUAvg10"],
			SomeMemAvg10: psiMap["SomeMemAvg10"],
			SomeIOAvg10:  psiMap["SomeIOAvg10"],
			FullCPUAvg10: psiMap["FullCPUAvg10"],
			FullMemAvg10: psiMap["FullMemAvg10"],
			FullIOAvg10:  psiMap["FullIOAvg10"],
		},
	}
	err = c.metricCache.InsertContainerInterferenceMetrics(collectTime, containerPsiMetric)
	if err != nil {
		klog.Errorf("insert container psi metrics failed, err %v", err)
	}
}

func (c *performanceCollector) collectPodMetrics() {
	klog.V(6).Infof("start collectPodMetrics")
	timeWindow := time.Now()
	podMetas := c.statesInformer.GetAllPods()
	var wg sync.WaitGroup
	wg.Add(len(podMetas))
	for _, meta := range podMetas {
		pod := meta.Pod
		podCgroupDir := meta.CgroupDir
		go func(pod *corev1.Pod, podCgroupDir string) {
			defer wg.Done()
			c.collectSinglePodPSI(pod, podCgroupDir)
		}(pod, podCgroupDir)
	}
	wg.Wait()
	klog.V(5).Infof("collectPodMetrics for time window %s finished at %s, pod num %d",
		timeWindow, time.Now(), len(podMetas))
}

func (c *performanceCollector) collectSinglePodPSI(pod *corev1.Pod, podCgroupDir string) {
	collectTime := time.Now()
	psiMap, err := util.GetPodPSI(podCgroupDir)
	if err != nil {
		klog.Errorf("collect pod %s psi err: %v", pod.Name, err)
		return
	}
	containerPsiMetric := &metriccache.PodInterferenceMetric{
		MetricName: metriccache.MetricNamePodPSI,
		PodUID:     string(pod.UID),
		MetricValue: &metriccache.PSIMetric{
			SomeCPUAvg10: psiMap["SomeCPUAvg10"],
			SomeMemAvg10: psiMap["SomeMemAvg10"],
			SomeIOAvg10:  psiMap["SomeIOAvg10"],
			FullCPUAvg10: psiMap["FullCPUAvg10"],
			FullMemAvg10: psiMap["FullMemAvg10"],
			FullIOAvg10:  psiMap["FullIOAvg10"],
		},
	}
	err = c.metricCache.InsertPodInterferenceMetrics(collectTime, containerPsiMetric)
	if err != nil {
		klog.Errorf("insert pod psi metrics failed, err %v", err)
	}
}
