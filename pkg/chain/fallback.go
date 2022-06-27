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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	fallbackLog = ctrl.Log.WithName("fallback")
)

type BlockHeightFallbackActuator struct {
	client.Client
	Namespace         string
	ChainName         string
	ChainReplicas     *int32
	ChainDeployMethod ChainDeployMethod
	NodeList          string
}

type Creator func(namespace, name string, client client.Client) (Chain, error)

var chains = make(map[ChainDeployMethod]Creator)

func Register(deployMethod ChainDeployMethod, register Creator) {
	chains[deployMethod] = register
}

func CreateChain(deployMethod ChainDeployMethod, namespace, name string, client client.Client) (Chain, error) {
	f, ok := chains[deployMethod]
	if ok {
		return f(namespace, name, client)
	}
	return nil, fmt.Errorf("invalid deploy type: %s", string(deployMethod))
}

type Chain interface {
	GetResources(ctx context.Context) ([]client.Object, error)
	Stop(ctx context.Context) error
	CheckStopped(ctx context.Context) error
	Fallback(ctx context.Context, blockHeight int64) error
	Start(ctx context.Context) error
}
