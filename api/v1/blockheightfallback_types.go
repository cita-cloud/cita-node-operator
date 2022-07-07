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
	fallback "github.com/cita-cloud/cita-node-operator/pkg/chain"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BlockHeightFallbackSpec defines the desired state of BlockHeightFallback
type BlockHeightFallbackSpec struct {
	// ChainName
	ChainName string `json:"chainName"`
	// Namespace
	Namespace string `json:"namespace"`
	// BlockHeight
	BlockHeight int64 `json:"blockHeight"`
	// ChainDeployMethod
	ChainDeployMethod fallback.DeployMethod `json:"chainDeployMethod"`
	// NodeList
	NodeList string `json:"nodeList,omitempty"`
	// Image
	Image string `json:"image,omitempty"`
	// PullPolicy
	PullPolicy v1.PullPolicy `json:"pullPolicy,omitempty"`
}

// BlockHeightFallbackStatus defines the observed state of BlockHeightFallback
type BlockHeightFallbackStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status JobConditionType `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=bhf

// BlockHeightFallback is the Schema for the blockheightfallbacks API
type BlockHeightFallback struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BlockHeightFallbackSpec   `json:"spec,omitempty"`
	Status BlockHeightFallbackStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BlockHeightFallbackList contains a list of BlockHeightFallback
type BlockHeightFallbackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlockHeightFallback `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlockHeightFallback{}, &BlockHeightFallbackList{})
}

type JobConditionType string

// These are valid conditions of a job.
const (
	// JobActive means the job is active.
	JobActive JobConditionType = "Active"
	// JobComplete means the job has completed its execution.
	JobComplete JobConditionType = "Complete"
	// JobFailed means the job has failed its execution.
	JobFailed JobConditionType = "Failed"
)

const (
	FallbackJobServiceAccount     = "fallback-job"
	FallbackJobClusterRole        = "fallback-job"
	FallbackJobClusterRoleBinding = "fallback-job"
)

const (
	VolumeName      = "datadir"
	VolumeMountPath = "/mnt"
)

const DefaultImage = "registry.devops.rivtower.com/cita-cloud/operator/fallback-job:v0.0.1"

func (in BlockHeightFallback) AllNodes() bool {
	if in.Spec.NodeList == "*" {
		return true
	}
	return false
}
