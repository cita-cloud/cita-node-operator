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

const (
	NetworkContainer    = "network"
	ConsensusContainer  = "consensus"
	ExecutorContainer   = "executor"
	StorageContainer    = "storage"
	ControllerContainer = "controller"
	KmsContainer        = "kms"

	AccountVolumeName         = "account"
	KmsDBMountPath            = "/mnt"
	LogConfigVolumeName       = "log-config"
	NodeConfigVolumeName      = "node-config"
	NodeConfigVolumeMountPath = "/etc/cita-cloud/config"
	DataVolumeName            = "datadir"
	DataVolumeMountPath       = "/data"
	LogConfigVolumeMountPath  = "/etc/cita-cloud/log"
	LogDir                    = DataVolumeMountPath + "/logs"

	NodeConfigFile          = "config.toml"
	ControllerLogConfigFile = "controller-log4rs.yaml"
	ExecutorLogConfigFile   = "executor-log4rs.yaml"
	KmsLogConfigFile        = "kms-log4rs.yaml"
	NetworkLogConfigFile    = "network-log4rs.yaml"
	StorageLogConfigFile    = "storage-log4rs.yaml"
	ConsensusLogConfigFile  = "consensus-log4rs.yaml"

	NetworkPort    = 40000
	ControllerPort = 50004
	ExecutorPort   = 50002

	POD_NAME_ENV      = "MY_POD_NAME"
	POD_NAMESPACE_ENV = "MY_POD_NAMESPACE"
)
