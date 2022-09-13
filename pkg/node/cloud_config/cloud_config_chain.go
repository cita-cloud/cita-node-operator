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

package cloud_config

import (
	"context"
	"fmt"
	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
	"github.com/cita-cloud/cita-node-operator/pkg/node"
	"github.com/cita-cloud/cita-node-operator/pkg/node/behavior"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/exec"
	"k8s.io/utils/pointer"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"time"
)

var (
	cloudConfigNodeLog = ctrl.Log.WithName("cloud-config-node")
)

type cloudConfigNode struct {
	client.Client
	behavior  behavior.Interface
	namespace string
	name      string
	chain     string
}

func (c *cloudConfigNode) GetName() string {
	return c.name
}

func (c *cloudConfigNode) UpdateAccountConfigmap(ctx context.Context, newConfigmap string) error {
	// find node
	sts := &appsv1.StatefulSet{}
	err := c.Get(ctx, types.NamespacedName{Name: c.name, Namespace: c.namespace}, sts)
	if err != nil {
		cloudConfigNodeLog.Error(err, fmt.Sprintf("get node %s/%s failed", c.namespace, c.name))
		return err
	}
	volumes := sts.Spec.Template.Spec.Volumes
	for _, vol := range volumes {
		if vol.Name == "node-account" {
			vol.VolumeSource.ConfigMap.LocalObjectReference.Name = newConfigmap
		}
	}
	sts.Spec.Template.Spec.Volumes = volumes
	err = c.Update(ctx, sts)
	if err != nil {
		cloudConfigNodeLog.Error(err, fmt.Sprintf("update node %s/%s account configmap failed", c.namespace, c.name))
		return err
	}
	return nil
}

func (c *cloudConfigNode) Restore(ctx context.Context, action node.Action) error {
	if action == node.StopAndStart {
		if err := c.Stop(ctx); err != nil {
			return err
		}
		if err := c.CheckStopped(ctx); err != nil {
			return err
		}
	}
	if err := c.behavior.Restore(citacloudv1.RestoreSourceVolumePath, citacloudv1.RestoreDestVolumePath); err != nil {
		return err
	}
	if action == node.StopAndStart {
		if err := c.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *cloudConfigNode) Stop(ctx context.Context) error {
	cloudConfigNodeLog.Info(fmt.Sprintf("stop node %s/%s for statefulset...", c.namespace, c.name))
	// find chain
	sts := &appsv1.StatefulSet{}
	err := c.Get(ctx, types.NamespacedName{Name: c.name, Namespace: c.namespace}, sts)
	if err != nil {
		cloudConfigNodeLog.Error(err, fmt.Sprintf("get node %s/%s failed", c.namespace, c.name))
		return err
	}
	sts.Spec.Replicas = pointer.Int32(0)
	err = c.Update(ctx, sts)
	if err != nil {
		return err
	}
	cloudConfigNodeLog.Info(fmt.Sprintf("scale down statefulset to 0 for node %s/%s successful", c.namespace, c.name))
	return nil
}

func (c *cloudConfigNode) CheckStopped(ctx context.Context) error {
	cloudConfigNodeLog.Info(fmt.Sprintf("wait the node %s/%s stopped...", c.namespace, c.name))
	var checkStopped func(context.Context) (bool, error)
	checkStopped = func(ctx context.Context) (bool, error) {
		found := &appsv1.StatefulSet{}
		err := c.Get(ctx, types.NamespacedName{Name: c.name, Namespace: c.namespace}, found)
		if err != nil {
			return false, err
		}
		if found.Status.ReadyReplicas != 0 {
			return false, nil
		}
		return true, nil
	}
	// check it for 3 per second
	err := wait.Poll(3*time.Second, 60*time.Second, func() (done bool, err error) {
		return checkStopped(ctx)
	})
	if err != nil {
		return fmt.Errorf("wait statefulset replicas to 0 timeout")
	}
	cloudConfigNodeLog.Info(fmt.Sprintf("the node %s/%s have stopped", c.namespace, c.name))
	return nil
}

func (c *cloudConfigNode) Fallback(ctx context.Context, blockHeight int64, crypto, consensus string) error {
	err := c.Stop(ctx)
	if err != nil {
		return err
	}
	err = c.CheckStopped(ctx)
	if err != nil {
		return err
	}
	err = c.behavior.Fallback(blockHeight, citacloudv1.VolumeMountPath, citacloudv1.ConfigMountPath, crypto, consensus)
	if err != nil {
		return err
	}
	err = c.Start(ctx)
	if err != nil {
		return err
	}
	return err
}

func (c *cloudConfigNode) Start(ctx context.Context) error {
	cloudConfigNodeLog.Info(fmt.Sprintf("starting node %s/%s ...", c.namespace, c.name))
	sts := &appsv1.StatefulSet{}
	err := c.Get(ctx, types.NamespacedName{Name: c.name, Namespace: c.namespace}, sts)
	if err != nil {
		return err
	}
	sts.Spec.Replicas = pointer.Int32(1)
	err = c.Update(ctx, sts)
	if err != nil {
		return err
	}
	cloudConfigNodeLog.Info(fmt.Sprintf("start node %s/%s successful", c.namespace, c.name))
	return nil
}

func (c *cloudConfigNode) Backup(ctx context.Context, action node.Action) error {
	if action == node.StopAndStart {
		err := c.Stop(ctx)
		if err != nil {
			return err
		}
		err = c.CheckStopped(ctx)
		if err != nil {
			return err
		}
	}
	totalSize, err := c.behavior.Backup(citacloudv1.BackupSourceVolumePath, citacloudv1.BackupDestVolumePath)
	if err != nil {
		return err
	}

	annotations := map[string]string{"backup-size": strconv.FormatInt(totalSize, 10)}
	err = c.AddAnnotations(ctx, annotations)
	if err != nil {
		return err
	}

	if action == node.StopAndStart {
		err = c.Start(ctx)
		if err != nil {
			return err
		}
	}
	return err
}

func (c *cloudConfigNode) AddAnnotations(ctx context.Context, annotations map[string]string) error {
	pod := &corev1.Pod{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      os.Getenv("MY_POD_NAME"),
		Namespace: os.Getenv("MY_POD_NAMESPACE"),
	}, pod)
	if err != nil {
		cloudConfigNodeLog.Error(err, fmt.Sprintf("get pod %s/%s failed", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
		return err
	}
	pod.Annotations = annotations
	err = c.Update(ctx, pod)
	if err != nil {
		cloudConfigNodeLog.Error(err, fmt.Sprintf("update pod %s/%s annotation failed", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
		return err
	}
	cloudConfigNodeLog.Info(fmt.Sprintf("update pod %s/%s annotation successful", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
	return nil
}

func newCloudConfigNode(namespace, name string, client client.Client, chain string, execer exec.Interface) (node.Node, error) {
	return &cloudConfigNode{
		Client:    client,
		behavior:  behavior.NewBehavior(execer, cloudConfigNodeLog),
		namespace: namespace,
		name:      name,
		chain:     chain,
	}, nil
}

func init() {
	node.Register(node.CloudConfig, newCloudConfigNode)
}
