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
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

// RecoverReconciler reconciles a Recover object
type RecoverReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=recovers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=recovers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=recovers/finalizers,verbs=update

func (r *RecoverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("recover crd %s in reconcile", req.NamespacedName))

	recoverCR := &citacloudv1.Recover{}
	if err := r.Get(ctx, req.NamespacedName, recoverCR); err != nil {
		logger.Info(fmt.Sprintf("the recover crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set default
	if r.setDefaultSpec(recoverCR) {
		err := r.Update(ctx, recoverCR)
		if err != nil {
			logger.Error(err, "set default spce failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if r.setDefaultStatus(recoverCR) {
		err := r.Status().Update(ctx, recoverCR)
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
	err = r.Get(ctx, types.NamespacedName{Name: recoverCR.Name, Namespace: recoverCR.Namespace}, foundJob)

	if recoverCR.Status.Status == citacloudv1.JobComplete || recoverCR.Status.Status == citacloudv1.JobFailed {
		logger.Info(fmt.Sprintf("restore status is finished: [%s]", recoverCR.Status.Status))
		// will delete job if job exist
		if err == nil && foundJob != nil {
			go CleanJob(ctx, r.Client, foundJob, recoverCR.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	if err != nil && errors.IsNotFound(err) {
		job, err := r.jobForRecover(ctx, recoverCR)
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

	// todo
	job := &v1.Job{}
	err = r.Get(ctx, req.NamespacedName, job)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to get job %s/%s", recoverCR.Namespace, recoverCR.Name))
		return ctrl.Result{}, err
	}
	cur := recoverCR.DeepCopy()
	if job.Status.Active == 1 {
		cur.Status.Status = citacloudv1.JobActive
	} else if job.Status.Failed == 1 {
		// get error log message from pod annotations
		errLog, err := GetErrorLogFromPod(ctx, r.Client, recoverCR.Namespace, recoverCR.Name, string(job.UID))
		if err != nil {
			return ctrl.Result{}, err
		}
		cur.Status.Message = errLog
		cur.Status.Status = citacloudv1.JobFailed
		endTime := job.Status.Conditions[0].LastTransitionTime
		cur.Status.EndTime = &endTime
	} else if job.Status.Succeeded == 1 {
		cur.Status.Status = citacloudv1.JobComplete
		cur.Status.EndTime = job.Status.CompletionTime
	}
	if !IsEqual(cur, recoverCR) {
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

func (r *RecoverReconciler) setDefaultSpec(recoverCR *citacloudv1.Recover) bool {
	updateFlag := false
	if recoverCR.Spec.Action == "" {
		recoverCR.Spec.Action = nodepkg.StopAndStart
		updateFlag = true
	}
	if recoverCR.Spec.Image == "" {
		recoverCR.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if recoverCR.Spec.PullPolicy == "" {
		recoverCR.Spec.PullPolicy = corev1.PullIfNotPresent
		updateFlag = true
	}
	if recoverCR.Spec.TTLSecondsAfterFinished == 0 {
		recoverCR.Spec.TTLSecondsAfterFinished = 30
		updateFlag = true
	}
	return updateFlag
}

func (r *RecoverReconciler) setDefaultStatus(recoverCR *citacloudv1.Recover) bool {
	updateFlag := false
	if recoverCR.Status.Status == "" {
		recoverCR.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	if recoverCR.Status.StartTime == nil {
		startTime := recoverCR.CreationTimestamp
		recoverCR.Status.StartTime = &startTime
		updateFlag = true
	}
	return updateFlag
}

// SetupWithManager sets up the controller with the Manager.
func (r *RecoverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.Recover{}).
		Owns(&v1.Job{}, builder.WithPredicates(jobPredicate())).
		Complete(r)
}

func (r *RecoverReconciler) jobForRecover(ctx context.Context, recoverCR *citacloudv1.Recover) (*v1.Job, error) {
	labels := LabelsForNode(recoverCR.Spec.Chain, recoverCR.Spec.Node)

	var pvcDestName string

	if recoverCR.Spec.DeployMethod == nodepkg.PythonOperator {
		// todo
	} else if recoverCR.Spec.DeployMethod == nodepkg.CloudConfig {
		pvcDestName = fmt.Sprintf("datadir-%s-0", recoverCR.Spec.Node)
	}

	volumes := []corev1.Volume{
		{
			Name: citacloudv1.RestoreSourceVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: recoverCR.Spec.Backend.Pvc,
					ReadOnly:  false,
				},
			},
		},
		{
			Name: citacloudv1.RestoreDestVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcDestName,
					ReadOnly:  false,
				},
			},
		},
	}

	nodeKey, err := GetNodeLabelKeyByType(recoverCR.Spec.DeployMethod)
	if err != nil {
		return nil, err
	}

	arg := []string{
		"restore",
		"--namespace", recoverCR.Namespace,
		"--chain", recoverCR.Spec.Chain,
		"--node", recoverCR.Spec.Node,
		"--deploy-method", string(recoverCR.Spec.DeployMethod),
		"--action", string(recoverCR.Spec.Action),
		"--source-path", filepath.Join(citacloudv1.RestoreSourceVolumePath, recoverCR.Spec.Backend.Dir),
		"--dest-path", citacloudv1.RestoreDestVolumePath,
	}

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recoverCR.Name,
			Namespace: recoverCR.Namespace,
			Labels:    labels,
		},
		Spec: v1.JobSpec{
			BackoffLimit: pointer.Int32(0),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity:           SetAffinity(recoverCR.Spec.PodAffinityFlag, nodeKey, recoverCR.Spec.Node),
					ServiceAccountName: citacloudv1.CITANodeJobServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "cita-node-cli",
							Image:           recoverCR.Spec.Image,
							ImagePullPolicy: recoverCR.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: arg,
							Env: []corev1.EnvVar{
								{
									Name: POD_NAME_ENV,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: POD_NAMESPACE_ENV,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      citacloudv1.RestoreSourceVolumeName,
									MountPath: citacloudv1.RestoreSourceVolumePath,
								},
								{
									Name:      citacloudv1.RestoreDestVolumeName,
									MountPath: citacloudv1.RestoreDestVolumePath,
								},
							},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
	// bind
	if err := ctrl.SetControllerReference(recoverCR, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}
