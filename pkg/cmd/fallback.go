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
	"github.com/cita-cloud/cita-node-operator/pkg/common"
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	"k8s.io/utils/exec"

	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

type Fallback struct {
	namespace    string
	node         string
	chain        string
	deployMethod string
	blockHeight  int64
	crypto       string
	consensus    string
}

var fallback = Fallback{}

func NewFallbackCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "fallback <subcommand>",
		Short: "Execute fallback for chain nodes",
		Run:   fallBackFunc,
	}
	cc.Flags().StringVarP(&fallback.namespace, "namespace", "n", "default", "The node's node of namespace.")
	cc.Flags().StringVarP(&fallback.node, "node", "", "", "The node that you want to fallback.")
	cc.Flags().StringVarP(&fallback.chain, "chain", "c", "test-chain", "The node name this node belongs to.")
	cc.Flags().StringVarP(&fallback.deployMethod, "deploy-method", "d", "cloud-config", "The node of deploy method.")
	cc.Flags().Int64VarP(&fallback.blockHeight, "block-height", "b", 999999999, "The block height you want to recover.")
	cc.Flags().StringVarP(&fallback.crypto, "crypto", "", "sm", "The node of crypto. [sm/eth]")
	cc.Flags().StringVarP(&fallback.consensus, "consensus", "", "bft", "The node of consensus. [bft/raft]")
	return cc
}

func fallBackFunc(cmd *cobra.Command, args []string) {
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
	switch fallback.deployMethod {
	case string(nodepkg.Helm):
		dm = nodepkg.Helm
	case string(nodepkg.PythonOperator):
		dm = nodepkg.PythonOperator
	case string(nodepkg.CloudConfig):
		dm = nodepkg.CloudConfig
	}
	node, err := nodepkg.CreateNode(dm, fallback.namespace, fallback.node, k8sClient, fallback.chain, exec.New())
	if err != nil {
		setupLog.Error(err, "unable to init node")
		os.Exit(1)
	}
	ctx := context.Background()
	err = common.AddLogToPodAnnotation(ctx, k8sClient, func() error {
		return node.Fallback(ctx, fallback.blockHeight, fallback.crypto, fallback.consensus)
	})
	if err != nil {
		setupLog.Error(err, "exec block height fallback failed")
		os.Exit(1)
	}
	setupLog.Info("exec block height fallback success")
}
