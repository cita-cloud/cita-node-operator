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

package cloud_config

import (
	"context"
	"fmt"
	chain2 "github.com/cita-cloud/cita-node-operator/pkg/chain"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"time"
	//ctrls "github.com/cita-cloud/cita-node-operator/controllers"
)

const (
	ChainName      = "test-chain"
	ChainNamespace = "default"
	timeout        = time.Second * 10
	duration       = time.Second * 10
	interval       = time.Millisecond * 250
)

var _ = Describe("Fallback for cloud-config chain", func() {
	Context("Exec fallback for cloud-config chain", func() {
		It("Should fallback to specified block height", func() {
			By("Prepare a cloud-config chain")
			createCloudConfigChain(ctx)

			By("Create cloud-config chain fallback interface")
			chain, err := chain2.CreateChain(chain2.CloudConfig, ChainNamespace, ChainName, k8sClient, "*")
			Expect(err).NotTo(HaveOccurred())
			err = chain.Fallback(ctx, 100)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should fallback to specified block height", func() {

			By("Create cloud-config chain fallback interface")
			chain, err := chain2.CreateChain(chain2.CloudConfig, ChainNamespace, ChainName, k8sClient, fmt.Sprintf("%s-1,%s-3", ChainName, ChainName))
			Expect(err).NotTo(HaveOccurred())
			err = chain.Fallback(ctx, 100)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func createCloudConfigChain(ctx context.Context) {
	for i := 0; i < 4; i++ {
		stsName := fmt.Sprintf("%s-%d", ChainName, i)
		sts := &appsv1.StatefulSet{}
		sts.Name = stsName
		sts.Namespace = ChainNamespace

		labels := map[string]string{"app.kubernetes.io/chain-name": ChainName}
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
							Name: "node-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s--config", stsName)},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				// data pvc
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "datadir",
						Namespace: ChainNamespace,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: *resource.NewQuantity(22, resource.BinarySI),
							},
						},
						StorageClassName: pointer.String("nas-csi"),
					},
				},
			},
		}

		//Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Create(ctx, sts)
			if err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())
	}
}
