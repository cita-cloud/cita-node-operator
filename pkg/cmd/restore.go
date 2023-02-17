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

type Restore struct {
	namespace    string
	chain        string
	node         string
	deployMethod string
	action       string
	sourcePath   string
	destPath     string
	decompress   bool
	md5          string
	input        string
}

var restore = Restore{}

func NewRestore() *cobra.Command {
	cc := &cobra.Command{
		Use:   "restore <subcommand>",
		Short: "Execute restore for node",
		Run:   restoreFunc,
	}
	cc.Flags().StringVarP(&restore.namespace, "namespace", "n", "default", "The node's node of namespace.")
	cc.Flags().StringVarP(&restore.chain, "chain", "c", "test-node", "The node name this node belongs to.")
	cc.Flags().StringVarP(&restore.node, "node", "", "", "The node that you want to restore.")
	cc.Flags().StringVarP(&restore.deployMethod, "deploy-method", "d", "cloud-config", "The node of deploy method.")
	cc.Flags().StringVarP(&restore.action, "action", "a", "StopAndStart", "The action when node restore.")
	cc.Flags().StringVarP(&restore.sourcePath, "source-path", "", "/restore-source", "The path you want to restore.")
	cc.Flags().StringVarP(&restore.destPath, "dest-path", "", "/restore-dest", "The path you want to save.")
	cc.Flags().BoolVarP(&restore.decompress, "decompress", "", false, "Decompress or not.")
	cc.Flags().StringVarP(&restore.md5, "md5", "", "", "Md5 check.")
	cc.Flags().StringVarP(&restore.input, "input", "", "", "Decompress file name.")
	return cc
}

func restoreFunc(cmd *cobra.Command, args []string) {
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
	switch restore.deployMethod {
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
	switch restore.action {
	case string(nodepkg.StopAndStart):
		action = nodepkg.StopAndStart
	case string(nodepkg.Direct):
		action = nodepkg.Direct
	default:
		setupLog.Error(fmt.Errorf("invalid parameter for action"), "invalid parameter for action")
		os.Exit(1)
	}

	dcprOpts := common.NewDecompressOptions(restore.decompress, restore.md5, restore.input)

	node, err := nodepkg.CreateNode(dm, restore.namespace, restore.node, k8sClient, restore.chain, exec.New())
	if err != nil {
		setupLog.Error(err, "unable to init node")
		os.Exit(1)
	}
	ctx := context.Background()
	err = common.AddLogToPodAnnotation(ctx, k8sClient, func() error {
		return node.Restore(ctx, action, restore.sourcePath, restore.destPath, dcprOpts)
	})
	if err != nil {
		setupLog.Error(err, "exec restore failed")
		os.Exit(1)
	}
	setupLog.Info("exec restore success")
}
