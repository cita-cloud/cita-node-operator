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

package node

import (
	"context"
	"fmt"
	"github.com/cita-cloud/cita-node-operator/pkg/common"
	"k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Creator func(namespace, name string, client client.Client, chain string, execer exec.Interface) (Node, error)

var nodes = make(map[DeployMethod]Creator)

func Register(deployMethod DeployMethod, register Creator) {
	nodes[deployMethod] = register
}

func CreateNode(deployMethod DeployMethod, namespace, name string, client client.Client, chain string, execer exec.Interface) (Node, error) {
	f, ok := nodes[deployMethod]
	if ok {
		return f(namespace, name, client, chain, execer)
	}
	return nil, fmt.Errorf("invalid deploy type: %s", string(deployMethod))
}

type Node interface {
	Stop(ctx context.Context) error
	CheckStopped(ctx context.Context) error
	Fallback(ctx context.Context, action Action, blockHeight int64, crypto, consensus string, deleteConsensusData bool) error
	Start(ctx context.Context) error
	Backup(ctx context.Context, action Action, sourcePath string, destPath string, options *common.CompressOptions) error
	Restore(ctx context.Context, action Action, sourcePath string, destPath string, options *common.DecompressOptions, deleteConsensusData bool) error
	GetName() string
	GetAccountConfigmap(ctx context.Context) (string, error)
	UpdateAccountConfigmap(ctx context.Context, newConfigmap string) error
	Snapshot(ctx context.Context, action Action, blockHeight int64, crypto, consensus string) error
	SnapshotRecover(ctx context.Context, action Action, blockHeight int64, crypto, consensus string, deleteConsensusData bool) error
	ChangeOwner(ctx context.Context, action Action, uid, gid int64) error
}
