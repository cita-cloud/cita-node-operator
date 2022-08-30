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
	nodepkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

// BlockHeightFallbackReconciler reconciles a BlockHeightFallback object
type BlockHeightFallbackReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=blockheightfallbacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=blockheightfallbacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=blockheightfallbacks/finalizers,verbs=update

func (r *BlockHeightFallbackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("fallback crd %s in reconcile", req.NamespacedName))

	bhf := &citacloudv1.BlockHeightFallback{}
	if err := r.Get(ctx, req.NamespacedName, bhf); err != nil {
		logger.Info(fmt.Sprintf("the fallback crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set default
	if r.setDefaultSpec(bhf) {
		err := r.Update(ctx, bhf)
		if err != nil {
			logger.Error(err, "set default spce failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if r.setDefaultStatus(bhf) {
		err := r.Status().Update(ctx, bhf)
		if err != nil {
			logger.Error(err, "set default status failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	jobRbac := newJobRbac(
		r.Client,
		log.FromContext(ctx),
		req.Namespace,
		citacloudv1.CITANodeJobServiceAccount,
		citacloudv1.CITANodeJobClusterRole,
		citacloudv1.CITANodeJobClusterRoleBinding)
	err := jobRbac.Ensure(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check if the job already exists, if not create a new one
	foundJob := &v1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: bhf.Name, Namespace: bhf.Namespace}, foundJob)

	if bhf.Status.Status == citacloudv1.JobComplete || bhf.Status.Status == citacloudv1.JobFailed {
		logger.Info(fmt.Sprintf("blockheightfallback status is finished: [%s]", bhf.Status.Status))
		// will delete job if job exist
		if err == nil && foundJob != nil {
			go CleanJob(ctx, r.Client, foundJob, bhf.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	if err != nil && errors.IsNotFound(err) {
		// Define a new job
		job, err := r.jobForBlockHeightFallback(ctx, bhf)
		if err != nil {
			logger.Error(err, "generate job resource failed")
			return ctrl.Result{}, err
		}
		logger.Info("creating a new Job")
		err = r.Create(ctx, job)
		if err != nil {
			logger.Error(err, "failed to create new Job")
			return ctrl.Result{}, err
		}
		// Job created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "failed to get job")
		return ctrl.Result{}, err
	}

	// todo
	job := &v1.Job{}
	err = r.Get(ctx, req.NamespacedName, job)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to get job %s/%s", bhf.Namespace, bhf.Name))
		return ctrl.Result{}, err
	}

	cur := bhf.DeepCopy()
	if job.Status.Active == 1 {
		cur.Status.Status = citacloudv1.JobActive
	} else if job.Status.Failed == 1 {
		cur.Status.Status = citacloudv1.JobFailed
		endTime := job.Status.Conditions[0].LastTransitionTime
		cur.Status.EndTime = &endTime
	} else if job.Status.Succeeded == 1 {
		cur.Status.Status = citacloudv1.JobComplete
		cur.Status.EndTime = job.Status.CompletionTime
	}
	if !IsEqual(cur, bhf) {
		logger.Info(fmt.Sprintf("update status: [%s]", cur.Status.Status))
		err := r.Status().Update(ctx, cur)
		if err != nil {
			logger.Error(err, "update status failed")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *BlockHeightFallbackReconciler) jobForBlockHeightFallback(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (*v1.Job, error) {
	labels := labelsForBlockHeightFallback(bhf)

	volumes, err := r.getVolumes(ctx, bhf)
	if err != nil {
		return nil, err
	}
	volumeMounts, err := r.getVolumeMounts(ctx, bhf)
	if err != nil {
		return nil, err
	}

	crypto, consensus, err := r.getCryptoAndConsensus(ctx, bhf)
	if err != nil {
		return nil, err
	}

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bhf.Name,
			Namespace: bhf.Namespace,
			Labels:    labels,
		},
		Spec: v1.JobSpec{
			BackoffLimit: pointer.Int32(0),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: citacloudv1.CITANodeJobServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "cita-node-cli",
							Image:           bhf.Spec.Image,
							ImagePullPolicy: bhf.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: []string{
								"fallback",
								"--namespace", bhf.Spec.Namespace,
								"--chain", bhf.Spec.Chain,
								"--deploy-method", string(bhf.Spec.DeployMethod),
								"--block-height", strconv.FormatInt(bhf.Spec.BlockHeight, 10),
								"--node", bhf.Spec.Node,
								"--crypto", crypto,
								"--consensus", consensus,
							},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
	// bind
	if err := ctrl.SetControllerReference(bhf, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *BlockHeightFallbackReconciler) getVolumes(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) ([]corev1.Volume, error) {
	if bhf.Spec.DeployMethod == nodepkg.Helm {
		pvcName, err := r.getHelmPVC(ctx, bhf)
		if err != nil {
			return nil, err
		}

		return []corev1.Volume{{
			Name: citacloudv1.VolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
					ReadOnly:  false,
				},
			},
		}}, nil
	} else if bhf.Spec.DeployMethod == nodepkg.PythonOperator {
		pvcName, err := r.getPyPVC(ctx, bhf)
		if err != nil {
			return nil, err
		}
		return []corev1.Volume{{
			Name: citacloudv1.VolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
					ReadOnly:  false,
				},
			},
		}}, nil
	} else if bhf.Spec.DeployMethod == nodepkg.CloudConfig {
		pvc, cm := r.getCloudConfigVolumes(ctx, bhf)
		vols := make([]corev1.Volume, 0)
		vols = append(vols, corev1.Volume{
			Name: citacloudv1.VolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc,
					ReadOnly:  false,
				},
			}})
		vols = append(vols, corev1.Volume{
			Name: citacloudv1.ConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm,
					},
				},
			}})
		return vols, nil
	} else {
		return nil, fmt.Errorf("mismatched deploy method")
	}
}

func (r *BlockHeightFallbackReconciler) getVolumeMounts(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) ([]corev1.VolumeMount, error) {
	if bhf.Spec.DeployMethod == nodepkg.Helm || bhf.Spec.DeployMethod == nodepkg.PythonOperator {
		return []corev1.VolumeMount{
			{
				Name:      citacloudv1.VolumeName,
				MountPath: citacloudv1.VolumeMountPath,
			},
		}, nil
	} else if bhf.Spec.DeployMethod == nodepkg.CloudConfig {
		volumeMounts := make([]corev1.VolumeMount, 0)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      citacloudv1.VolumeName,
			MountPath: citacloudv1.VolumeMountPath,
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      citacloudv1.ConfigName,
			MountPath: citacloudv1.ConfigMountPath,
		})
		return volumeMounts, nil
	}
	return nil, nil
}

