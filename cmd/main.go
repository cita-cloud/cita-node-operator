package main

import (
	"context"
	"flag"
	fallback "github.com/cita-cloud/cita-node-operator/pkg"
	"go.uber.org/zap/zapcore"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	var namespace string
	var chainName string
	var deployMethod string
	var blockHeight int64
	flag.StringVar(&namespace, "namespace", "default", "The chain of namespace.")
	flag.StringVar(&chainName, "chain-name", "test-chain", "The chain of name.")
	flag.StringVar(&deployMethod, "deploy-method", "helm", "The chain of name.")
	flag.Int64Var(&blockHeight, "block-height", 9999999, "The block height you want to recover.")

	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	k8sClient, err := fallback.InitK8sClient()
	if err != nil {
		setupLog.Error(err, "unable to init k8s client")
		os.Exit(1)
	}

	var dm fallback.ChainDeployMethod
	switch deployMethod {
	case string(fallback.Helm):
		dm = fallback.Helm
	case string(fallback.PythonOperator):
		dm = fallback.PythonOperator
	case string(fallback.CRDOperator):
		dm = fallback.CRDOperator
	}
	actuator := fallback.NewBlockHeightFallbackActuator(namespace, chainName, dm, k8sClient)
	err = actuator.Run(context.Background(), blockHeight)
	if err != nil {
		setupLog.Error(err, "exec block height fallback failed")
		os.Exit(1)
	}
	setupLog.Info("exec block height fallback success")
	os.Exit(0)
}
