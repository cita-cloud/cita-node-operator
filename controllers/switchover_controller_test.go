package controllers

import (
	"context"
	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Backup controller", func() {
	Context("When create backup for python chain", func() {

		const (
			node           = "backup-test-chain-0"
			switchoverName = "switchover-sample"
			namespace      = "default"
		)

		It("Should create a Job by controller", func() {
			By("Prepare a cita cloud chain")
			createCloudConfigChain(ctx, "test-chain-for-switch")

			ctx := context.Background()
			switchover := &citacloudv1.Switchover{
				ObjectMeta: metav1.ObjectMeta{
					Name:      switchoverName,
					Namespace: namespace,
				},
				Spec: citacloudv1.SwitchoverSpec{
					Chain:      BackupChainName,
					SourceNode: "das",
					DestNode:   "ds",
				},
			}
			Expect(k8sClient.Create(ctx, switchover)).Should(Succeed())

			switchoverLookupKey := types.NamespacedName{Name: switchoverName, Namespace: namespace}
			createdSwitchover := &citacloudv1.Switchover{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, switchoverLookupKey, createdSwitchover)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			//By("By checking that a new job has been created for backup")
			//jobLookupKey := types.NamespacedName{Name: switchoverName, Namespace: namespace}
			//createdJob := &v1.Job{}
			//Eventually(func() bool {
			//	err := k8sClient.Get(ctx, jobLookupKey, createdJob)
			//	if err != nil {
			//		return false
			//	}
			//	return true
			//}, timeout, interval).Should(BeTrue())
			//
			//// set job succeed
			//createdJob.Status.Succeeded = 1
			//Expect(k8sClient.Update(ctx, createdJob)).Should(Succeed())
		})
	})
})