func (r *BlockHeightFallbackReconciler) getCryptoAndConsensus(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (string, string, error) {
	var crypto, consensus string
	if bhf.Spec.DeployMethod == nodepkg.Helm {
		sts := &appsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{Name: bhf.Spec.Chain, Namespace: bhf.Spec.Namespace}, sts)
		if err != nil {
			return "", "", err
		}
		crypto, consensus = filterCryptoAndConsensus(sts.Spec.Template.Spec.Containers)
	} else if bhf.Spec.DeployMethod == nodepkg.PythonOperator {
		dep := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Name: bhf.Spec.Node, Namespace: bhf.Spec.Namespace}, dep)
		if err != nil {
			return "", "", err
		}
		crypto, consensus = filterCryptoAndConsensus(dep.Spec.Template.Spec.Containers)
	} else if bhf.Spec.DeployMethod == nodepkg.CloudConfig {
		sts := &appsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{Name: bhf.Spec.Node, Namespace: bhf.Spec.Namespace}, sts)
		if err != nil {
			return "", "", err
		}
		crypto, consensus = filterCryptoAndConsensus(sts.Spec.Template.Spec.Containers)
	} else {
		return "", "", fmt.Errorf("error deploy method")
	}
	return crypto, consensus, nil
}

func filterCryptoAndConsensus(containers []corev1.Container) (string, string) {
	var crypto, consensus string
	for _, container := range containers {
		if container.Name == "crypto" || container.Name == "kms" {
			if strings.Contains(container.Image, "sm") {
				crypto = "sm"
			} else if strings.Contains(container.Image, "eth") {
				crypto = "eth"
			}
		} else if container.Name == "consensus" {
			if strings.Contains(container.Image, "bft") {
				consensus = "bft"
			} else if strings.Contains(container.Image, "raft") {
				consensus = "raft"
			} else if strings.Contains(container.Image, "overlord") {
				consensus = "overlord"
			}
		}
	}
	return crypto, consensus
}

