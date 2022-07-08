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
	"sync"
	"time"
)

type helmChain struct {
	client.Client
	namespace string
	name      string
	// statefulset's replica
	replicas *int32
	nodeStr  string
	nodeObjs []*appsv1.StatefulSet
}

func (h *helmChain) InitResources(ctx context.Context) error {
	stsList := &appsv1.StatefulSetList{}
	stsOpts := []client.ListOption{
		client.InNamespace(h.namespace),
		client.MatchingLabels(map[string]string{"app.kubernetes.io/instance": h.name}),
	}
	if err := h.List(ctx, stsList, stsOpts...); err != nil {
		return err
	}
	if len(stsList.Items) == 0 {
		return fmt.Errorf(fmt.Sprintf("cann't find statefuleset: %s/%s", h.namespace, h.name))
	}
	if len(stsList.Items) != 1 {
		return fmt.Errorf(fmt.Sprintf("find multi statefuleset for labels: [app.kubernetes.io/instance: %s]", h.name))
	}
	h.nodeObjs = append(h.nodeObjs, &stsList.Items[0])
	return nil
}

func (h *helmChain) Stop(ctx context.Context) error {
	fallbackLog.Info("stop chain for statefulset...")
	for _, nodeObj := range h.nodeObjs {
		// save replicas
		h.replicas = nodeObj.Spec.Replicas
		// update StatefulSet to scale replica 0
		nodeObj.Spec.Replicas = pointer.Int32(0)
		err := h.Update(ctx, nodeObj)
		if err != nil {
			return err
		}
	}
	fallbackLog.Info("scale down statefulset to 0 for chain successful")
	return nil
}

func (h *helmChain) CheckStopped(ctx context.Context) error {
	fallbackLog.Info("wait the chain's nodes stopped...")
	var checkStopped func(context.Context) (bool, error)
	checkStopped = func(ctx context.Context) (bool, error) {
		for _, nodeObj := range h.nodeObjs {
			found := &appsv1.StatefulSet{}
			err := h.Get(ctx, types.NamespacedName{Name: nodeObj.Name, Namespace: nodeObj.Namespace}, found)
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

func (h *helmChain) Fallback(ctx context.Context, blockHeight int64) error {
	err := h.InitResources(ctx)
	if err != nil {
		return err
	}
	err = h.Stop(ctx)
	if err != nil {
		fallbackLog.Error(err, "stop chain failed")
		return err
	}
	err = h.CheckStopped(ctx)
	if err != nil {
		fallbackLog.Error(err, "check chain stopped failed")
		return err
	}

	var wg sync.WaitGroup
	replicas := int(*h.replicas)
	wg.Add(replicas)
	for i := 0; i < replicas; i++ {
		go h.fallback(i, blockHeight, &wg)
	}
	wg.Wait()

	err = h.Start(ctx)
	if err != nil {
		fallbackLog.Error(err, "start chain failed")
		return err
	}

	return nil
}

func (h *helmChain) fallback(index int, blockHeight int64, wg *sync.WaitGroup) {
	fallbackLog.Info(fmt.Sprintf("exec block height fallback: [node: %s-%d, height: %d]...", h.name, index, blockHeight))
	exec := utilexec.New()
	err := exec.Command("cloud-op", "recover", fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("/mnt/%s-%d", h.name, index),
		"--config-path", fmt.Sprintf("/mnt/%s-%d/config.toml", h.name, index)).Run()
	if err != nil {
		fallbackLog.Error(err, "exec block height fallback failed")
	}
	fallbackLog.Info(fmt.Sprintf("exec block height fallback: [node: %s-%d, height: %d] successful", h.name, index, blockHeight))
	wg.Done()
}

func (h *helmChain) Start(ctx context.Context) error {
	fallbackLog.Info("start chain for statefulset...")
	for _, nodeObj := range h.nodeObjs {
		latestObj := &appsv1.StatefulSet{}
		err := h.Get(ctx, types.NamespacedName{Name: nodeObj.Name, Namespace: nodeObj.Namespace}, latestObj)
		if err != nil {
			return err
		}
		latestObj.Spec.Replicas = h.replicas
		err = h.Update(ctx, latestObj)
		if err != nil {
			return err
		}
		fallbackLog.Info(fmt.Sprintf("scale up statefulset to [%d] for chain successful", *h.replicas))
	}
	return nil
}

func newHelmChain(namespace, name string, client client.Client, nodeStr string) (Chain, error) {
	return &helmChain{
		Client:    client,
		namespace: namespace,
		name:      name,
		nodeStr:   nodeStr,
	}, nil
}

func init() {
	Register(Helm, newHelmChain)
}
