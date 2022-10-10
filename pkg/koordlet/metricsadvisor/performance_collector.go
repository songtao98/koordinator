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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/koordinator-sh/koordinator/pkg/koordlet/metriccache"
	"github.com/koordinator-sh/koordinator/pkg/koordlet/statesinformer"
	"github.com/koordinator-sh/koordinator/pkg/util"
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
	containerStatusesMap := map[*corev1.ContainerStatus]string{}
	podMetas := c.statesInformer.GetAllPods()
	for _, meta := range podMetas {
		pod := meta.Pod
		for i := range pod.Status.ContainerStatuses {
			containerStat := &pod.Status.ContainerStatuses[i]
			containerStatusesMap[containerStat] = meta.CgroupDir
		}
	}
	var wg sync.WaitGroup
	wg.Add(len(containerStatusesMap))
	for containerStatus, podParentCgroupDir := range containerStatusesMap {
		go func(status *corev1.ContainerStatus, parent string) {
			defer wg.Done()
			c.collectSingleContainerCPI(parent, status)
			c.collectSingleContainerPSI(parent, status)
		}(containerStatus, podParentCgroupDir)
	}
	wg.Wait()
	klog.V(5).Infof("collectContainerMetrics for time window %s finished at %s, container num %d",
		timeWindow, time.Now(), len(containerStatusesMap))
}

func (c *performanceCollector) collectSingleContainerCPI(podParentCgroupDir string, containerStatus *corev1.ContainerStatus) {
	collectTime := time.Now()
	cycles, instructions, err := util.GetContainerCyclesAndInstructions(podParentCgroupDir, containerStatus, c.collectTimeWindow)
	if err != nil {
		klog.Errorf("collect container %s cpi err: %v", containerStatus.Name, err)
		return
	}
	containerCpiMetric := &metriccache.InterferenceMetric{
		MetricName: metriccache.MetricNameContainerCPI,
		ObjectID:   containerStatus.ContainerID,
		MetricValue: &metriccache.CPIMetric{
			Cycles:       cycles,
			Instructions: instructions,
		},
	}
	err = c.metricCache.InsertInterferenceMetrics(collectTime, containerCpiMetric)
	if err != nil {
		klog.Errorf("insert container cpi metrics failed, err %v", err)
	}
}

func (c *performanceCollector) collectSingleContainerPSI(podParentCgroupDir string, containerStatus *corev1.ContainerStatus) {
	collectTime := time.Now()
	psiMap, err := util.GetContainerPSI(podParentCgroupDir, containerStatus)
	if err != nil {
		klog.Errorf("collect container %s psi err: %v", containerStatus.Name, err)
		return
	}
	containerPsiMetric := &metriccache.InterferenceMetric{
		MetricName: metriccache.MetricNameContainerPSI,
		ObjectID:   containerStatus.ContainerID,
		MetricValue: &metriccache.PSIMetric{
			SomeCPUAvg10: psiMap["SomeCPUAvg10"],
			SomeMemAvg10: psiMap["SomeMemAvg10"],
			SomeIOAvg10:  psiMap["SomeIOAvg10"],
			FullCPUAvg10: psiMap["FullCPUAvg10"],
			FullMemAvg10: psiMap["FullMemAvg10"],
			FullIOAvg10:  psiMap["FullIOAvg10"],
		},
	}
	err = c.metricCache.InsertInterferenceMetrics(collectTime, containerPsiMetric)
	if err != nil {
		klog.Errorf("insert container psi metrics failed, err %v", err)
	}
}

// todo: try to make this collector linux specific, e.g., by build tag linux
