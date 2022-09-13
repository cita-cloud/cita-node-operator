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
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	switchoverpkg "github.com/cita-cloud/cita-node-operator/pkg/switchover"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"k8s.io/utils/exec"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type switchover struct {
	namespace  string
	chain      string
	sourceNode string
	destNode   string
}

var swParameter = switchover{}

func NewSwitchover() *cobra.Command {
	cc := &cobra.Command{
		Use:   "switchover <subcommand>",
		Short: "Execute switchover for two nodes",
		Run:   switchoverFunc,
	}
	cc.Flags().StringVarP(&swParameter.namespace, "namespace", "n", "default", "The node's node of namespace.")
	cc.Flags().StringVarP(&swParameter.chain, "chain", "c", "test-node", "The node name this node belongs to.")
	cc.Flags().StringVarP(&swParameter.sourceNode, "source-node", "", "", "The source node that you want to switchover.")
	cc.Flags().StringVarP(&swParameter.destNode, "dest-node", "", "", "The destination node that you want to switchover.")
	return cc
}

func switchoverFunc(cmd *cobra.Command, args []string) {
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

	sourceNode, err := nodepkg.CreateNode(nodepkg.CloudConfig, swParameter.namespace, swParameter.sourceNode, k8sClient, swParameter.chain, exec.New())
	if err != nil {
		setupLog.Error(err, "unable to init source node")
		os.Exit(1)
	}
	destNode, err := nodepkg.CreateNode(nodepkg.CloudConfig, swParameter.namespace, swParameter.destNode, k8sClient, swParameter.chain, exec.New())
	if err != nil {
		setupLog.Error(err, "unable to init dest node")
		os.Exit(1)
	}
	swMgr := switchoverpkg.NewSwitchoverMgr()
	err = swMgr.Switch(context.Background(), sourceNode, destNode)
	if err != nil {
		setupLog.Error(err, "exec switchover failed")
		os.Exit(1)
	}
	setupLog.Info("exec switchover success")
}
