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
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

func CleanJob(ctx context.Context, cli client.Client, job *v1.Job, ttl int64) {
	logger := log.FromContext(ctx)
	myT := time.NewTimer(time.Duration(ttl) * time.Second)
	<-myT.C
	dp := metav1.DeletePropagationForeground
	do := &client.DeleteOptions{}
	do.ApplyOptions([]client.DeleteOption{
		client.PropagationPolicy(dp),
	})
	err := cli.Delete(ctx, job, do)
	if err != nil {
		logger.Error(err, "delete job failed")
		return
	}
	//myT.Stop()
	logger.Info("delete job success")
}
