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

package controllers

import (
	"fmt"
	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

type NodeService struct {
	Node *citacloudv1.CitaNode
}

func NewChainNodeServiceForLog(node *citacloudv1.CitaNode) *NodeService {
	return &NodeService{Node: node}
}

type ChainNodeServiceImpl interface {
	GenerateControllerLogConfig() string
	GenerateExecutorLogConfig() string
	GenerateKmsLogConfig() string
	GenerateNetworkLogConfig() string
	GenerateStorageLogConfig() string
	GenerateConsensusLogConfig() string
}

func (cns *NodeService) GenerateControllerLogConfig() string {
	return fmt.Sprintf(`# Scan this file for changes every 30 seconds
refresh_rate: 30 seconds

appenders:
# An appender named "stdout" that writes to stdout
  stdout:
    kind: console

  journey-service:
    kind: rolling_file
    path: "%s/controller-service.log"
    policy:
      # Identifies which policy is to be used. If no kind is specified, it will
      # default to "compound".
      kind: compound
      # The remainder of the configuration is passed along to the policy's
      # deserializer, and will vary based on the kind of policy.
      trigger:
        kind: size
        limit: 50mb
      roller:
        kind: fixed_window
        base: 1
        count: 5
        pattern: "%s/controller-service.{}.gz"

# Set the default logging level and attach the default appender to the root
root:
  level: %s
  appenders:
    - stdout
    - journey-service`, LogDir, LogDir, string(cns.Node.Spec.LogLevel))
}

func (cns *NodeService) GenerateExecutorLogConfig() string {
	return fmt.Sprintf(`# Scan this file for changes every 30 seconds
refresh_rate: 30 seconds

appenders:
# An appender named "stdout" that writes to stdout
  stdout:
    kind: console

  journey-service:
    kind: rolling_file
    path: "%s/executor-service.log"
    policy:
      # Identifies which policy is to be used. If no kind is specified, it will
      # default to "compound".
      kind: compound
      # The remainder of the configuration is passed along to the policy's
      # deserializer, and will vary based on the kind of policy.
      trigger:
        kind: size
        limit: 50mb
      roller:
        kind: fixed_window
        base: 1
        count: 5
        pattern: "%s/executor-service.{}.gz"

# Set the default logging level and attach the default appender to the root
root:
  level: %s
  appenders:
    - stdout
    - journey-service`, LogDir, LogDir, string(cns.Node.Spec.LogLevel))
}

func (cns *NodeService) GenerateKmsLogConfig() string {
	return fmt.Sprintf(`# Scan this file for changes every 30 seconds
refresh_rate: 30 seconds

appenders:
# An appender named "stdout" that writes to stdout
  stdout:
    kind: console

  journey-service:
    kind: rolling_file
    path: "%s/kms-service.log"
    policy:
      # Identifies which policy is to be used. If no kind is specified, it will
      # default to "compound".
      kind: compound
      # The remainder of the configuration is passed along to the policy's
      # deserializer, and will vary based on the kind of policy.
      trigger:
        kind: size
        limit: 50mb
      roller:
        kind: fixed_window
        base: 1
        count: 5
        pattern: "%s/kms-service.{}.gz"

# Set the default logging level and attach the default appender to the root
root:
  level: %s
  appenders:
    - stdout
    - journey-service`, LogDir, LogDir, string(cns.Node.Spec.LogLevel))
}

func (cns *NodeService) GenerateNetworkLogConfig() string {
	return fmt.Sprintf(`# Scan this file for changes every 30 seconds
refresh_rate: 30 seconds

appenders:
# An appender named "stdout" that writes to stdout
  stdout:
    kind: console

  journey-service:
    kind: rolling_file
    path: "%s/network-service.log"
    policy:
      # Identifies which policy is to be used. If no kind is specified, it will
      # default to "compound".
      kind: compound
      # The remainder of the configuration is passed along to the policy's
      # deserializer, and will vary based on the kind of policy.
      trigger:
        kind: size
        limit: 50mb
      roller:
        kind: fixed_window
        base: 1
        count: 5
        pattern: "%s/network-service.{}.gz"

# Set the default logging level and attach the default appender to the root
root:
  level: %s
  appenders:
    - stdout
    - journey-service`, LogDir, LogDir, string(cns.Node.Spec.LogLevel))
}

func (cns *NodeService) GenerateConsensusLogConfig() string {
	return fmt.Sprintf(`# Scan this file for changes every 30 seconds
refresh_rate: 30 seconds

appenders:
# An appender named "stdout" that writes to stdout
  stdout:
    kind: console

  journey-service:
    kind: rolling_file
    path: "%s/consensus-service.log"
    policy:
      # Identifies which policy is to be used. If no kind is specified, it will
      # default to "compound".
      kind: compound
      # The remainder of the configuration is passed along to the policy's
      # deserializer, and will vary based on the kind of policy.
      trigger:
        kind: size
        limit: 50mb
      roller:
        kind: fixed_window
        base: 1
        count: 5
        pattern: "%s/consensus-service.{}.gz"

# Set the default logging level and attach the default appender to the root
root:
  level: %s
  appenders:
    - stdout
    - journey-service`, LogDir, LogDir, string(cns.Node.Spec.LogLevel))
}

func (cns *NodeService) GenerateStorageLogConfig() string {
	return fmt.Sprintf(`# Scan this file for changes every 30 seconds
refresh_rate: 30 seconds

appenders:
# An appender named "stdout" that writes to stdout
  stdout:
    kind: console

  journey-service:
    kind: rolling_file
    path: "%s/storage-service.log"
    policy:
      # Identifies which policy is to be used. If no kind is specified, it will
      # default to "compound".
      kind: compound
      # The remainder of the configuration is passed along to the policy's
      # deserializer, and will vary based on the kind of policy.
      trigger:
        kind: size
        limit: 50mb
      roller:
        kind: fixed_window
        base: 1
        count: 5
        pattern: "%s/storage-service.{}.gz"

# Set the default logging level and attach the default appender to the root
root:
  level: %s
  appenders:
    - stdout
    - journey-service`, LogDir, LogDir, string(cns.Node.Spec.LogLevel))
}
