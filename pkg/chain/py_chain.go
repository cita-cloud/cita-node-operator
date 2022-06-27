package chain

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type pyChain struct {
	client.Client
	namespace string
	name      string
}

func (p pyChain) GetResources(ctx context.Context) ([]client.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (p pyChain) Stop(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (p pyChain) CheckStopped(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (p pyChain) Fallback(ctx context.Context, blockHeight int64) error {
	//TODO implement me
	panic("implement me")
}

func (p pyChain) Start(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func newPyChain(namespace, name string, client client.Client) (Chain, error) {
	return &pyChain{
		Client:    nil,
		namespace: "",
		name:      "",
	}, nil
}

func init() {
	Register(PythonOperator, newPyChain)
}
