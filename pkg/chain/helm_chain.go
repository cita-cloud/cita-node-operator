package chain

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
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
	replicas  *int32
}

func (h *helmChain) GetResources(ctx context.Context) ([]client.Object, error) {
	stsList := &appsv1.StatefulSetList{}
	stsOpts := []client.ListOption{
		client.InNamespace(h.namespace),
		client.MatchingLabels{"app.kubernetes.io/instance": h.name},
	}
	if err := h.List(ctx, stsList, stsOpts...); err != nil {
		return nil, err
	}
	if len(stsList.Items) == 0 {
		return nil, fmt.Errorf(fmt.Sprintf("cann't find statefuleset: %s/%s", h.namespace, h.name))
	}
	if len(stsList.Items) != 1 {
		return nil, fmt.Errorf(fmt.Sprintf("find multi statefuleset for labels: [app.kubernetes.io/instance: %s]", h.name))
	}
	res := make([]client.Object, 0)
	res = append(res, &stsList.Items[0])
	return res, nil
}

func (h *helmChain) Stop(ctx context.Context) error {
	fallbackLog.Info("stop chain for statefulset...")
	resources, err := h.GetResources(ctx)
	if err != nil {
		return err
	}
	stsResource := resources[0].(*appsv1.StatefulSet)
	// save replicas
	h.replicas = stsResource.Spec.Replicas

	stsResource.Spec.Replicas = pointer.Int32(0)
	// update StatefulSet to scale replica 0
	err = h.Update(ctx, stsResource)
	if err != nil {
		return err
	}
	fallbackLog.Info("scale down statefulset to 0 for chain successful")
	return nil
}

func (h *helmChain) CheckStopped(ctx context.Context) error {
	fallbackLog.Info("wait the chain's nodes stopped...")
	var checkStopped func(context.Context) (bool, error)
	checkStopped = func(ctx context.Context) (bool, error) {
		resources, err := h.GetResources(ctx)
		if err != nil {
			return false, err
		}
		stsResource := resources[0].(*appsv1.StatefulSet)
		if stsResource.Status.ReadyReplicas == 0 {
			return true, nil
		}
		return false, nil
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
	err := h.Stop(ctx)
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
	resources, err := h.GetResources(ctx)
	if err != nil {
		return err
	}
	stsResource := resources[0].(*appsv1.StatefulSet)
	stsResource.Spec.Replicas = h.replicas

	// update StatefulSet to scale replica 0
	err = h.Update(ctx, stsResource)
	if err != nil {
		return err
	}
	fallbackLog.Info(fmt.Sprintf("scale up statefulset to [%d] for chain successful", *h.replicas))
	return nil
}

//func (h *helmChain) Run(ctx context.Context, blockHeight int64) error {
//	err := h.Stop(ctx)
//	if err != nil {
//		fallbackLog.Error(err, "stop chain failed")
//		return err
//	}
//	err = h.CheckStopped(ctx)
//	if err != nil {
//		fallbackLog.Error(err, "check chain stopped failed")
//		return err
//	}
//	h.Fallback(blockHeight)
//	err = h.Start(ctx)
//	if err != nil {
//		fallbackLog.Error(err, "start chain failed")
//		return err
//	}
//	return nil
//}

func newHelmChain(namespace, name string, client client.Client) (Chain, error) {
	return &helmChain{
		Client:    client,
		namespace: namespace,
		name:      name,
	}, nil
}

func init() {
	Register(Helm, newHelmChain)
}
