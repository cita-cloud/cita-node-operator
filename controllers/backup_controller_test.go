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
	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
	chainpkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	BackupChainName      = "backup-test-chain"
	BackupChainNamespace = "default"
)

var _ = Describe("Backup controller", func() {
	Context("When create backup for python chain", func() {

		const (
			node                     = "backup-test-chain-0"
			BackupNameForPythonChain = "backup-sample-for-python-chain"
		)

		It("Should create a Job by controller", func() {
			By("Prepare a python chain")
			createPythonChain(ctx, BackupChainName, true)

			ctx := context.Background()
			backup := &citacloudv1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      BackupNameForPythonChain,
					Namespace: BackupChainNamespace,
				},
				Spec: citacloudv1.BackupSpec{
					Chain:        BackupChainName,
					Namespace:    BackupChainNamespace,
					DeployMethod: chainpkg.PythonOperator,
					Node:         node,
					StorageClass: "nfs-csi",
				},
			}
			Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

			backupLookupKey := types.NamespacedName{Name: BackupNameForPythonChain, Namespace: BackupChainNamespace}
			createdBackup := &citacloudv1.Backup{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, backupLookupKey, createdBackup)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(createdBackup.Spec.DeployMethod).Should(Equal(chainpkg.PythonOperator))

			By("By checking that a new job has been created for backup")
			jobLookupKey := types.NamespacedName{Name: BackupNameForPythonChain, Namespace: BackupChainNamespace}
			createdJob := &v1.Job{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, jobLookupKey, createdJob)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			// set job succeed
			createdJob.Status.Succeeded = 1
			Expect(k8sClient.Update(ctx, createdJob)).Should(Succeed())

			//Eventually(func() bool {
			//	err := k8sClient.Get(ctx, backupLookupKey, createdBackup)
			//	if err != nil {
			//		return false
			//	}
			//	if createdBackup.Status.Status == citacloudv1.JobComplete {
			//		return true
			//	}
			//	return false
			//}, timeout, interval).Should(BeTrue())

		})
	})
})
