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

package python

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
	pyNodeLog = ctrl.Log.WithName("py-node")
)

type pyNode struct {
	client.Client
	behavior  behavior.Interface
	namespace string
	name      string
	chain     string
}

func (p *pyNode) SnapshotRecover(ctx context.Context, blockHeight int64, crypto, consensus string) error {
	//TODO implement me
	panic("implement me")
}

func (p *pyNode) GetName() string {
	return p.name
}

func (p *pyNode) UpdateAccountConfigmap(ctx context.Context, newConfigmap string) error {
	//TODO implement me
	panic("implement me")
}

func (p *pyNode) ChangeOwner(ctx context.Context, action node.Action, uid, gid int64) error {
	if action == node.StopAndStart {
		if err := p.Stop(ctx); err != nil {
			return err
		}
		if err := p.CheckStopped(ctx); err != nil {
			return err
		}
	}
	if err := p.behavior.ChangeOwner(uid, gid, fmt.Sprintf("%s/%s", citacloudv1.ChangeOwnerVolumePath, p.name)); err != nil {
		return err
	}
	if action == node.StopAndStart {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *pyNode) Restore(ctx context.Context, action node.Action) error {
	if action == node.StopAndStart {
		if err := p.Stop(ctx); err != nil {
			return err
		}
		if err := p.CheckStopped(ctx); err != nil {
			return err
		}
	}
	if err := p.behavior.Restore(citacloudv1.RestoreSourceVolumePath, fmt.Sprintf("%s/%s", citacloudv1.RestoreDestVolumePath, p.name)); err != nil {
		return err
	}
	if action == node.StopAndStart {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *pyNode) Backup(ctx context.Context, action node.Action) error {
	if action == node.StopAndStart {
		err := p.Stop(ctx)
		if err != nil {
			return err
		}
		err = p.CheckStopped(ctx)
		if err != nil {
			return err
		}
	}

	totalSize, err := p.behavior.Backup(fmt.Sprintf("%s/%s", citacloudv1.BackupSourceVolumePath, p.name), citacloudv1.BackupDestVolumePath)
	if err != nil {
		return err
	}

	annotations := map[string]string{"backup-size": strconv.FormatInt(totalSize, 10)}
	err = p.AddAnnotations(ctx, annotations)
	if err != nil {
		return err
	}
	if action == node.StopAndStart {
		err = p.Start(ctx)
		if err != nil {
			return err
		}
	}
	return err
}

func (p *pyNode) AddAnnotations(ctx context.Context, annotations map[string]string) error {
	pod := &corev1.Pod{}
	err := p.Get(ctx, types.NamespacedName{
		Name:      os.Getenv("MY_POD_NAME"),
		Namespace: os.Getenv("MY_POD_NAMESPACE"),
	}, pod)
	if err != nil {
		pyNodeLog.Error(err, fmt.Sprintf("get pod %s/%s failed", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
		return err
	}
	pod.Annotations = annotations
	err = p.Update(ctx, pod)
	if err != nil {
		pyNodeLog.Error(err, fmt.Sprintf("update pod %s/%s annotation failed", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
		return err
	}
	pyNodeLog.Info(fmt.Sprintf("update pod %s/%s annotation successful", os.Getenv("MY_POD_NAMESPACE"), os.Getenv("MY_POD_NAME")))
	return nil
}

func (p *pyNode) Stop(ctx context.Context) error {
	pyNodeLog.Info(fmt.Sprintf("stop node %s/%s for deployment...", p.namespace, p.name))
	// find chain
	dep := &appsv1.Deployment{}
	err := p.Get(ctx, types.NamespacedName{Name: p.name, Namespace: p.namespace}, dep)
	if err != nil {
		pyNodeLog.Error(err, fmt.Sprintf("get node %s/%s failed", p.namespace, p.name))
		return err
	}
	dep.Spec.Replicas = pointer.Int32(0)
	err = p.Update(ctx, dep)
	if err != nil {
		return err
	}
	pyNodeLog.Info(fmt.Sprintf("scale down deployment to 0 for node %s/%s successful", p.namespace, p.name))
	return nil
}

func (p *pyNode) CheckStopped(ctx context.Context) error {
	pyNodeLog.Info(fmt.Sprintf("wait the node %s/%s stopped...", p.namespace, p.name))
	var checkStopped func(context.Context) (bool, error)
	checkStopped = func(ctx context.Context) (bool, error) {
		found := &appsv1.Deployment{}
		err := p.Get(ctx, types.NamespacedName{Name: p.name, Namespace: p.namespace}, found)
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
		return fmt.Errorf("wait deployment replicas to 0 timeout")
	}
	pyNodeLog.Info(fmt.Sprintf("the node %s/%s have stopped", p.namespace, p.name))
	return nil
}

func (p *pyNode) Fallback(ctx context.Context, blockHeight int64, crypto, consensus string) error {
	err := p.Stop(ctx)
	if err != nil {
		return err
	}
	err = p.CheckStopped(ctx)
	if err != nil {
		return err
	}
	err = p.behavior.Fallback(blockHeight, fmt.Sprintf("%s/%s", citacloudv1.VolumeMountPath, p.name),
		fmt.Sprintf("%s/%s", citacloudv1.VolumeMountPath, p.name), crypto, consensus)
	if err != nil {
		return err
	}
	err = p.Start(ctx)
	if err != nil {
		return err
	}
	return err
}

func (p *pyNode) Snapshot(ctx context.Context, blockHeight int64, crypto, consensus string) error {
	err := p.Stop(ctx)
	if err != nil {
		return err
	}
	err = p.CheckStopped(ctx)
	if err != nil {
		return err
	}
	snapshotSize, err := p.behavior.Snapshot(blockHeight, fmt.Sprintf("%s/%s", citacloudv1.BackupSourceVolumePath, p.name),
		fmt.Sprintf("%s/%s", citacloudv1.BackupSourceVolumePath, p.name), citacloudv1.BackupDestVolumePath, crypto, consensus)
	if err != nil {
		return err
	}

	annotations := map[string]string{"snapshot-size": strconv.FormatInt(snapshotSize, 10)}
	err = p.AddAnnotations(ctx, annotations)
	if err != nil {
		return err
	}

	err = p.Start(ctx)
	if err != nil {
		return err
	}
	return err
}

func (p *pyNode) Start(ctx context.Context) error {
	pyNodeLog.Info(fmt.Sprintf("starting node %s/%s ...", p.namespace, p.name))
	dep := &appsv1.Deployment{}
	err := p.Get(ctx, types.NamespacedName{Name: p.name, Namespace: p.namespace}, dep)
	if err != nil {
		return err
	}
	dep.Spec.Replicas = pointer.Int32(1)
	err = p.Update(ctx, dep)
	if err != nil {
		return err
	}
	pyNodeLog.Info(fmt.Sprintf("start node %s/%s successful", p.namespace, p.name))
	return nil
}

func newPyNode(namespace, name string, client client.Client, chain string, execer exec.Interface) (node.Node, error) {
	return &pyNode{
		Client:    client,
		behavior:  behavior.NewBehavior(execer, pyNodeLog),
		namespace: namespace,
		name:      name,
		chain:     chain,
	}, nil
}

func init() {
	node.Register(node.PythonOperator, newPyNode)
}
