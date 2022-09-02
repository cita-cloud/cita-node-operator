/*
Copyright Rivtower Technologies LLC.

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

package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SwitchoverSpec defines the desired state of Switchover
// Role conversion for two nodes in the same k8s cluster
type SwitchoverSpec struct {
	// Chain
	Chain string `json:"chain"`
	// SourceNode
	SourceNode string `json:"sourceNode"`
	// DestNode
	DestNode string `json:"destNode"`
	// Image
	Image string `json:"image,omitempty"`
	// PullPolicy
	PullPolicy v1.PullPolicy `json:"pullPolicy,omitempty"`
	// ttlSecondsAfterFinished clean up finished Jobs (either Complete or Failed) automatically
	TTLSecondsAfterFinished int64 `json:"ttlSecondsAfterFinished,omitempty"`
}

// SwitchoverStatus defines the observed state of Switchover
type SwitchoverStatus struct {
	// Status
	Status JobConditionType `json:"status,omitempty"`
	// StartTime
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// EndTime
	EndTime *metav1.Time `json:"endTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Switchover is the Schema for the switchovers API
type Switchover struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwitchoverSpec   `json:"spec,omitempty"`
	Status SwitchoverStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SwitchoverList contains a list of Switchover
type SwitchoverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Switchover `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Switchover{}, &SwitchoverList{})
}
