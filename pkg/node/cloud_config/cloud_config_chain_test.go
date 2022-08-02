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
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
	"k8s.io/utils/pointer"
	"os"
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

var _ = Describe("Test for cloud-config node", func() {
	Context("Perform action for cloud-config node", func() {
		It("Should fallback to specified block height", func() {
			By("Prepare a cloud-config node")
			createCloudConfigChain(ctx)

			By("Create cloud-config node fallback interface")

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

			chain, err := nodepkg.CreateNode(nodepkg.CloudConfig, ChainNamespace, NodeName, k8sClient, ChainName, &fexec)
			Expect(err).NotTo(HaveOccurred())
			err = chain.Fallback(ctx, 100)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should backup node data", func() {

			By("Create cloud-config node backup interface")

			fcmd := fakeexec.FakeCmd{
				RunScript: []fakeexec.FakeAction{
					// Success.
					func() ([]byte, []byte, error) { return nil, nil, nil },
					// Failure.
					//func() ([]byte, []byte, error) { return nil, nil, &fakeexec.FakeExitError{Status: 1} },
				},
				CombinedOutputScript: []fakeexec.FakeAction{
					func() ([]byte, []byte, error) {
						return []byte("1827315822\t/backup-dest\n"), nil, nil
					},
				},
			}
			fexec := fakeexec.FakeExec{
				CommandScript: []fakeexec.FakeCommandAction{
					func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
					func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
				},
			}

			chain, err := nodepkg.CreateNode(nodepkg.CloudConfig, ChainNamespace, NodeName, k8sClient, ChainName, &fexec)
			Expect(err).NotTo(HaveOccurred())
			err = chain.Backup(ctx, nodepkg.StopAndStart)
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

		labels := map[string]string{"app.kubernetes.io/node-name": ChainName}
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
									LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-config", stsName)},
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

	_ = os.Setenv("MY_POD_NAME", fmt.Sprintf("%s-0-cte", ChainName))
	_ = os.Setenv("MY_POD_NAMESPACE", ChainNamespace)

	pod := &corev1.Pod{}
	pod.Name = fmt.Sprintf("%s-0-cte", ChainName)
	pod.Namespace = ChainNamespace

	pod.Spec = corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "controller",
				Image: "image",
				Env: []corev1.EnvVar{
					{
						Name: "MY_POD_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						},
					},
					{
						Name: "MY_POD_NAMESPACE",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.namespace",
							},
						},
					},
				},
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
	}

	Eventually(func() bool {
		err := k8sClient.Create(ctx, pod)
		if err != nil {
			return false
		}
		return true
	}, timeout, interval).Should(BeTrue())
}
