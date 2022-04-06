/*
Copyright 2022.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CitaNodeSpec defines the desired state of CitaNode
type CitaNodeSpec struct {
	// chain name
	ChainName string `json:"chainName"`

	// EnableTLS
	EnableTLS bool `json:"enableTls,omitempty"`

	// ConsensusType
	// +kubebuilder:validation:Enum=BFT;Raft
	ConsensusType ConsensusType `json:"consensusType"`

	// config.toml && kms.db configmap ref
	ConfigMapRef string `json:"configMapRef"`
	// log level
	// +kubebuilder:default:=info
	LogLevel LogLevel `json:"logLevel,omitempty"`
	// storage class name
	StorageClassName *string `json:"storageClassName"`
	// storage size
	StorageSize *int64 `json:"storageSize"`
	// ImageInfo
	ImageInfo `json:"imageInfo,omitempty"`
	// desire status
	// +kubebuilder:validation:Enum=Running;Stopped
	DesiredState Status `json:"desiredState"`
}

// CitaNodeStatus defines the observed state of CitaNode
type CitaNodeStatus struct {
	Status Status `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CitaNode is the Schema for the citanodes API
type CitaNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CitaNodeSpec   `json:"spec,omitempty"`
	Status CitaNodeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CitaNodeList contains a list of CitaNode
type CitaNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CitaNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CitaNode{}, &CitaNodeList{})
}

type LogLevel string

const (
	Info LogLevel = "info"
	Warn LogLevel = "warn"
)

type Status string

const (
	Running   Status = "Running"
	Stopped   Status = "Stopped"
	Stopping  Status = "Stopping"
	Starting  Status = "Starting"
	Upgrading Status = "Upgrading"
	Error     Status = "Error"
)

type ConsensusType string

const (
	BFT  ConsensusType = "BFT"
	Raft ConsensusType = "Raft"
)
