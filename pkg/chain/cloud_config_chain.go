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

package chain

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	utilexec "k8s.io/utils/exec"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"
	"time"
)

type cloudConfigChain struct {
	client.Client
	namespace string
	name      string
	nodeStr   string
	nodeObjs  []appsv1.StatefulSet
}

func (c *cloudConfigChain) InitResources(ctx context.Context) error {
	stsList := &appsv1.StatefulSetList{}
	stsOpts := []client.ListOption{
		client.InNamespace(c.namespace),
		client.MatchingLabels(map[string]string{"app.kubernetes.io/chain-name": c.name}),
	}
	if err := c.List(ctx, stsList, stsOpts...); err != nil {
		return err
	}
	if AllNode(c.nodeStr) {
		for _, sts := range stsList.Items {
			c.nodeObjs = append(c.nodeObjs, sts)
		}
	} else {
		for _, sts := range stsList.Items {
			if val, ok := sts.Labels["app.kubernetes.io/chain-node"]; ok {
				if !ok {
					fallbackLog.Error(fmt.Errorf("the statefuleset %s doesn't have label: app.kubernetes.io/chain-node", sts.Name), "")
					continue
				}
				if strings.Contains(c.nodeStr, val) {
					c.nodeObjs = append(c.nodeObjs, sts)
				}
			}
		}
	}
	return nil
}

func (c *cloudConfigChain) Stop(ctx context.Context) error {
	fallbackLog.Info("stop chain for statefulset...")
	for _, nodeObj := range c.nodeObjs {
		//stsResource := resource.(*appsv1.StatefulSet)
		nodeObj.Spec.Replicas = pointer.Int32(0)

		// update StatefulSet to scale replica 0
		err := c.Update(ctx, &nodeObj)
		if err != nil {
			return err
		}
		fallbackLog.Info(fmt.Sprintf("scale down statefulset [%s] to 0 successful", nodeObj.Name))
	}
	return nil
}

func (c *cloudConfigChain) CheckStopped(ctx context.Context) error {
	fallbackLog.Info("wait the chain's nodes stopped...")
	var checkStopped func(context.Context) (bool, error)
	checkStopped = func(ctx context.Context) (bool, error) {
		for _, nodeObj := range c.nodeObjs {
			//stsResource := resource.(*appsv1.StatefulSet)
			found := &appsv1.StatefulSet{}
			err := c.Get(ctx, types.NamespacedName{Name: nodeObj.Name, Namespace: nodeObj.Namespace}, found)
			if err != nil {
				return false, err
			}

			if found.Status.ReadyReplicas != 0 {
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
		return fmt.Errorf("wait statefulset replicas to 0 timeout")
	}
	fallbackLog.Info("the chain's all node have stopped")
	return nil
}

func (c *cloudConfigChain) Fallback(ctx context.Context, blockHeight int64) error {
	err := c.InitResources(ctx)
	if err != nil {
		return err
	}
	err = c.Stop(ctx)
	if err != nil {
		fallbackLog.Error(err, "stop chain failed")
		return err
	}
	err = c.CheckStopped(ctx)
	if err != nil {
		fallbackLog.Error(err, "check chain stopped failed")
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(c.nodeObjs))
	for _, nodeObj := range c.nodeObjs {
		go c.fallback(nodeObj.Name, blockHeight, &wg)
	}
	wg.Wait()

	err = c.Start(ctx)
	if err != nil {
		fallbackLog.Error(err, "start chain failed")
		return err
	}
	return nil
}

func (c *cloudConfigChain) fallback(node string, blockHeight int64, wg *sync.WaitGroup) {
	fallbackLog.Info(fmt.Sprintf("exec block height fallback: [node: %s, height: %d]...", node, blockHeight))
	exec := utilexec.New()
	err := exec.Command("cloud-op", "recover", fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("/%s-data", node),
		"--config-path", fmt.Sprintf("/%s-config/config.toml", node)).Run()
	if err != nil {
		fallbackLog.Error(err, "exec block height fallback failed")
	}
	fallbackLog.Info(fmt.Sprintf("exec block height fallback: [node: %s, height: %d] successful", node, blockHeight))
	wg.Done()
}

func (c *cloudConfigChain) Start(ctx context.Context) error {
	fallbackLog.Info("start chain for statefulset...")
	for _, nodeObj := range c.nodeObjs {
		latestObj := &appsv1.StatefulSet{}
		err := c.Get(ctx, types.NamespacedName{Name: nodeObj.Name, Namespace: nodeObj.Namespace}, latestObj)
		if err != nil {
			return err
		}

		latestObj.Spec.Replicas = pointer.Int32(1)

		// update StatefulSet to scale replica 0
		err = c.Update(ctx, latestObj)
		if err != nil {
			return err
		}
		fallbackLog.Info(fmt.Sprintf("scale up statefulset [%s] to 1 successful", latestObj.Name))
	}
	return nil
}

func newCloudConfigChain(namespace, name string, client client.Client, nodeStr string) (Chain, error) {
	return &cloudConfigChain{
		Client:    client,
		namespace: namespace,
		name:      name,
		nodeStr:   nodeStr,
		nodeObjs:  make([]appsv1.StatefulSet, 0),
	}, nil
}

func init() {
	Register(CloudConfig, newCloudConfigChain)
}
