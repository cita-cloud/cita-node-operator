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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/exec"
	"k8s.io/utils/pointer"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

var (
	pyNodeLog = ctrl.Log.WithName("py-node")
)

type pyNode struct {
	client.Client
	namespace string
	name      string
	chain     string
	execer    exec.Interface
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
	if err := p.restore(); err != nil {
		return err
	}
	if action == node.StopAndStart {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *pyNode) restore() error {
	err := p.execer.Command("cp", "-r", citacloudv1.RestoreSourceVolumePath, fmt.Sprintf("%s/%s", citacloudv1.RestoreDestVolumePath, p.name)).Run()
	if err != nil {
		pyNodeLog.Error(err, "restore file failed")
		return err
	}

	pyNodeLog.Info("restore file completed")
	return nil
}

func (p *pyNode) Backup(ctx context.Context) error {
	err := p.Stop(ctx)
	if err != nil {
		return err
	}
	err = p.CheckStopped(ctx)
	if err != nil {
		return err
	}
	err = p.backup()
	if err != nil {
		return err
	}
	size, err := p.calculateSize()
	if err != nil {
		return err
	}

	annotations := map[string]string{"backup-size": size}
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

func (p *pyNode) backup() error {
	err := p.execer.Command("/bin/sh", "-c", fmt.Sprintf("cp -r %s/%s/* %s", citacloudv1.BackupSourceVolumePath, p.name, citacloudv1.BackupDestVolumePath)).Run()
	if err != nil {
		pyNodeLog.Error(err, "copy file failed")
		return err
	}
	pyNodeLog.Info("copy file completed")
	return nil
}

func (p *pyNode) calculateSize() (string, error) {
	// calculate size
	usageByte, err := p.execer.Command("du", "-sb", citacloudv1.BackupDestVolumePath).CombinedOutput()
	if err != nil {
		pyNodeLog.Info("calculate backup size failed")
		return "", err
	}
	usageStr := strings.Split(string(usageByte), "\t")
	pyNodeLog.Info(fmt.Sprintf("calculate backup size success: [%s]", usageStr[0]))
	return usageStr[0], nil
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

func (p *pyNode) Fallback(ctx context.Context, blockHeight int64) error {
	err := p.Stop(ctx)
	if err != nil {
		return err
	}
	err = p.CheckStopped(ctx)
	if err != nil {
		return err
	}
	err = p.fallback(blockHeight)
	if err != nil {
		return err
	}
	err = p.Start(ctx)
	if err != nil {
		return err
	}
	return err
}

func (p *pyNode) fallback(blockHeight int64) error {
	pyNodeLog.Info(fmt.Sprintf("exec block height fallback: [node: %s/%s, height: %d]...", p.namespace, p.name, blockHeight))
	err := p.execer.Command("cloud-op", "recover", fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("%s", citacloudv1.VolumeMountPath),
		"--config-path", fmt.Sprintf("%s/config.toml", citacloudv1.ConfigMountPath)).Run()
	if err != nil {
		pyNodeLog.Error(err, "exec block height fallback failed")
		return err
	}
	pyNodeLog.Info(fmt.Sprintf("exec block height fallback: [node: %s/%s, height: %d] successful", p.namespace, p.name, blockHeight))
	return nil
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
		namespace: namespace,
		name:      name,
		chain:     chain,
		execer:    execer,
	}, nil
}

func init() {
	node.Register(node.PythonOperator, newPyNode)
}
