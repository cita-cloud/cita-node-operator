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
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BackupSpec defines the desired state of Backup
type BackupSpec struct {
	// Chain
	Chain string `json:"chain"`
	// Node
	Node string `json:"node"`
	// DeployMethod
	DeployMethod nodepkg.DeployMethod `json:"deployMethod"`
	// StorageClass
	StorageClass string `json:"storageClass,omitempty"`
	// Action
	Action nodepkg.Action `json:"action,omitempty"`
	// Image
	Image string `json:"image,omitempty"`
	// PullPolicy
	PullPolicy v1.PullPolicy `json:"pullPolicy,omitempty"`
	// ttlSecondsAfterFinished clean up finished Jobs (either Complete or Failed) automatically
	TTLSecondsAfterFinished int64 `json:"ttlSecondsAfterFinished,omitempty"`
	// PodAffinityFlag weather or not the job's affinity with chain node's pod. Notice: helm chain must be false
	PodAffinityFlag bool `json:"podAffinityFlag,omitempty"`
}

// BackupStatus defines the observed state of Backup
type BackupStatus struct {
	// Allocate
	Allocate int64 `json:"allocate,omitempty"`
	// Actual
	Actual int64 `json:"actual,omitempty"`
	// Status
	Status JobConditionType `json:"status,omitempty"`
	// StartTime
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// EndTime
	EndTime *metav1.Time `json:"endTime,omitempty"`
	// BlockHeight
	BlockHeight int64 `json:"blockHeight,omitempty"`
	// Message
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Backup is the Schema for the backups API
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}

const (
	BackupSourceVolumeName = "backup-source"
	BackupSourceVolumePath = "/backup-source"
	BackupDestVolumeName   = "backup-dest"
	BackupDestVolumePath   = "/backup-dest"
)
