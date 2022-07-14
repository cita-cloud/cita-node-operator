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
package controllers

import (
	"context"
	"fmt"
	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
	chainpkg "github.com/cita-cloud/cita-node-operator/pkg/chain"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"strconv"
	"time"
)

// +kubebuilder:docs-gen:collapse=Imports

const (
	ChainName      = "test-chain"
	ChainNamespace = "default"
	//BlockHeightFallbackName      = "blockheightfallback-sample"
	BlockHeightFallbackNamespace = "default"
	BlockHeight                  = 30

	timeout  = time.Second * 10
	duration = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("BlockHeightFallback controller", func() {

	Context("When create BlockHeightFallback for helm chain", func() {

		const (
			nodeList                            = "*"
			BlockHeightFallbackNameForHelmChain = "blockheightfallback-sample-for-helm-chain"
		)

		It("Should create a Job by controller", func() {
			By("Prepare a helm chain")

			createHelmChain(ctx)

			By("By creating a new BlockHeightFallback for helm chain")
			ctx := context.Background()
			bhf := &citacloudv1.BlockHeightFallback{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlockHeightFallbackNameForHelmChain,
					Namespace: BlockHeightFallbackNamespace,
				},
				Spec: citacloudv1.BlockHeightFallbackSpec{
					ChainName:         ChainName,
					Namespace:         ChainNamespace,
					BlockHeight:       BlockHeight,
					ChainDeployMethod: chainpkg.Helm,
					NodeList:          nodeList,
				},
			}
			Expect(k8sClient.Create(ctx, bhf)).Should(Succeed())

			bhfLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameForHelmChain, Namespace: BlockHeightFallbackNamespace}
			createdBhf := &citacloudv1.BlockHeightFallback{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, bhfLookupKey, createdBhf)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdBhf.Spec.ChainDeployMethod).Should(Equal(chainpkg.Helm))

			By("By checking that a new job has been created")
			jobLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameForHelmChain, Namespace: BlockHeightFallbackNamespace}
			createdJob := &v1.Job{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, jobLookupKey, createdJob)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("By checking that job volumes")

			volumes, err := bhfReconciler.getVolumes(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdJob.Spec.Template.Spec.Volumes).Should(Equal(volumes))

			By("By checking that job volumeMounts")

			Expect(1).Should(Equal(len(createdJob.Spec.Template.Spec.Containers)))

			mounts, err := bhfReconciler.getVolumeMounts(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())

			container := createdJob.Spec.Template.Spec.Containers[0]
			Expect(container.VolumeMounts).Should(Equal(mounts))

			By("By checking that job container parameters")
			args := []string{
				"fallback",
				"--namespace", ChainNamespace,
				"--chain-name", ChainName,
				"--deploy-method", string(chainpkg.Helm),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node-list", nodeList}
			Expect(container.Args).Should(Equal(args))
		})
	})

	Context("When create BlockHeightFallback for python chain", func() {

		const (
			nodeList                              = "*"
			BlockHeightFallbackNameForPythonChain = "blockheightfallback-sample-for-python-chain"
		)

		It("Should create a Job by controller", func() {
			By("Prepare a python chain")

			createPythonChain(ctx)

			By("By creating a new BlockHeightFallback for python chain")
			ctx := context.Background()
			bhf := &citacloudv1.BlockHeightFallback{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlockHeightFallbackNameForPythonChain,
					Namespace: BlockHeightFallbackNamespace,
				},
				Spec: citacloudv1.BlockHeightFallbackSpec{
					ChainName:         ChainName,
					Namespace:         ChainNamespace,
					BlockHeight:       BlockHeight,
					ChainDeployMethod: chainpkg.PythonOperator,
					NodeList:          nodeList,
				},
			}
			Expect(k8sClient.Create(ctx, bhf)).Should(Succeed())

			bhfLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameForPythonChain, Namespace: BlockHeightFallbackNamespace}
			createdBhf := &citacloudv1.BlockHeightFallback{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, bhfLookupKey, createdBhf)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdBhf.Spec.ChainDeployMethod).Should(Equal(chainpkg.PythonOperator))

			By("By checking that a new job has been created")
			jobLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameForPythonChain, Namespace: BlockHeightFallbackNamespace}
			createdJob := &v1.Job{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, jobLookupKey, createdJob)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("By checking that job volumes")

			volumes, err := bhfReconciler.getVolumes(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdJob.Spec.Template.Spec.Volumes).Should(Equal(volumes))

			By("By checking that job volumeMounts")

			Expect(1).Should(Equal(len(createdJob.Spec.Template.Spec.Containers)))

			mounts, err := bhfReconciler.getVolumeMounts(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())

			container := createdJob.Spec.Template.Spec.Containers[0]
			Expect(container.VolumeMounts).Should(Equal(mounts))

			By("By checking that job container parameters")
			args := []string{
				"fallback",
				"--namespace", ChainNamespace,
				"--chain-name", ChainName,
				"--deploy-method", string(chainpkg.PythonOperator),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node-list", nodeList}
			Expect(container.Args).Should(Equal(args))

		})
	})

	Context("When create BlockHeightFallback for cita-cloud chain", func() {

		const (
			nodeList                                 = "*"
			BlockHeightFallbackNameForCitaCloudChain = "blockheightfallback-sample-for-cita-cloud-chain"
		)

		It("Should create a Job by controller", func() {
			By("Prepare a cita-cloud chain")

			createCloudConfigChain(ctx)

			By("By creating a new BlockHeightFallback for cita-cloud chain")
			ctx := context.Background()
			bhf := &citacloudv1.BlockHeightFallback{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlockHeightFallbackNameForCitaCloudChain,
					Namespace: BlockHeightFallbackNamespace,
				},
				Spec: citacloudv1.BlockHeightFallbackSpec{
					ChainName:         ChainName,
					Namespace:         ChainNamespace,
					BlockHeight:       BlockHeight,
					ChainDeployMethod: chainpkg.CloudConfig,
					NodeList:          nodeList,
				},
			}
			Expect(k8sClient.Create(ctx, bhf)).Should(Succeed())

			bhfLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameForCitaCloudChain, Namespace: BlockHeightFallbackNamespace}
			createdBhf := &citacloudv1.BlockHeightFallback{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, bhfLookupKey, createdBhf)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdBhf.Spec.ChainDeployMethod).Should(Equal(chainpkg.CloudConfig))

			By("By checking that a new job has been created")
			jobLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameForCitaCloudChain, Namespace: BlockHeightFallbackNamespace}
			createdJob := &v1.Job{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, jobLookupKey, createdJob)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("By checking that job volumes")

			volumes, err := bhfReconciler.getVolumes(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(createdJob.Spec.Template.Spec.Volumes)).Should(Equal(len(volumes)))

			By("By checking that job volumeMounts")

			Expect(1).Should(Equal(len(createdJob.Spec.Template.Spec.Containers)))

			mounts, err := bhfReconciler.getVolumeMounts(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())

			container := createdJob.Spec.Template.Spec.Containers[0]
			Expect(len(container.VolumeMounts)).Should(Equal(len(mounts)))

			By("By checking that job container parameters")
			args := []string{
				"fallback",
				"--namespace", ChainNamespace,
				"--chain-name", ChainName,
				"--deploy-method", string(chainpkg.CloudConfig),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node-list", nodeList}
			Expect(container.Args).Should(Equal(args))
		})
	})

	Context("When create BlockHeightFallback specify one node for python chain", func() {
		oneNode := fmt.Sprintf("%s-3", ChainName)
		const (
			BlockHeightFallbackNameOneNodeForPythonChain = "blockheightfallback-sample-one-node-for-python-chain"
		)

		It("Should create a Job by controller", func() {

			By("By creating a new BlockHeightFallback for python chain")
			ctx := context.Background()
			bhf := &citacloudv1.BlockHeightFallback{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlockHeightFallbackNameOneNodeForPythonChain,
					Namespace: BlockHeightFallbackNamespace,
				},
				Spec: citacloudv1.BlockHeightFallbackSpec{
					ChainName:         ChainName,
					Namespace:         ChainNamespace,
					BlockHeight:       BlockHeight,
					ChainDeployMethod: chainpkg.PythonOperator,
					NodeList:          oneNode,
				},
			}
			Expect(k8sClient.Create(ctx, bhf)).Should(Succeed())

			bhfLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameOneNodeForPythonChain, Namespace: BlockHeightFallbackNamespace}
			createdBhf := &citacloudv1.BlockHeightFallback{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, bhfLookupKey, createdBhf)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdBhf.Spec.ChainDeployMethod).Should(Equal(chainpkg.PythonOperator))

			By("By checking that a new job has been created")
			jobLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameOneNodeForPythonChain, Namespace: BlockHeightFallbackNamespace}
			createdJob := &v1.Job{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, jobLookupKey, createdJob)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("By checking that job volumes")

			volumes, err := bhfReconciler.getVolumes(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdJob.Spec.Template.Spec.Volumes).Should(Equal(volumes))

			By("By checking that job volumeMounts")

			Expect(1).Should(Equal(len(createdJob.Spec.Template.Spec.Containers)))

			mounts, err := bhfReconciler.getVolumeMounts(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())

			container := createdJob.Spec.Template.Spec.Containers[0]
			Expect(container.VolumeMounts).Should(Equal(mounts))

			By("By checking that job container parameters")
			args := []string{
				"fallback",
				"--namespace", ChainNamespace,
				"--chain-name", ChainName,
				"--deploy-method", string(chainpkg.PythonOperator),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node-list", oneNode}
			Expect(container.Args).Should(Equal(args))
		})
	})

	Context("When create BlockHeightFallback specify two node for cita-cloud chain", func() {

		twoNode := fmt.Sprintf("%s-1,%s-3", ChainName, ChainName)
		const (
			BlockHeightFallbackNameTwoNodeForPythonChain = "blockheightfallback-sample-two-node-for-python-chain"
		)

		It("Should create a Job by controller", func() {

			By("By creating a new BlockHeightFallback for cita-cloud chain")
			ctx := context.Background()
			bhf := &citacloudv1.BlockHeightFallback{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlockHeightFallbackNameTwoNodeForPythonChain,
					Namespace: BlockHeightFallbackNamespace,
				},
				Spec: citacloudv1.BlockHeightFallbackSpec{
					ChainName:         ChainName,
					Namespace:         ChainNamespace,
					BlockHeight:       BlockHeight,
					ChainDeployMethod: chainpkg.CloudConfig,
					NodeList:          twoNode,
				},
			}
			Expect(k8sClient.Create(ctx, bhf)).Should(Succeed())

			bhfLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameTwoNodeForPythonChain, Namespace: BlockHeightFallbackNamespace}
			createdBhf := &citacloudv1.BlockHeightFallback{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, bhfLookupKey, createdBhf)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdBhf.Spec.ChainDeployMethod).Should(Equal(chainpkg.CloudConfig))

			By("By checking that a new job has been created")
			jobLookupKey := types.NamespacedName{Name: BlockHeightFallbackNameTwoNodeForPythonChain, Namespace: BlockHeightFallbackNamespace}
			createdJob := &v1.Job{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, jobLookupKey, createdJob)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("By checking that job volumes")

			volumes, err := bhfReconciler.getVolumes(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(createdJob.Spec.Template.Spec.Volumes)).Should(Equal(len(volumes)))

			By("By checking that job volumeMounts")

			Expect(1).Should(Equal(len(createdJob.Spec.Template.Spec.Containers)))

			mounts, err := bhfReconciler.getVolumeMounts(ctx, createdBhf)
			Expect(err).NotTo(HaveOccurred())

			container := createdJob.Spec.Template.Spec.Containers[0]
			Expect(len(container.VolumeMounts)).Should(Equal(len(mounts)))

			By("By checking that job container parameters")
			args := []string{
				"fallback",
				"--namespace", ChainNamespace,
				"--chain-name", ChainName,
				"--deploy-method", string(chainpkg.CloudConfig),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node-list", twoNode}
			Expect(container.Args).Should(Equal(args))
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

	Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
}

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

		Expect(k8sClient.Create(ctx, dep)).Should(Succeed())
	}
}

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
							Name: NodeConfigVolumeName,
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
						Name:      DataVolumeName,
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

		Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
	}
}
