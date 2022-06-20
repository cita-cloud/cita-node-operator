package pkg

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

type BlockHeightFallbackActuator struct {
	client.Client
	Namespace         string
	ChainName         string
	ChainReplicas     *int32
	ChainDeployMethod ChainDeployMethod
}

func NewBlockHeightFallbackActuator(namespace, chainName string, method ChainDeployMethod, k8sClient client.Client) Interface {
	return &BlockHeightFallbackActuator{
		Client:            k8sClient,
		Namespace:         namespace,
		ChainName:         chainName,
		ChainDeployMethod: method,
	}
}

type Interface interface {
	GetResource(ctx context.Context) (client.Object, error)
	StopChain(ctx context.Context) error
	CheckStopped(ctx context.Context) error
	Fallback(blockHeight int64)
	StartChain(ctx context.Context) error
	Run(ctx context.Context, blockHeight int64) error
}

func (b *BlockHeightFallbackActuator) GetResource(ctx context.Context) (client.Object, error) {
	stsList := &appsv1.StatefulSetList{}
	stsOpts := []client.ListOption{
		client.InNamespace(b.Namespace),
		client.MatchingLabels{"app.kubernetes.io/instance": b.ChainName},
	}
	if err := b.List(ctx, stsList, stsOpts...); err != nil {
		return nil, err
	}
	if len(stsList.Items) == 0 {
		fmt.Println("len = 0")
		return nil, fmt.Errorf(fmt.Sprintf("cann't find statefuleset: %s/%s", b.Namespace, b.ChainName))
	}
	if len(stsList.Items) != 1 {
		fmt.Println("len != 1")
		return nil, fmt.Errorf(fmt.Sprintf("find multi statefuleset for labels: [app.kubernetes.io/instance: %s]", b.ChainName))
	}
	return &stsList.Items[0], nil
}

func (b *BlockHeightFallbackActuator) StopChain(ctx context.Context) error {
	resource, err := b.GetResource(ctx)
	if err != nil {
		fmt.Println("get resource error")
		return err
	}
	stsResource := resource.(*appsv1.StatefulSet)
	// save replicas
	b.ChainReplicas = stsResource.Spec.Replicas

	stsResource.Spec.Replicas = pointer.Int32(0)
	// update StatefulSet to scale replica 0
	err = b.Update(ctx, stsResource)
	if err != nil {
		fmt.Println("update sts error")
		return err
	}
	return nil
}

func (b *BlockHeightFallbackActuator) CheckStopped(ctx context.Context) error {
	var checkStopped func(context.Context, string, string) (bool, error)
	checkStopped = func(ctx context.Context, s string, s2 string) (bool, error) {
		resource, err := b.GetResource(ctx)
		if err != nil {
			return false, err
		}
		stsResource := resource.(*appsv1.StatefulSet)
		if stsResource.Status.ReadyReplicas == 0 {
			return true, nil
		}
		return false, nil
	}
	// check it for 3 per second
	err := wait.Poll(3*time.Second, 60*time.Second, func() (done bool, err error) {
		return checkStopped(ctx, b.Namespace, b.ChainName)
	})
	if err != nil {
		return fmt.Errorf("wait statefulset replicas to 0 timeout")
	}
	return nil
}

func (b *BlockHeightFallbackActuator) Fallback(blockHeight int64) {
	var wg sync.WaitGroup
	replicas := int(*b.ChainReplicas)
	wg.Add(replicas)
	for i := 0; i < replicas; i++ {
		go b.fallback(i, blockHeight, &wg)
	}
	wg.Wait()
	return
}

func (b *BlockHeightFallbackActuator) fallback(index int, blockHeight int64, wg *sync.WaitGroup) {
	exec := utilexec.New()
	err := exec.Command("cloud-op", "recover", fmt.Sprintf("%d", blockHeight),
		"--node-root", fmt.Sprintf("/mnt/%s-%d", b.ChainName, index),
		"--config-path", fmt.Sprintf("/mnt/%s-%d/config.toml", b.ChainName, index)).Run()
	if err != nil {
		e := fmt.Sprintf("error: %v", err)
		fmt.Println(e)
	}
	wg.Done()
}

func (b *BlockHeightFallbackActuator) StartChain(ctx context.Context) error {
	resource, err := b.GetResource(ctx)
	if err != nil {
		return err
	}
	stsResource := resource.(*appsv1.StatefulSet)
	stsResource.Spec.Replicas = b.ChainReplicas

	// update StatefulSet to scale replica 0
	err = b.Update(ctx, stsResource)
	if err != nil {
		return err
	}
	return nil
}

func (b *BlockHeightFallbackActuator) Run(ctx context.Context, blockHeight int64) error {
	err := b.StopChain(ctx)
	if err != nil {
		fmt.Println("error stop chain")
		return err
	}
	err = b.CheckStopped(ctx)
	if err != nil {
		fmt.Println("error check stop chain")
		return err
	}
	b.Fallback(blockHeight)
	err = b.StartChain(ctx)
	if err != nil {
		fmt.Println("error start chain")
		return err
	}
	return nil
}
