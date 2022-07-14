package cmd

import (
	"context"
	chainpkg "github.com/cita-cloud/cita-node-operator/pkg/chain"

	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	namespace    string
	chainName    string
	deployMethod string
	blockHeight  int64
	nodeList     string

	setupLog = ctrl.Log.WithName("setup")
)

func NewFallbackCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "fallback <subcommand>",
		Short: "Execute fallback for chain nodes",
		Run:   fallBackFunc,
	}
	cc.Flags().StringVarP(&namespace, "namespace", "n", "default", "The chain's node of namespace.")
	cc.Flags().StringVarP(&chainName, "chain-name", "c", "test-chain", "The chain name this node belongs to.")
	cc.Flags().StringVarP(&deployMethod, "deploy-method", "d", "helm", "The chain of deploy method.")
	cc.Flags().Int64VarP(&blockHeight, "block-height", "b", 999999999, "The block height you want to recover.")
	cc.Flags().StringVarP(&nodeList, "node-list", "l", "*", "The node or nodes that you want to fallback, like: node1,node2; If all, you can set *.")
	return cc
}

func fallBackFunc(cmd *cobra.Command, args []string) {
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	k8sClient, err := chainpkg.InitK8sClient()
	if err != nil {
		setupLog.Error(err, "unable to init k8s client")
		os.Exit(1)
	}

	var dm chainpkg.DeployMethod
	switch deployMethod {
	case string(chainpkg.Helm):
		dm = chainpkg.Helm
	case string(chainpkg.PythonOperator):
		dm = chainpkg.PythonOperator
	case string(chainpkg.CloudConfig):
		dm = chainpkg.CloudConfig
	}
	chain, err := chainpkg.CreateChain(dm, namespace, chainName, k8sClient, nodeList)
	if err != nil {
		setupLog.Error(err, "unable to init chain")
		os.Exit(1)
	}
	err = chain.Fallback(context.Background(), blockHeight)
	if err != nil {
		setupLog.Error(err, "exec block height fallback failed")
		os.Exit(1)
	}
	setupLog.Info("exec block height fallback success")
}
