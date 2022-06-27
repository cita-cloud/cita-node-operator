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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"strconv"

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

	// Check if the serviceAccount already exists, if not create a new one
	// todo

	// Check if the job already exists, if not create a new one
	found := &v1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: bhf.Name, Namespace: bhf.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
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
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "failed to get job")
		return ctrl.Result{}, err
	}

	// Update the BlockHeightFallback status
	jobList := &v1.JobList{}
	listOpts := []client.ListOption{
		client.InNamespace(bhf.Namespace),
		client.MatchingLabels(labelsForBlockHeightFallback(bhf)),
	}
	if err = r.List(ctx, jobList, listOpts...); err != nil {
		logger.Error(err, "failed to list jobs")
		return ctrl.Result{}, err
	}

	cur := bhf.DeepCopy()
	job := jobList.Items[0]
	if job.Status.Active == 1 {
		cur.Status.Status = citacloudv1.JobActive
	} else if job.Status.Failed == 1 {
		cur.Status.Status = citacloudv1.JobFailed
	} else if job.Status.Succeeded == 1 {
		cur.Status.Status = citacloudv1.JobComplete
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
	pvcName, err := r.getPVC(ctx, bhf)
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
			BackoffLimit: pointer.Int32(1),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: citacloudv1.JobServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "fallback",
							Image:           bhf.Spec.Image,
							ImagePullPolicy: bhf.Spec.PullPolicy,
							Command: []string{
								"/fallback",
							},
							Args: []string{
								"--namespace", bhf.Spec.Namespace,
								"--chain-name", bhf.Spec.ChainName,
								"--deploy-method", string(bhf.Spec.ChainDeployMethod),
								"--block-height", strconv.FormatInt(bhf.Spec.BlockHeight, 10),
								"--node-list", bhf.Spec.NodeList,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      citacloudv1.VolumeName,
									MountPath: citacloudv1.VolumeMountPath,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: citacloudv1.VolumeName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
									ReadOnly:  false,
								},
							},
						},
					},
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

func labelsForBlockHeightFallback(bhf *citacloudv1.BlockHeightFallback) map[string]string {
	return map[string]string{"app.kubernetes.io/chain-name": bhf.Spec.ChainName}
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
	return updateFlag
}

func (r *BlockHeightFallbackReconciler) setDefaultStatus(bhf *citacloudv1.BlockHeightFallback) bool {
	updateFlag := false
	if bhf.Status.Status == "" {
		bhf.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	return updateFlag
}

func (r *BlockHeightFallbackReconciler) getPVC(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (string, error) {
	// find chain's statefuleset
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: bhf.Spec.ChainName, Namespace: bhf.Spec.Namespace}, sts)
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

// SetupWithManager sets up the controller with the Manager.
func (r *BlockHeightFallbackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.BlockHeightFallback{}).
		Owns(&v1.Job{}).
		Complete(r)
}
