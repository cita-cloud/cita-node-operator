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

package helm

import (
	"context"
	chain2 "github.com/cita-cloud/cita-node-operator/pkg/chain"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

const (
	ChainName      = "test-chain"
	ChainNamespace = "default"
	timeout        = time.Second * 10
	duration       = time.Second * 10
	interval       = time.Millisecond * 250
)

var _ = Describe("Fallback for helm chain", func() {
	Context("Exec fallback for helm chain", func() {
		It("Should fallback to specified block height", func() {
			By("Prepare a helm chain")
			createHelmChain(ctx)

			By("Create helm chain fallback interface")
			chain, err := chain2.CreateChain(chain2.Helm, ChainNamespace, ChainName, k8sClient, "*")
			Expect(err).NotTo(HaveOccurred())
			err = chain.Fallback(ctx, 100)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func createHelmChain(ctx context.Context) {
	sts := &appsv1.StatefulSet{}
	sts.Name = ChainName
	sts.Namespace = ChainNamespace

	labels := map[string]string{"app.kubernetes.io/instance": ChainName, "app.kubernetes.io/name": "cita-cloud-local-cluster"}
	sts.Labels = labels

	sts.Spec = appsv1.StatefulSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
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
		err := k8sClient.Create(ctx, sts)
		if err != nil {
			return false
		}
		return true
	}, timeout, interval).Should(BeTrue())
}
