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

type Chown struct {
	namespace    string
	chain        string
	node         string
	deployMethod string
	action       string
	uid          int64
	gid          int64
}

var chown = Chown{}

func NewChown() *cobra.Command {
	cc := &cobra.Command{
		Use:   "chown <subcommand>",
		Short: "Execute chown for node files",
		Run:   chownFunc,
	}
	cc.Flags().StringVarP(&chown.namespace, "namespace", "n", "default", "The node's node of namespace.")
	cc.Flags().StringVarP(&chown.chain, "chain", "c", "test-node", "The node name this node belongs to.")
	cc.Flags().StringVarP(&chown.node, "node", "", "", "The node that you want to chown.")
	cc.Flags().StringVarP(&chown.deployMethod, "deploy-method", "d", "cloud-config", "The node of deploy method.")
	cc.Flags().StringVarP(&chown.action, "action", "a", "StopAndStart", "The action when node chown.")
	cc.Flags().Int64VarP(&chown.uid, "uid", "u", 1000, "The uid of chown.")
	cc.Flags().Int64VarP(&chown.gid, "gid", "g", 1000, "The gid of chown.")
	return cc
}

func chownFunc(cmd *cobra.Command, args []string) {
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
	switch chown.deployMethod {
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

	var action nodepkg.Action
	switch chown.action {
	case string(nodepkg.StopAndStart):
		action = nodepkg.StopAndStart
	case string(nodepkg.Direct):
		action = nodepkg.Direct
	default:
		setupLog.Error(fmt.Errorf("invalid parameter for action"), "invalid parameter for action")
		os.Exit(1)
	}

	node, err := nodepkg.CreateNode(dm, chown.namespace, chown.node, k8sClient, chown.chain, exec.New())
	if err != nil {
		setupLog.Error(err, "unable to init node")
		os.Exit(1)
	}
	ctx := context.Background()
	err = common.AddLogToPodAnnotation(ctx, k8sClient, func() error {
		return node.ChangeOwner(ctx, action, chown.uid, chown.gid)
	})
	if err != nil {
		setupLog.Error(err, "exec chown failed")
		os.Exit(1)
	}
	setupLog.Info("exec chown success")
}
