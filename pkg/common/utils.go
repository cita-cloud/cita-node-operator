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

package common

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AddLogToPodAnnotation(ctx context.Context, client client.Client, fn func() error) error {
	jobErr := fn()
	if jobErr != nil {
		// get job's pod
		pod := &corev1.Pod{}
		err := client.Get(ctx, types.NamespacedName{
			Namespace: os.Getenv("MY_POD_NAMESPACE"),
			Name:      os.Getenv("MY_POD_NAME"),
		}, pod)
		if err != nil {
			return err
		}
		// update err logs to job's pod
		annotations := map[string]string{"err-log": jobErr.Error()}
		pod.Annotations = annotations
		err = client.Update(ctx, pod)
		if err != nil {
			return err
		}
		return jobErr
	}
	return nil
}
