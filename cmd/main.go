package main

import (
	"context"
	"flag"
	fallback "github.com/cita-cloud/cita-node-operator/pkg"
	"os"
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
	flag.Parse()

	k8sClient, err := fallback.InitK8sClient()
	if err != nil {
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
		os.Exit(1)
	}
	os.Exit(0)
}
