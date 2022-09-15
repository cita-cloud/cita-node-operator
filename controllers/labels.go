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
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
)

func LabelsForNode(chainName, nodeName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/chain-name": chainName,
		"app.kubernetes.io/chain-node": nodeName,
	}
}

func LabelsForChain(chainName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/chain-name": chainName,
	}
}

// MergeLabels merges all labels together and returns a new label.
func MergeLabels(allLabels ...map[string]string) map[string]string {
	lb := make(map[string]string)

	for _, label := range allLabels {
		for k, v := range label {
			lb[k] = v
		}
	}

	return lb
}

func GetNodeLabelKeyByType(deployMethod nodepkg.DeployMethod) (string, error) {
	switch deployMethod {
	case nodepkg.Helm:
		return "app.kubernetes.io/chain-name", nil
	case nodepkg.PythonOperator:
		return "node_name", nil
	case nodepkg.CloudConfig:
		return "app.kubernetes.io/chain-node", nil
	default:
		return "", fmt.Errorf("mismatched deploy method")
	}
}