func labelsForBlockHeightFallback(bhf *citacloudv1.BlockHeightFallback) map[string]string {
	return map[string]string{"app.kubernetes.io/chain-name": bhf.Spec.Chain}
}

func (r *BlockHeightFallbackReconciler) setDefaultSpec(bhf *citacloudv1.BlockHeightFallback) bool {
	updateFlag := false
	if bhf.Spec.Image == "" {
		bhf.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if bhf.Spec.PullPolicy == "" {
		bhf.Spec.PullPolicy = corev1.PullIfNotPresent
	}
	if bhf.Spec.TTLSecondsAfterFinished == 0 {
		bhf.Spec.TTLSecondsAfterFinished = 30
		updateFlag = true
	}
	return updateFlag
}

func (r *BlockHeightFallbackReconciler) setDefaultStatus(bhf *citacloudv1.BlockHeightFallback) bool {
	updateFlag := false
	if bhf.Status.Status == "" {
		bhf.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	if bhf.Status.StartTime == nil {
		startTime := bhf.CreationTimestamp
		bhf.Status.StartTime = &startTime
		updateFlag = true
	}
	return updateFlag
}

func (r *BlockHeightFallbackReconciler) getHelmPVC(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (string, error) {
	// find chain's statefuleset
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: bhf.Spec.Chain, Namespace: bhf.Spec.Namespace}, sts)
	if err != nil {
		return "", err
	}
	for _, volume := range sts.Spec.Template.Spec.Volumes {
		if volume.Name == "datadir" {
			return volume.PersistentVolumeClaim.ClaimName, nil
		}
	}
	return "", fmt.Errorf("cann't find pvc name")
}

func (r *BlockHeightFallbackReconciler) getPyPVC(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (string, error) {
	// find chain's statefuleset
	deployList := &appsv1.DeploymentList{}
	deployOpts := []client.ListOption{
		client.InNamespace(bhf.Spec.Namespace),
		client.MatchingLabels(map[string]string{"chain_name": bhf.Spec.Chain}),
	}
	if err := r.List(ctx, deployList, deployOpts...); err != nil {
		return "", err
	}
	if len(deployList.Items) == 0 {
		return "", fmt.Errorf("get chain deployment items is 0")
	}
	for _, volume := range deployList.Items[0].Spec.Template.Spec.Volumes {
		if volume.Name == "datadir" {
			return volume.PersistentVolumeClaim.ClaimName, nil
		}
	}
	return "", fmt.Errorf("cann't find pvc name")
}

func (r *BlockHeightFallbackReconciler) getCloudConfigVolumes(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (string, string) {
	return fmt.Sprintf("datadir-%s-0", bhf.Spec.Node), fmt.Sprintf("%s-config", bhf.Spec.Node)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlockHeightFallbackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.BlockHeightFallback{}).
		Owns(&v1.Job{}, builder.WithPredicates(jobPredicate())).
		Complete(r)
}
