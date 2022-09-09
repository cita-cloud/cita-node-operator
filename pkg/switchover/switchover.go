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

package switchover

import (
	"context"
	"fmt"
	"github.com/cita-cloud/cita-node-operator/pkg/node"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type switchoverMgr struct {
	client.Client
	sourceNode string
	destNode   string
	namespace  string
}

func NewSwitchoverMgr() *switchoverMgr {
	return &switchoverMgr{}
}

func (s *switchoverMgr) Switch(ctx context.Context, sourceNode, destNode node.Node) error {
	err := sourceNode.Stop(ctx)
	if err != nil {
		return err
	}
	err = sourceNode.CheckStopped(ctx)
	if err != nil {
		return err
	}
	err = destNode.Stop(ctx)
	if err != nil {
		return err
	}
	err = destNode.CheckStopped(ctx)
	if err != nil {
		return err
	}
	// swap account configmap for two node
	err = sourceNode.UpdateAccountConfigmap(ctx, fmt.Sprintf("%s-account", destNode.GetName()))
	if err != nil {
		return err
	}
	err = destNode.UpdateAccountConfigmap(ctx, fmt.Sprintf("%s-account", sourceNode.GetName()))
	if err != nil {
		return err
	}
	err = sourceNode.Start(ctx)
	if err != nil {
		return err
	}
	err = destNode.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}
