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
	chainpkg "github.com/cita-cloud/cita-node-operator/pkg/node"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

// DuplicateReconciler reconciles a Duplicate object
type DuplicateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=duplicates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=duplicates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=duplicates/finalizers,verbs=update

func (r *DuplicateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("duplicate crd %s in reconcile", req.NamespacedName))

	duplicate := &citacloudv1.Duplicate{}
	if err := r.Get(ctx, req.NamespacedName, duplicate); err != nil {
		logger.Info(fmt.Sprintf("the duplicate crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set default
	if r.setDefaultSpec(duplicate) {
		err := r.Update(ctx, duplicate)
		if err != nil {
			logger.Error(err, "set default spce failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if r.setDefaultStatus(duplicate) {
		err := r.Status().Update(ctx, duplicate)
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
	err = r.Get(ctx, types.NamespacedName{Name: duplicate.Name, Namespace: duplicate.Namespace}, foundJob)

	if duplicate.Status.Status == citacloudv1.JobComplete || duplicate.Status.Status == citacloudv1.JobFailed {
		logger.Info(fmt.Sprintf("backup status is finished: [%s]", duplicate.Status.Status))
		// will delete job if job exist
		if err == nil && foundJob != nil {
			go CleanJob(ctx, r.Client, foundJob, duplicate.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	if err != nil && errors.IsNotFound(err) {
		// Define a new job
		job, err := r.jobForDuplicate(ctx, duplicate)
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
		logger.Error(err, fmt.Sprintf("failed to get job %s/%s", duplicate.Namespace, duplicate.Name))
		return ctrl.Result{}, err
	}
	cur := duplicate.DeepCopy()
	if job.Status.Active == 1 {
		cur.Status.Status = citacloudv1.JobActive
	} else if job.Status.Failed == 1 {
		// get error log message from pod annotations
		errLog, err := GetErrorLogFromPod(ctx, r.Client, duplicate.Namespace, duplicate.Name, string(job.UID))
		if err != nil {
			return ctrl.Result{}, err
		}
		cur.Status.Message = errLog
		cur.Status.Status = citacloudv1.JobFailed
		endTime := job.Status.Conditions[0].LastTransitionTime
		cur.Status.EndTime = &endTime
	} else if job.Status.Succeeded == 1 {
		cur.Status.Status = citacloudv1.JobComplete
		// get backup size from pod annotations
		podList := &corev1.PodList{}
		podOpts := []client.ListOption{
			client.InNamespace(duplicate.Namespace),
			client.MatchingLabels(map[string]string{"job-name": duplicate.Name, "controller-uid": string(job.UID)}),
		}
		if err := r.List(ctx, podList, podOpts...); err != nil {
			return ctrl.Result{}, err
		}
		if len(podList.Items) != 1 {
			logger.Error(err, fmt.Sprintf("job's pod != 1"))
			return ctrl.Result{}, err
		}
		backupSizeStr := podList.Items[0].Annotations["backup-size"]
		backupSize, err := strconv.ParseInt(backupSizeStr, 10, 64)
		if err != nil {
			logger.Error(err, "parse backup size from string to int64 failed")
			return ctrl.Result{}, err
		}
		cur.Status.Actual = backupSize
		cur.Status.EndTime = job.Status.CompletionTime
	}
	if !IsEqual(cur, duplicate) {
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

func (r *DuplicateReconciler) setDefaultSpec(duplicate *citacloudv1.Duplicate) bool {
	updateFlag := false
	if duplicate.Spec.Action == "" {
		duplicate.Spec.Action = chainpkg.StopAndStart
		updateFlag = true
	}
	if duplicate.Spec.Image == "" {
		duplicate.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if duplicate.Spec.PullPolicy == "" {
		duplicate.Spec.PullPolicy = corev1.PullIfNotPresent
		updateFlag = true
	}
	if duplicate.Spec.TTLSecondsAfterFinished == 0 {
		duplicate.Spec.TTLSecondsAfterFinished = 30
		updateFlag = true
	}
	return updateFlag
}

func (r *DuplicateReconciler) setDefaultStatus(duplicate *citacloudv1.Duplicate) bool {
	updateFlag := false
	if duplicate.Status.Status == "" {
		duplicate.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	if duplicate.Status.StartTime == nil {
		startTime := duplicate.CreationTimestamp
		duplicate.Status.StartTime = &startTime
		updateFlag = true
	}
	return updateFlag
}

// SetupWithManager sets up the controller with the Manager.
func (r *DuplicateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.Duplicate{}).
		Owns(&v1.Job{}, builder.WithPredicates(jobPredicate())).
		Complete(r)
}

func (r *DuplicateReconciler) jobForDuplicate(ctx context.Context, duplicate *citacloudv1.Duplicate) (*v1.Job, error) {
	labels := LabelsForNode(duplicate.Spec.Chain, duplicate.Spec.Node)

	var pvcSourceName string

	if duplicate.Spec.DeployMethod == chainpkg.PythonOperator {
		deploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: duplicate.Spec.Node, Namespace: duplicate.Namespace}, deploy); err != nil {
			return nil, err
		}
		volumes := deploy.Spec.Template.Spec.Volumes
		for _, volume := range volumes {
			if volume.Name == "datadir" {
				pvcSourceName = volume.PersistentVolumeClaim.ClaimName
				break
			}
		}
		if pvcSourceName == "" {
			return nil, fmt.Errorf("cann't get deployment's pvc")
		}
	} else {
		pvcSourceName = fmt.Sprintf("datadir-%s-0", duplicate.Spec.Node)
	}

	volumes := []corev1.Volume{
		{
			Name: citacloudv1.BackupSourceVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcSourceName,
					ReadOnly:  false,
				},
			},
		},
		{
			Name: citacloudv1.BackupDestVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: duplicate.Spec.Backend.Pvc,
					ReadOnly:  false,
				},
			},
		},
	}

	nodeKey, err := GetNodeLabelKeyByType(duplicate.Spec.DeployMethod)
	if err != nil {
		return nil, err
	}

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      duplicate.Name,
			Namespace: duplicate.Namespace,
			Labels:    labels,
		},
		Spec: v1.JobSpec{
			BackoffLimit: pointer.Int32(0),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity:           SetAffinity(duplicate.Spec.PodAffinityFlag, nodeKey, duplicate.Spec.Node),
					ServiceAccountName: citacloudv1.CITANodeJobServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "cita-node-cli",
							Image:           duplicate.Spec.Image,
							ImagePullPolicy: duplicate.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: []string{
								"backup",
								"--namespace", duplicate.Namespace,
								"--chain", duplicate.Spec.Chain,
								"--node", duplicate.Spec.Node,
								"--deploy-method", string(duplicate.Spec.DeployMethod),
								"--action", string(duplicate.Spec.Action),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      citacloudv1.BackupSourceVolumeName,
									MountPath: citacloudv1.BackupSourceVolumePath,
								},
								{
									Name:      citacloudv1.BackupDestVolumeName,
									MountPath: citacloudv1.BackupDestVolumePath,
									SubPath:   duplicate.Spec.Backend.Dir,
								},
							},
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
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
	// bind
	if err := ctrl.SetControllerReference(duplicate, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}
