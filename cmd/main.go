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

package main

import (
	"context"
	"flag"
	chainpkg "github.com/cita-cloud/cita-node-operator/pkg/chain"
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
	var nodeList string
	flag.StringVar(&namespace, "namespace", "default", "The chain of namespace.")
	flag.StringVar(&chainName, "chain-name", "test-chain", "The chain of name.")
	flag.StringVar(&deployMethod, "deploy-method", "helm", "The chain of name.")
	flag.Int64Var(&blockHeight, "block-height", 9999999, "The block height you want to recover.")
	flag.StringVar(&nodeList, "node-list", "*", "The node or nodes that you want to fallback, like: node1,node2; If all, you can set *.")

	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
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
	}
	err = chain.Fallback(context.Background(), blockHeight)
	if err != nil {
		setupLog.Error(err, "exec block height fallback failed")
		os.Exit(1)
	}
	setupLog.Info("exec block height fallback success")
	os.Exit(0)
}
