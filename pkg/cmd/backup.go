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
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"k8s.io/utils/exec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/cita-cloud/cita-node-operator/pkg/common"
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
)

type Backup struct {
	namespace    string
	chain        string
	node         string
	deployMethod string
	action       string
	sourcePath   string
	destPath     string
	compress     bool
	compressType string
	output       string
}

var backup = Backup{}

func NewBackup() *cobra.Command {
	cc := &cobra.Command{
		Use:   "backup <subcommand>",
		Short: "Execute backup for node",
		Run:   backupFunc,
	}
	cc.Flags().StringVarP(&backup.namespace, "namespace", "n", "default", "The node's node of namespace.")
	cc.Flags().StringVarP(&backup.chain, "chain", "c", "test-node", "The node name this node belongs to.")
	cc.Flags().StringVarP(&backup.node, "node", "", "", "The node that you want to backup.")
	cc.Flags().StringVarP(&backup.deployMethod, "deploy-method", "d", "cloud-config", "The node of deploy method.")
	cc.Flags().StringVarP(&backup.action, "action", "a", "StopAndStart", "The action when node backup.")
	cc.Flags().StringVarP(&backup.sourcePath, "source-path", "", "/backup-source", "The path you want to backup.")
	cc.Flags().StringVarP(&backup.destPath, "dest-path", "", "/backup-dest", "The path you want to save.")
	cc.Flags().BoolVarP(&backup.compress, "compress", "", true, "Compress or not.")
	cc.Flags().StringVarP(&backup.compressType, "compress-type", "", "gzip", "Compress type.")
	cc.Flags().StringVarP(&backup.output, "output", "o", "", "Compressed file name.")
	return cc
}

func backupFunc(cmd *cobra.Command, args []string) {
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
	switch backup.deployMethod {
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
	switch backup.action {
	case string(nodepkg.StopAndStart):
		action = nodepkg.StopAndStart
	case string(nodepkg.Direct):
		action = nodepkg.Direct
	default:
		setupLog.Error(fmt.Errorf("invalid parameter for action"), "invalid parameter for action")
		os.Exit(1)
	}

	cprOpts := common.NewCompressOptions(backup.compress, backup.compressType, backup.output)

	node, err := nodepkg.CreateNode(dm, backup.namespace, backup.node, k8sClient, backup.chain, exec.New())
	if err != nil {
		setupLog.Error(err, "unable to init node")
		os.Exit(1)
	}
	ctx := context.Background()
	err = common.AddLogToPodAnnotation(ctx, k8sClient, func() error {
		return node.Backup(ctx, action, backup.sourcePath, backup.destPath, cprOpts)
	})
	if err != nil {
		setupLog.Error(err, "exec backup failed")
		os.Exit(1)
	}
	setupLog.Info("exec backup success")
}
