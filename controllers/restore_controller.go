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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	snapshot *citacloudv1.Snapshot
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=restores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=restores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=restores/finalizers,verbs=update

func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("restore crd %s in reconcile", req.NamespacedName))

	restore := &citacloudv1.Restore{}
	if err := r.Get(ctx, req.NamespacedName, restore); err != nil {
		logger.Info(fmt.Sprintf("the restore crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set default
	if r.setDefaultSpec(restore) {
		err := r.Update(ctx, restore)
		if err != nil {
			logger.Error(err, "set default spce failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// check
	if restore.Spec.Backup != "" {
		backup := &citacloudv1.Backup{}
		err := r.Get(ctx, types.NamespacedName{Name: restore.Spec.Backup, Namespace: restore.Namespace}, backup)
		if err != nil {
			logger.Error(err, fmt.Sprintf("get backup %s/%s failed", restore.Namespace, restore.Spec.Backup))
			return ctrl.Result{}, err
		}
	}
	if restore.Spec.Snapshot != "" {
		snapshot := &citacloudv1.Snapshot{}
		err := r.Get(ctx, types.NamespacedName{Name: restore.Spec.Snapshot, Namespace: restore.Namespace}, snapshot)
		if err != nil {
			logger.Error(err, fmt.Sprintf("get snapshot %s/%s failed", restore.Namespace, restore.Spec.Snapshot))
			return ctrl.Result{}, err
		}
		r.snapshot = snapshot
	}

	if r.setDefaultStatus(restore) {
		err := r.Status().Update(ctx, restore)
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
	err = r.Get(ctx, types.NamespacedName{Name: restore.Name, Namespace: restore.Namespace}, foundJob)

	if restore.Status.Status == citacloudv1.JobComplete || restore.Status.Status == citacloudv1.JobFailed {
		logger.Info(fmt.Sprintf("restore status is finished: [%s]", restore.Status.Status))
		// will delete job if job exist
		if err == nil && foundJob != nil {
			go CleanJob(ctx, r.Client, foundJob, restore.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		var job *v1.Job
		if restore.Spec.Backup != "" {
			job, err = r.jobForRestore(ctx, restore)
			if err != nil {
				logger.Error(err, "generate job resource failed")
				return ctrl.Result{}, err
			}
		}
		if restore.Spec.Snapshot != "" {
			job, err = r.jobForSnapshotRecover(ctx, restore)
			if err != nil {
				logger.Error(err, "generate job resource failed")
				return ctrl.Result{}, err
			}
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
		logger.Error(err, fmt.Sprintf("failed to get job %s/%s", restore.Namespace, restore.Name))
		return ctrl.Result{}, err
	}
	cur := restore.DeepCopy()
	if job.Status.Active == 1 {
		cur.Status.Status = citacloudv1.JobActive
	} else if job.Status.Failed == 1 {
		// get error log message from pod annotations
		errLog, err := GetErrorLogFromPod(ctx, r.Client, restore.Namespace, restore.Name, string(job.UID))
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
	if !IsEqual(cur, restore) {
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

func (r *RestoreReconciler) setDefaultSpec(restore *citacloudv1.Restore) bool {
	updateFlag := false
	if restore.Spec.Action == "" {
		restore.Spec.Action = nodepkg.StopAndStart
		updateFlag = true
	}
	if restore.Spec.Image == "" {
		restore.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if restore.Spec.PullPolicy == "" {
		restore.Spec.PullPolicy = corev1.PullIfNotPresent
		updateFlag = true
	}
	if restore.Spec.TTLSecondsAfterFinished == 0 {
		restore.Spec.TTLSecondsAfterFinished = 30
		updateFlag = true
	}
	return updateFlag
}

func (r *RestoreReconciler) setDefaultStatus(restore *citacloudv1.Restore) bool {
	updateFlag := false
	if restore.Status.Status == "" {
		restore.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	if restore.Status.StartTime == nil {
		startTime := restore.CreationTimestamp
		restore.Status.StartTime = &startTime
		updateFlag = true
	}
	return updateFlag
}

func (r *RestoreReconciler) jobForRestore(ctx context.Context, restore *citacloudv1.Restore) (*v1.Job, error) {
	labels := LabelsForNode(restore.Spec.Chain, restore.Spec.Node)

	var pvcDestName string

	if restore.Spec.DeployMethod == nodepkg.PythonOperator {
		// todo
	} else if restore.Spec.DeployMethod == nodepkg.CloudConfig {
		pvcDestName = fmt.Sprintf("datadir-%s-0", restore.Spec.Node)
	}

	volumes := []corev1.Volume{
		{
			Name: citacloudv1.RestoreSourceVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: restore.Spec.Backup,
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

	nodeKey, err := GetNodeLabelKeyByType(restore.Spec.DeployMethod)
	if err != nil {
		return nil, err
	}

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restore.Name,
			Namespace: restore.Namespace,
			Labels:    labels,
		},
		Spec: v1.JobSpec{
			BackoffLimit: pointer.Int32(0),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity:           SetAffinity(restore.Spec.PodAffinityFlag, nodeKey, restore.Spec.Node),
					ServiceAccountName: citacloudv1.CITANodeJobServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "cita-node-cli",
							Image:           restore.Spec.Image,
							ImagePullPolicy: restore.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: r.buildArgsForFile(restore),
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
	if err := ctrl.SetControllerReference(restore, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *RestoreReconciler) jobForSnapshotRecover(ctx context.Context, restore *citacloudv1.Restore) (*v1.Job, error) {
	labels := LabelsForNode(restore.Spec.Chain, restore.Spec.Node)

	var pvcDestName string
	var crypto, consensus string

	if restore.Spec.DeployMethod == nodepkg.PythonOperator {
		// todo
	} else if restore.Spec.DeployMethod == nodepkg.CloudConfig {
		sts := &appsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: restore.Namespace,
			Name:      restore.Spec.Node,
		}, sts)
		if err != nil {
			return nil, err
		}
		crypto, consensus = filterCryptoAndConsensus(sts.Spec.Template.Spec.Containers)
		pvcDestName = fmt.Sprintf("datadir-%s-0", restore.Spec.Node)
	}

	volumes := []corev1.Volume{
		{
			Name: citacloudv1.RestoreSourceVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: restore.Spec.Snapshot,
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
		{
			// configmap
			Name: citacloudv1.ConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-config", restore.Spec.Node),
					},
				},
			},
		},
	}

	nodeKey, err := GetNodeLabelKeyByType(restore.Spec.DeployMethod)
	if err != nil {
		return nil, err
	}

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restore.Name,
			Namespace: restore.Namespace,
			Labels:    labels,
		},
		Spec: v1.JobSpec{
			BackoffLimit: pointer.Int32(0),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity:           SetAffinity(restore.Spec.PodAffinityFlag, nodeKey, restore.Spec.Node),
					ServiceAccountName: citacloudv1.CITANodeJobServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "cita-node-cli",
							Image:           restore.Spec.Image,
							ImagePullPolicy: restore.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: r.buildArgsForSnapshot(restore, crypto, consensus),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      citacloudv1.RestoreSourceVolumeName,
									MountPath: citacloudv1.RestoreSourceVolumePath,
								},
								{
									Name:      citacloudv1.RestoreDestVolumeName,
									MountPath: citacloudv1.RestoreDestVolumePath,
								},
								{
									Name:      citacloudv1.ConfigName,
									MountPath: citacloudv1.ConfigMountPath,
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
	if err := ctrl.SetControllerReference(restore, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *RestoreReconciler) buildArgsForFile(restore *citacloudv1.Restore) []string {
	if restore.Spec.DeleteConsensusData {
		return []string{
			"restore",
			"--namespace", restore.Namespace,
			"--chain", restore.Spec.Chain,
			"--node", restore.Spec.Node,
			"--deploy-method", string(restore.Spec.DeployMethod),
			"--action", string(restore.Spec.Action),
			"--delete-consensus-data",
		}
	} else {
		return []string{
			"restore",
			"--namespace", restore.Namespace,
			"--chain", restore.Spec.Chain,
			"--node", restore.Spec.Node,
			"--deploy-method", string(restore.Spec.DeployMethod),
			"--action", string(restore.Spec.Action),
		}
	}
}

func (r *RestoreReconciler) buildArgsForSnapshot(restore *citacloudv1.Restore, crypto, consensus string) []string {
	if crypto != "" {
		return []string{
			"snapshot-recover",
			"--namespace", restore.Namespace,
			"--chain", restore.Spec.Chain,
			"--node", restore.Spec.Node,
			"--deploy-method", string(restore.Spec.DeployMethod),
			"--block-height", strconv.FormatInt(r.snapshot.Spec.BlockHeight, 10),
			"--crypto", crypto,
			"--consensus", consensus,
		}
	} else {
		return []string{
			"snapshot-recover",
			"--namespace", restore.Namespace,
			"--chain", restore.Spec.Chain,
			"--node", restore.Spec.Node,
			"--deploy-method", string(restore.Spec.DeployMethod),
			"--block-height", strconv.FormatInt(r.snapshot.Spec.BlockHeight, 10),
			"--consensus", consensus,
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.Restore{}).
		Owns(&v1.Job{}, builder.WithPredicates(jobPredicate())).
		Complete(r)
}
