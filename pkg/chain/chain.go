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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BlockHeightFallbackActuator struct {
	client.Client
	Namespace         string
	ChainName         string
	ChainReplicas     *int32
	ChainDeployMethod DeployMethod
	NodeList          string
}

type Creator func(namespace, name string, client client.Client, nodeStr string) (Chain, error)

var chains = make(map[DeployMethod]Creator)

func Register(deployMethod DeployMethod, register Creator) {
	chains[deployMethod] = register
}

func CreateChain(deployMethod DeployMethod, namespace, name string, client client.Client, nodeStr string) (Chain, error) {
	f, ok := chains[deployMethod]
	if ok {
		return f(namespace, name, client, nodeStr)
	}
	return nil, fmt.Errorf("invalid deploy type: %s", string(deployMethod))
}

type Chain interface {
	InitResources(ctx context.Context) error
	Stop(ctx context.Context) error
	CheckStopped(ctx context.Context) error
	Fallback(ctx context.Context, blockHeight int64) error
	Start(ctx context.Context) error
}
