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
	chainpkg "github.com/cita-cloud/cita-node-operator/pkg/node"
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
	ChainName                    = "test-chain"
	ChainNamespace               = "default"
	BlockHeightFallbackNamespace = "default"
	BlockHeight                  = 30

	timeout  = time.Second * 10
	duration = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("BlockHeightFallback controller", func() {

	Context("When create BlockHeightFallback for helm node", func() {

		const (
			node                                = "test-chain-0"
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
					Chain:        ChainName,
					BlockHeight:  BlockHeight,
					DeployMethod: chainpkg.Helm,
					Node:         node,
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

			Expect(createdBhf.Spec.DeployMethod).Should(Equal(chainpkg.Helm))

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
				"--chain", ChainName,
				"--deploy-method", string(chainpkg.Helm),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node", node,
				"--crypto", "sm",
				"--consensus", "bft"}
			Expect(container.Args).Should(Equal(args))
		})
	})

	Context("When create BlockHeightFallback for python node", func() {

		const (
			node                                  = "test-chain-0"
			BlockHeightFallbackNameForPythonChain = "blockheightfallback-sample-for-python-chain"
		)

		It("Should create a Job by controller", func() {
			By("Prepare a python chain")

			createPythonChain(ctx, ChainName, false)

			By("By creating a new BlockHeightFallback for python node")
			ctx := context.Background()
			bhf := &citacloudv1.BlockHeightFallback{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlockHeightFallbackNameForPythonChain,
					Namespace: BlockHeightFallbackNamespace,
				},
				Spec: citacloudv1.BlockHeightFallbackSpec{
					Chain:        ChainName,
					BlockHeight:  BlockHeight,
					DeployMethod: chainpkg.PythonOperator,
					Node:         node,
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

			Expect(createdBhf.Spec.DeployMethod).Should(Equal(chainpkg.PythonOperator))

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
				"--chain", ChainName,
				"--deploy-method", string(chainpkg.PythonOperator),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node", node,
				"--crypto", "sm",
				"--consensus", "raft"}
			Expect(container.Args).Should(Equal(args))

		})
	})

	Context("When create BlockHeightFallback for cita-cloud node", func() {

		const (
			node                                     = "test-chain-0"
			BlockHeightFallbackNameForCitaCloudChain = "blockheightfallback-sample-for-cita-cloud-chain"
		)

		It("Should create a Job by controller", func() {
			By("Prepare a cita-cloud chain")

			createCloudConfigChain(ctx, ChainName)

			By("By creating a new BlockHeightFallback for cita-cloud node")
			ctx := context.Background()
			bhf := &citacloudv1.BlockHeightFallback{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlockHeightFallbackNameForCitaCloudChain,
					Namespace: BlockHeightFallbackNamespace,
				},
				Spec: citacloudv1.BlockHeightFallbackSpec{
					Chain:        ChainName,
					BlockHeight:  BlockHeight,
					DeployMethod: chainpkg.CloudConfig,
					Node:         node,
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

			Expect(createdBhf.Spec.DeployMethod).Should(Equal(chainpkg.CloudConfig))

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
				"--chain", ChainName,
				"--deploy-method", string(chainpkg.CloudConfig),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node", node,
				"--crypto", "sm",
				"--consensus", "overlord"}
			Expect(container.Args).Should(Equal(args))
		})
	})

	Context("When create BlockHeightFallback specify one node for python node", func() {
		oneNode := fmt.Sprintf("%s-3", ChainName)
		const (
			BlockHeightFallbackNameOneNodeForPythonChain = "blockheightfallback-sample-one-node-for-python-chain"
		)

		It("Should create a Job by controller", func() {

			By("By creating a new BlockHeightFallback for python node")
			ctx := context.Background()
			bhf := &citacloudv1.BlockHeightFallback{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BlockHeightFallbackNameOneNodeForPythonChain,
					Namespace: BlockHeightFallbackNamespace,
				},
				Spec: citacloudv1.BlockHeightFallbackSpec{
					Chain:        ChainName,
					BlockHeight:  BlockHeight,
					DeployMethod: chainpkg.PythonOperator,
					Node:         oneNode,
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

			Expect(createdBhf.Spec.DeployMethod).Should(Equal(chainpkg.PythonOperator))

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
				"--chain", ChainName,
				"--deploy-method", string(chainpkg.PythonOperator),
				"--block-height", strconv.FormatInt(BlockHeight, 10),
				"--node", oneNode,
				"--crypto", "sm",
				"--consensus", "raft"}
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
				Containers: []corev1.Container{
					{
						Name:  "consensus",
						Image: "citacloud/consensus_bft:v6.4.0",
					},
					{
						Name:  "kms",
						Image: "citacloud/kms_sm:v6.4.0",
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

	Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
}

func createPythonChain(ctx context.Context, chainName string, create bool) {
	// create pvc
	if create {
		localPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "local-pvc",
				Namespace: ChainNamespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceStorage: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI)},
				},
				StorageClassName: pointer.String("nfs-csi"),
			},
		}
		Expect(k8sClient.Create(ctx, localPVC)).Should(Succeed())
	}

	for i := 0; i < 4; i++ {
		dep := &appsv1.Deployment{}
		dep.Name = fmt.Sprintf("%s-%d", chainName, i)
		dep.Namespace = ChainNamespace

		labels := map[string]string{"chain_name": chainName, "node_name": fmt.Sprintf("%s-%d", chainName, i)}
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
						{
							Name:  "consensus",
							Image: "citacloud/consensus_raft:v6.5.0",
						},
						{
							Name:  "crypto",
							Image: "citacloud/crypto_sm:v6.5.0",
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

func createCloudConfigChain(ctx context.Context, chainName string) {
	for i := 0; i < 4; i++ {
		stsName := fmt.Sprintf("%s-%d", chainName, i)
		sts := &appsv1.StatefulSet{}
		sts.Name = stsName
		sts.Namespace = ChainNamespace

		labels := map[string]string{"app.kubernetes.io/chain-name": chainName}
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
					Containers: []corev1.Container{
						{
							Name:  "consensus",
							Image: "citacloud/consensus_overlord:v6.6.0",
						},
						{
							Name:  "crypto",
							Image: "citacloud/crypto_sm:v6.6.0",
						},
					},
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
