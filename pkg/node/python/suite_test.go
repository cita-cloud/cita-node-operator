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

package python

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	testscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
)

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Fallback Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	//Expect(os.Setenv("TEST_ASSET_KUBE_APISERVER", "/Users/zhujianqiang/Library/Application Support/io.kubebuilder.envtest/k8s/1.23.5-darwin-amd64/kube-apiserver")).To(Succeed())
	//Expect(os.Setenv("TEST_ASSET_ETCD", "/Users/zhujianqiang/Library/Application Support/io.kubebuilder.envtest/k8s/1.23.5-darwin-amd64/etcd")).To(Succeed())
	//Expect(os.Setenv("TEST_ASSET_KUBECTL", "/Users/zhujianqiang/Library/Application Support/io.kubebuilder.envtest/k8s/1.23.5-darwin-amd64/kubectl")).To(Succeed())

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: testscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

}, 60)

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())

	//Expect(os.Unsetenv("TEST_ASSET_KUBE_APISERVER")).To(Succeed())
	//Expect(os.Unsetenv("TEST_ASSET_ETCD")).To(Succeed())
	//Expect(os.Unsetenv("TEST_ASSET_KUBECTL")).To(Succeed())
})
