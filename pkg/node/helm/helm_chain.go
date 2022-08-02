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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/exec"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var (
	helmNodeLog = ctrl.Log.WithName("helm-node")
)

type helmNode struct {
	client.Client
	namespace string
	name      string
	chain     string
	replicas  *int32
	execer    exec.Interface
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
	if err := h.restore(); err != nil {
		return err
	}
	if action == node.StopAndStart {
		if err := h.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (h *helmNode) restore() error {
	err := h.execer.Command("cp", "-r", citacloudv1.RestoreSourceVolumePath, fmt.Sprintf("%s/%s", citacloudv1.RestoreDestVolumePath, h.name)).Run()
	if err != nil {
		helmNodeLog.Error(err, "restore file failed")
		return err
	}

	helmNodeLog.Info("restore file completed")
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

func (h *helmNode) Fallback(ctx context.Context, blockHeight int64) error {
	err := h.Stop(ctx)
	if err != nil {
		return err
	}
	err = h.CheckStopped(ctx)
	if err != nil {
		return err
	}
	err = h.fallback(blockHeight)
	if err != nil {
		return err
	}
	err = h.Start(ctx)
	if err != nil {
		return err
	}
	return err
}

func (h *helmNode) fallback(blockHeight int64) error {
	helmNodeLog.Info(fmt.Sprintf("exec block height fallback: [node: %s, height: %d]...", h.name, blockHeight))
	//exec := utilexec.New()
	err := h.execer.Command("cloud-op", "recover", fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("/mnt/%s", h.name),
		"--config-path", fmt.Sprintf("/mnt/%s/config.toml", h.name)).Run()
	if err != nil {
		helmNodeLog.Error(err, "exec block height fallback failed")
		return err
	}
	helmNodeLog.Info(fmt.Sprintf("exec block height fallback: [node: %s, height: %d] successful", h.name, blockHeight))
	return nil
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
	//TODO implement me
	panic("implement me")
}

func newHelmNode(namespace, name string, client client.Client, chain string, execer exec.Interface) (node.Node, error) {
	return &helmNode{
		Client:    client,
		namespace: namespace,
		name:      name,
		chain:     chain,
		execer:    execer,
	}, nil
}

func init() {
	node.Register(node.Helm, newHelmNode)
}
