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
	"github.com/cita-cloud/cita-node-operator/pkg/chain"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	utilexec "k8s.io/utils/exec"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"
	"time"
)

var (
	pyChainLog = ctrl.Log.WithName("py-chain")
)

type pyChain struct {
	client.Client
	namespace string
	name      string
	nodeStr   string
	nodeObjs  []*appsv1.Deployment
}

func (p *pyChain) InitResources(ctx context.Context) error {
	deployList := &appsv1.DeploymentList{}
	deployOpts := []client.ListOption{
		client.InNamespace(p.namespace),
		client.MatchingLabels(map[string]string{"chain_name": p.name}),
	}
	if err := p.List(ctx, deployList, deployOpts...); err != nil {
		return err
	}
	if chain.AllNode(p.nodeStr) {
		for _, deploy := range deployList.Items {
			p.nodeObjs = append(p.nodeObjs, &deploy)
		}
	} else {
		for _, deploy := range deployList.Items {
			if val, ok := deploy.Labels["node_name"]; ok {
				if !ok {
					pyChainLog.Error(fmt.Errorf("the deployment %s doesn't have label: node_name", deploy.Name), "")
					continue
				}
				if strings.Contains(p.nodeStr, val) {
					p.nodeObjs = append(p.nodeObjs, &deploy)
				}
			}
		}
	}
	return nil
}

func (p *pyChain) Stop(ctx context.Context) error {
	pyChainLog.Info("stop chain for deployment...")
	for _, nodeObj := range p.nodeObjs {
		//deployResource := resource.(*appsv1.Deployment)
		nodeObj.Spec.Replicas = pointer.Int32(0)

		// update StatefulSet to scale replica 0
		err := p.Update(ctx, nodeObj)
		if err != nil {
			return err
		}
		pyChainLog.Info(fmt.Sprintf("scale down deployment [%s] to 0 successful", nodeObj.Name))
	}
	return nil
}

func (p *pyChain) CheckStopped(ctx context.Context) error {
	pyChainLog.Info("wait the chain's nodes stopped...")
	var checkStopped func(context.Context) (bool, error)
	checkStopped = func(ctx context.Context) (bool, error) {
		for _, nodeObj := range p.nodeObjs {
			//deployResource := resource.(*appsv1.Deployment)
			// todo
			if nodeObj.Status.ReadyReplicas != 0 {
				return false, nil
			}
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
	pyChainLog.Info("the chain's all node have stopped")
	return nil
}

func (p *pyChain) Fallback(ctx context.Context, blockHeight int64) error {
	err := p.Stop(ctx)
	if err != nil {
		pyChainLog.Error(err, "stop chain failed")
		return err
	}
	err = p.CheckStopped(ctx)
	if err != nil {
		pyChainLog.Error(err, "check chain stopped failed")
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(p.nodeObjs))
	for _, nodeObj := range p.nodeObjs {
		go p.fallback(nodeObj.Name, blockHeight, &wg)
	}
	wg.Wait()

	err = p.Start(ctx)
	if err != nil {
		pyChainLog.Error(err, "start chain failed")
		return err
	}
	return nil
}

func (p *pyChain) fallback(node string, blockHeight int64, wg *sync.WaitGroup) {
	pyChainLog.Info(fmt.Sprintf("exec block height fallback: [node: %s, height: %d]...", node, blockHeight))
	exec := utilexec.New()
	err := exec.Command("cloud-op", "recover", fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("/mnt/%s", node),
		"--config-path", fmt.Sprintf("/mnt/%s/config.toml", node)).Run()
	if err != nil {
		pyChainLog.Error(err, "exec block height fallback failed")
	}
	pyChainLog.Info(fmt.Sprintf("exec block height fallback: [node: %s, height: %d] successful", node, blockHeight))
	wg.Done()
}

func (p *pyChain) Start(ctx context.Context) error {
	pyChainLog.Info("start chain for deployment...")
	for _, nodeObj := range p.nodeObjs {
		//deployResource := resource.(*appsv1.Deployment)
		nodeObj.Spec.Replicas = pointer.Int32(1)

		// update StatefulSet to scale replica 0
		err := p.Update(ctx, nodeObj)
		if err != nil {
			return err
		}
		pyChainLog.Info(fmt.Sprintf("scale up deployment [%s] to 1 successful", nodeObj.Name))
	}
	return nil
}

func newPyChain(namespace, name string, client client.Client, nodeStr string) (chain.Chain, error) {
	return &pyChain{
		Client:    client,
		namespace: namespace,
		name:      name,
		nodeStr:   nodeStr,
	}, nil
}

func init() {
	chain.Register(chain.PythonOperator, newPyChain)
}
