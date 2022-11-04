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
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetErrorLogFromPod(ctx context.Context, cli client.Client, namespace, name, jobId string) (string, error) {
	podList := &corev1.PodList{}
	podOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{"job-name": name, "controller-uid": jobId}),
	}
	if err := cli.List(ctx, podList, podOpts...); err != nil {
		return "", err
	}
	if len(podList.Items) != 1 {
		return "", fmt.Errorf("job's pod != 1")
	}
	return podList.Items[0].Annotations["err-log"], nil
}
