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
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=backups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=backups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=backups/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=create;get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups="apps",resources=statefulsets/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups="apps",resources=deployments/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=create;get;list;watch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=create;get;list;watch;update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=create;get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=create;get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;get;list;watch;update;patch;delete

func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("backup crd %s in reconcile", req.NamespacedName))

	backup := &citacloudv1.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		logger.Info(fmt.Sprintf("the backup crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set default
	if r.setDefaultSpec(backup) {
		err := r.Update(ctx, backup)
		if err != nil {
			logger.Error(err, "set default spce failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if r.setDefaultStatus(backup) {
		err := r.Status().Update(ctx, backup)
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
	err = r.Get(ctx, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}, foundJob)

	if backup.Status.Status == citacloudv1.JobComplete || backup.Status.Status == citacloudv1.JobFailed {
		logger.Info(fmt.Sprintf("backup status is finished: [%s]", backup.Status.Status))
		// will delete job if job exist
		if err == nil && foundJob != nil {
			go CleanJob(ctx, r.Client, foundJob, backup.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	if err != nil && errors.IsNotFound(err) {
		// Define a new job
		job, err := r.jobForBackup(ctx, backup)
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
		logger.Error(err, fmt.Sprintf("failed to get job %s/%s", backup.Namespace, backup.Name))
		return ctrl.Result{}, err
	}
	cur := backup.DeepCopy()
	if job.Status.Active == 1 {
		cur.Status.Status = citacloudv1.JobActive
	} else if job.Status.Failed == 1 {
		cur.Status.Status = citacloudv1.JobFailed
		endTime := job.Status.Conditions[0].LastTransitionTime
		cur.Status.EndTime = &endTime
	} else if job.Status.Succeeded == 1 {
		cur.Status.Status = citacloudv1.JobComplete
		// get backup size from pod annotations
		podList := &corev1.PodList{}
		podOpts := []client.ListOption{
			client.InNamespace(backup.Namespace),
			client.MatchingLabels(map[string]string{"job-name": backup.Name, "controller-uid": string(job.UID)}),
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
	if !IsEqual(cur, backup) {
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

func (r *BackupReconciler) setDefaultSpec(backup *citacloudv1.Backup) bool {
	updateFlag := false
	if backup.Spec.Action == "" {
		backup.Spec.Action = chainpkg.StopAndStart
		updateFlag = true
	}
	if backup.Spec.Image == "" {
		backup.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if backup.Spec.PullPolicy == "" {
		backup.Spec.PullPolicy = corev1.PullIfNotPresent
		updateFlag = true
	}
	if backup.Spec.TTLSecondsAfterFinished == 0 {
		backup.Spec.TTLSecondsAfterFinished = 30
		updateFlag = true
	}
	return updateFlag
}

func (r *BackupReconciler) setDefaultStatus(backup *citacloudv1.Backup) bool {
	updateFlag := false
	if backup.Status.Status == "" {
		backup.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	if backup.Status.StartTime == nil {
		startTime := backup.CreationTimestamp
		backup.Status.StartTime = &startTime
		updateFlag = true
	}
	return updateFlag
}

func (r *BackupReconciler) jobForBackup(ctx context.Context, backup *citacloudv1.Backup) (*v1.Job, error) {
	labels := LabelsForNode(backup.Spec.Chain, backup.Spec.Node)

	var resourceRequirements corev1.ResourceRequirements
	var pvcSourceName string

	if backup.Spec.DeployMethod == chainpkg.PythonOperator {
		deployList := &appsv1.DeploymentList{}
		deployOpts := []client.ListOption{
			client.InNamespace(backup.Namespace),
			client.MatchingLabels(map[string]string{"chain_name": backup.Spec.Chain, "node_name": backup.Spec.Node}),
		}
		if err := r.List(ctx, deployList, deployOpts...); err != nil {
			return nil, err
		}
		if len(deployList.Items) != 1 {
			return nil, fmt.Errorf("node deployment != 1")
		}
		volumes := deployList.Items[0].Spec.Template.Spec.Volumes
		for _, volume := range volumes {
			if volume.Name == "datadir" {
				pvcSourceName = volume.PersistentVolumeClaim.ClaimName
				break
			}
		}
		if pvcSourceName == "" {
			return nil, fmt.Errorf("cann't get deployment's pvc")
		}
		pvc := &corev1.PersistentVolumeClaim{}
		err := r.Get(ctx, types.NamespacedName{Name: pvcSourceName, Namespace: backup.Namespace}, pvc)
		if err != nil {
			return nil, err
		}
		resourceRequirements = pvc.Spec.Resources
	} else {
		stsList := &appsv1.StatefulSetList{}
		stsOpts := []client.ListOption{
			client.InNamespace(backup.Namespace),
			client.MatchingLabels(map[string]string{"app.kubernetes.io/chain-name": backup.Spec.Chain, "app.kubernetes.io/chain-node": backup.Spec.Node}),
		}
		if err := r.List(ctx, stsList, stsOpts...); err != nil {
			return nil, err
		}
		if len(stsList.Items) != 1 {
			return nil, fmt.Errorf("node statefulset != 1")
		}
		pvcs := stsList.Items[0].Spec.VolumeClaimTemplates
		for _, pvc := range pvcs {
			if pvc.Name == "datadir" {
				resourceRequirements = pvc.Spec.Resources
				break
			}
		}
		pvcSourceName = fmt.Sprintf("datadir-%s-0", backup.Spec.Node)
	}
	destPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:        resourceRequirements,
			StorageClassName: pointer.String(backup.Spec.StorageClass),
		},
	}
	// bind
	if err := ctrl.SetControllerReference(backup, destPVC, r.Scheme); err != nil {
		return nil, err
	}
	err := r.Create(ctx, destPVC)
	if err != nil {
		return nil, err
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
					ClaimName: backup.Name,
					ReadOnly:  false,
				},
			},
		},
	}

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
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
							Image:           backup.Spec.Image,
							ImagePullPolicy: backup.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: []string{
								"backup",
								"--namespace", backup.Namespace,
								"--chain", backup.Spec.Chain,
								"--node", backup.Spec.Node,
								"--deploy-method", string(backup.Spec.DeployMethod),
								"--action", string(backup.Spec.Action),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      citacloudv1.BackupSourceVolumeName,
									MountPath: citacloudv1.BackupSourceVolumePath,
								},
								{
									Name:      citacloudv1.BackupDestVolumeName,
									MountPath: citacloudv1.BackupDestVolumePath,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "MY_POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "MY_POD_NAMESPACE",
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
	if err := ctrl.SetControllerReference(backup, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.Backup{}).
		Owns(&v1.Job{}, builder.WithPredicates(jobPredicate())).
		Complete(r)
}
