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

package helm

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
	helmNodeLog = ctrl.Log.WithName("helm-node")
)

type helmNode struct {
	client.Client
	behavior  behavior.Interface
	namespace string
	name      string
	chain     string
	replicas  *int32
}

func (h *helmNode) SnapshotRecover(ctx context.Context, blockHeight int64, crypto, consensus string) error {
	//TODO implement me
	panic("implement me")
}

func (h *helmNode) GetName() string {
	return h.name
}

func (h *helmNode) UpdateAccountConfigmap(ctx context.Context, newConfigmap string) error {
	//TODO implement me
	panic("implement me")
}

func (h *helmNode) Restore(ctx context.Context, action node.Action) error {
	if action == node.StopAndStart {
		if err := h.Stop(ctx); err != nil {
			return err
		}
		if err := h.CheckStopped(ctx); err != nil {
			return err
		}
	}
	if err := h.behavior.Restore(citacloudv1.RestoreSourceVolumePath, fmt.Sprintf("%s/%s", citacloudv1.RestoreDestVolumePath, h.name)); err != nil {
		return err
	}
	if action == node.StopAndStart {
		if err := h.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop Since it is a chain composed of statefulsets, the entire chain must be stopped.
func (h *helmNode) Stop(ctx context.Context) error {
	helmNodeLog.Info("stop node for statefulset...")
	// find chain
	sts := &appsv1.StatefulSet{}
	err := h.Get(ctx, types.NamespacedName{Name: h.chain, Namespace: h.namespace}, sts)
	if err != nil {
		helmNodeLog.Error(err, fmt.Sprintf("get helm chain %s/%s failed", h.namespace, h.chain))
		return err
	}
	h.replicas = sts.Spec.Replicas
	sts.Spec.Replicas = pointer.Int32(0)
	err = h.Update(ctx, sts)
	if err != nil {
		return err
	}
	helmNodeLog.Info("scale down statefulset to 0 for node successful")
	return nil
}

func (h *helmNode) CheckStopped(ctx context.Context) error {
	helmNodeLog.Info("wait the chain's node stopped...")
	var checkStopped func(context.Context) (bool, error)
	checkStopped = func(ctx context.Context) (bool, error) {
		found := &appsv1.StatefulSet{}
		err := h.Get(ctx, types.NamespacedName{Name: h.chain, Namespace: h.namespace}, found)
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
	helmNodeLog.Info("the node's all node have stopped")
	return nil
}

func (h *helmNode) Fallback(ctx context.Context, blockHeight int64, crypto, consensus string) error {
	err := h.Stop(ctx)
	if err != nil {
		return err
	}
	err = h.CheckStopped(ctx)
	if err != nil {
		return err
	}
	err = h.behavior.Fallback(blockHeight, fmt.Sprintf("%s/%s", citacloudv1.VolumeMountPath, h.name),
		fmt.Sprintf("%s/%s", citacloudv1.VolumeMountPath, h.name), crypto, consensus)
	if err != nil {
		return err
	}
	err = h.Start(ctx)
	if err != nil {
		return err
	}
	return err
}

func (h *helmNode) Snapshot(ctx context.Context, blockHeight int64, crypto, consensus string) error {
	//TODO implement me
	panic("implement me")
}

func (h *helmNode) Start(ctx context.Context) error {
	helmNodeLog.Info("start node for statefulset...")
	sts := &appsv1.StatefulSet{}
	err := h.Get(ctx, types.NamespacedName{Name: h.chain, Namespace: h.namespace}, sts)
	if err != nil {
		return err
	}
	sts.Spec.Replicas = h.replicas
	err = h.Update(ctx, sts)
	if err != nil {
		return err
	}
	helmNodeLog.Info(fmt.Sprintf("scale up statefulset to [%d] for node successful", *h.replicas))
	return nil
}

func (h *helmNode) Backup(ctx context.Context, action node.Action) error {
	if action == node.StopAndStart {
		err := h.Stop(ctx)
		if err != nil {
			return err
		}
		err = h.CheckStopped(ctx)
		if err != nil {
			return err
		}
	}

	totalSize, err := h.behavior.Backup(fmt.Sprintf("%s/%s", citacloudv1.BackupSourceVolumePath, h.name), citacloudv1.BackupDestVolumePath)
	if err != nil {
		return err
	}

	annotations := map[string]string{"backup-size": strconv.FormatInt(totalSize, 10)}
	err = h.AddAnnotations(ctx, annotations)
	if err != nil {
		return err
	}
	if action == node.StopAndStart {
		err = h.Start(ctx)
		if err != nil {
			return err
		}
	}
	return err
}

func (h *helmNode) AddAnnotations(ctx context.Context, annotations map[string]string) error {
	pod := &corev1.Pod{}
	err := h.Get(ctx, types.NamespacedName{
		Name:      os.Getenv("MY_POD_NAME"),
		Namespace: os.Getenv("MY_POD_NAMESPACE"),
	}, pod)
	if err != nil {
		helmNodeLog.Error(err, fmt.Sprintf("get pod %s/%s failed", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
		return err
	}
	pod.Annotations = annotations
	err = h.Update(ctx, pod)
	if err != nil {
		helmNodeLog.Error(err, fmt.Sprintf("update pod %s/%s annotation failed", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
		return err
	}
	helmNodeLog.Info(fmt.Sprintf("update pod %s/%s annotation successful", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
	return nil
}

func newHelmNode(namespace, name string, client client.Client, chain string, execer exec.Interface) (node.Node, error) {
	return &helmNode{
		Client:    client,
		behavior:  behavior.NewBehavior(execer, helmNodeLog),
		namespace: namespace,
		name:      name,
		chain:     chain,
	}, nil
}

func init() {
	node.Register(node.Helm, newHelmNode)
}
