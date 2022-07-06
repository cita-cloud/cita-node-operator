package chain

import (
	"context"
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
			chain, err := CreateChain(Helm, ChainNamespace, ChainName, k8sClient, "*")
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
