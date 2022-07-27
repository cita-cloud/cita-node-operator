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
	"fmt"
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
	"time"
)

const (
	ChainName      = "test-chain"
	NodeName       = "test-chain-1"
	ChainNamespace = "default"
	timeout        = time.Second * 10
	duration       = time.Second * 10
	interval       = time.Millisecond * 250
)

var _ = Describe("Fallback for python node", func() {
	Context("Exec fallback for python node", func() {
		It("Should fallback to specified block height", func() {
			By("Prepare a python node")
			createPythonChain(ctx)

			By("Create python node fallback interface")

			fcmd := fakeexec.FakeCmd{
				RunScript: []fakeexec.FakeAction{
					// Success.
					func() ([]byte, []byte, error) { return nil, nil, nil },
				},
			}
			fexec := fakeexec.FakeExec{
				CommandScript: []fakeexec.FakeCommandAction{
					func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
				},
			}

			chain, err := nodepkg.CreateNode(nodepkg.PythonOperator, ChainNamespace, NodeName, k8sClient, ChainName, &fexec)
			Expect(err).NotTo(HaveOccurred())
			err = chain.Fallback(ctx, 100)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should backup node data", func() {

			By("Create python node backup interface")

			fcmd := fakeexec.FakeCmd{
				RunScript: []fakeexec.FakeAction{
					// Success.
					func() ([]byte, []byte, error) { return nil, nil, nil },
					// Failure.
					//func() ([]byte, []byte, error) { return nil, nil, &fakeexec.FakeExitError{Status: 1} },
				},
			}
			fexec := fakeexec.FakeExec{
				CommandScript: []fakeexec.FakeCommandAction{
					func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
					//func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
				},
			}

			chain, err := nodepkg.CreateNode(nodepkg.PythonOperator, ChainNamespace, NodeName, k8sClient, ChainName, &fexec)
			Expect(err).NotTo(HaveOccurred())
			err = chain.Backup(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

	})
})

func createPythonChain(ctx context.Context) {
	for i := 0; i < 4; i++ {
		dep := &appsv1.Deployment{}
		dep.Name = fmt.Sprintf("%s-%d", ChainName, i)
		dep.Namespace = ChainNamespace

		labels := map[string]string{"chain_name": ChainName}
		dep.Labels = labels

		dep.Spec = appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "controller",
							Image: "image",
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "datadir",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "local-pvc",
									ReadOnly:  false,
								},
							},
						},
					},
				},
			},
		}

		Eventually(func() bool {
			err := k8sClient.Create(ctx, dep)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())
	}
}
