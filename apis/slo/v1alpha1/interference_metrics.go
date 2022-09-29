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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// InterferenceMetricSpec defines the desired state of InterferenceMetric
type InterferenceMetricSpec struct {
	// CollectPolicy defines the Metric collection policy
	CollectPolicy *InterferenceMetricCollectPolicy `json:"metricCollectPolicy,omitempty"`
}

// InterferenceMetricCollectPolicy defines the Metric collection policy
type InterferenceMetricCollectPolicy struct {
	// ReportIntervalSeconds represents the report period in seconds
	ReportIntervalSeconds *int64 `json:"reportIntervalSeconds,omitempty"`
}

// InterferenceMetricStatus defines the observed state of InterferenceMetric
type InterferenceMetricStatus struct {
	// UpdateTime is the last time this InterferenceMetric was updated.
	UpdateTime *metav1.Time `json:"updateTime,omitempty"`

	// todo：define low-level metrics api, which should contains cpi/psi/csd
}

// InterferenceMetric is the Schema for the interference_metric API
type InterferenceMetric struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InterferenceMetricSpec   `json:"spec,omitempty"`
	Status InterferenceMetricStatus `json:"status,omitempty"`
}

// InterferenceMetricList contains a list of InterferenceMetric
type InterferenceMetricList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InterferenceMetric `json:"items"`
}

func init() {
	// todo: controller-gen
	// SchemeBuilder.Register(&InterferenceMetric{}, &InterferenceMetricList{})
}
