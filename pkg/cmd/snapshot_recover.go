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

package cmd

import (
	"context"
	"fmt"
	"github.com/cita-cloud/cita-node-operator/pkg/common"
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"k8s.io/utils/exec"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type SnapshotRecover struct {
	namespace    string
	chain        string
	node         string
	deployMethod string
	blockHeight  int64
	crypto       string
	consensus    string
}

var snapshotRecover = SnapshotRecover{}

func NewSnapshotRecover() *cobra.Command {
	cc := &cobra.Command{
		Use:   "snapshot-recover <subcommand>",
		Short: "Execute snapshot recover for node",
		Run:   snapshotRecoverFunc,
	}
	cc.Flags().StringVarP(&snapshotRecover.namespace, "namespace", "n", "default", "The node's node of namespace.")
	cc.Flags().StringVarP(&snapshotRecover.chain, "chain", "c", "test-node", "The node name this node belongs to.")
	cc.Flags().StringVarP(&snapshotRecover.node, "node", "", "", "The node that you want to restore.")
	cc.Flags().StringVarP(&snapshotRecover.deployMethod, "deploy-method", "d", "cloud-config", "The node of deploy method.")
	cc.Flags().Int64VarP(&snapshotRecover.blockHeight, "block-height", "b", 999999999, "The block height you want to recover.")
	cc.Flags().StringVarP(&snapshotRecover.crypto, "crypto", "", "sm", "The node of crypto. [sm/eth]")
	cc.Flags().StringVarP(&snapshotRecover.consensus, "consensus", "", "bft", "The node of consensus. [bft/raft]")
	return cc
}

func snapshotRecoverFunc(cmd *cobra.Command, args []string) {
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	k8sClient, err := nodepkg.InitK8sClient()
	if err != nil {
		setupLog.Error(err, "unable to init k8s client")
		os.Exit(1)
	}

	var dm nodepkg.DeployMethod
	switch snapshotRecover.deployMethod {
	case string(nodepkg.Helm):
		dm = nodepkg.Helm
	case string(nodepkg.PythonOperator):
		dm = nodepkg.PythonOperator
	case string(nodepkg.CloudConfig):
		dm = nodepkg.CloudConfig
	default:
		setupLog.Error(fmt.Errorf("invalid parameter for deploy method"), "invalid parameter for deploy method")
		os.Exit(1)
	}

	node, err := nodepkg.CreateNode(dm, snapshotRecover.namespace, snapshotRecover.node, k8sClient, snapshotRecover.chain, exec.New())
	if err != nil {
		setupLog.Error(err, "unable to init node")
		os.Exit(1)
	}
	ctx := context.Background()
	err = common.AddLogToPodAnnotation(ctx, k8sClient, func() error {
		return node.SnapshotRecover(ctx, snapshotRecover.blockHeight, snapshotRecover.crypto, snapshotRecover.consensus)
	})
	if err != nil {
		setupLog.Error(err, "exec snapshot recover failed")
		os.Exit(1)
	}
	setupLog.Info("exec snapshot recover success")
}
