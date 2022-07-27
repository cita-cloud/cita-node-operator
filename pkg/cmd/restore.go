package cmd

import (
	"context"
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
	}

	node, err := nodepkg.CreateNode(dm, restore.namespace, restore.node, k8sClient, restore.chain, exec.New())
	if err != nil {
		setupLog.Error(err, "unable to init node")
		os.Exit(1)
	}
	err = node.Restore(context.Background())
	if err != nil {
		setupLog.Error(err, "exec restore failed")
		os.Exit(1)
	}
	setupLog.Info("exec restore success")
}
