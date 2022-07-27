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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=backups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=backups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=backups/finalizers,verbs=update

func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("backup crd %s in reconcile", req.NamespacedName))

	backup := &citacloudv1.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		logger.Info(fmt.Sprintf("the backup crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// todo: finalize

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

	// Check if the serviceAccount already exists, if not create a new one
	foundServiceAccount := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: citacloudv1.CITANodeJobServiceAccount, Namespace: backup.Namespace}, foundServiceAccount)
	if err != nil && errors.IsNotFound(err) {
		sa := r.serviceAccountForBlockHeightFallback(backup)
		logger.Info("creating a new service account")
		err = r.Create(ctx, sa)
		if err != nil {
			logger.Error(err, "failed to create new service account")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		logger.Error(err, "failed to get service account")
		return ctrl.Result{}, err
	} else {
		// todo reconcile
	}

	// Check if the cluster role already exists, if not create a new one
	foundClusterRole := &rbacv1.ClusterRole{}
	err = r.Get(ctx, types.NamespacedName{Name: citacloudv1.CITANodeJobClusterRole}, foundClusterRole)
	if err != nil && errors.IsNotFound(err) {
		clusterRole := r.clusterRoleForBlockHeightFallback()
		logger.Info("creating a new cluster role")
		err = r.Create(ctx, clusterRole)
		if err != nil {
			logger.Error(err, "failed to create new cluster role")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		logger.Error(err, "failed to get cluster role")
		return ctrl.Result{}, err
	} else {
		// todo reconcile
	}

	// Check if the cluster role binding already exists, if not create a new one
	foundClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = r.Get(ctx, types.NamespacedName{Name: citacloudv1.CITANodeJobClusterRoleBinding}, foundClusterRoleBinding)
	if err != nil && errors.IsNotFound(err) {
		clusterRoleBinding := r.clusterRoleBindingForBlockHeightFallback(backup)
		logger.Info("creating a new cluster role binding")
		err = r.Create(ctx, clusterRoleBinding)
		if err != nil {
			logger.Error(err, "failed to create new cluster role binding")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		logger.Error(err, "failed to get cluster role binding")
		return ctrl.Result{}, err
	} else {
		var existServiceAccount bool
		for _, subject := range foundClusterRoleBinding.Subjects {
			if subject.Name == citacloudv1.CITANodeJobServiceAccount && subject.Namespace == backup.Namespace {
				existServiceAccount = true
			}
		}
		if !existServiceAccount {
			foundClusterRoleBinding.Subjects = append(foundClusterRoleBinding.Subjects, rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      citacloudv1.CITANodeJobServiceAccount,
				Namespace: backup.Namespace,
			})
			err := r.Update(ctx, foundClusterRoleBinding)
			if err != nil {
				logger.Error(err, "failed to update cluster role binding")
				return ctrl.Result{}, err
			}
		}
	}

	// Check if the job already exists, if not create a new one
	foundJob := &v1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}, foundJob)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
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
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "failed to get job")
		return ctrl.Result{}, err
	}

	// todo
	return ctrl.Result{}, nil
}

func (r *BackupReconciler) setDefaultSpec(backup *citacloudv1.Backup) bool {
	updateFlag := false
	if backup.Spec.Image == "" {
		backup.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if backup.Spec.PullPolicy == "" {
		backup.Spec.PullPolicy = corev1.PullIfNotPresent
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
	return updateFlag
}

func (r *BackupReconciler) serviceAccountForBlockHeightFallback(backup *citacloudv1.Backup) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      citacloudv1.CITANodeJobServiceAccount,
			Namespace: backup.Namespace,
		},
	}
	return sa
}

func (r *BackupReconciler) clusterRoleForBlockHeightFallback() *rbacv1.ClusterRole {
	stsPR := rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch", "update"},
		APIGroups: []string{"apps"},
		Resources: []string{"statefulsets"},
	}
	depPR := rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch", "update"},
		APIGroups: []string{"apps"},
		Resources: []string{"deployments"},
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: citacloudv1.CITANodeJobClusterRole,
		},
		Rules: []rbacv1.PolicyRule{
			stsPR,
			depPR,
		},
	}
	return clusterRole
}

func (r *BackupReconciler) clusterRoleBindingForBlockHeightFallback(backup *citacloudv1.Backup) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: citacloudv1.CITANodeJobClusterRoleBinding,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      citacloudv1.CITANodeJobServiceAccount,
				Namespace: backup.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     citacloudv1.CITANodeJobClusterRole,
		},
	}
	return clusterRoleBinding
}

func (r *BackupReconciler) jobForBackup(ctx context.Context, backup *citacloudv1.Backup) (*v1.Job, error) {
	labels := labelsForBackup(backup)

	var resourceRequirements corev1.ResourceRequirements
	var pvcSourceName string

	if backup.Spec.DeployMethod == chainpkg.PythonOperator {
		deployList := &appsv1.DeploymentList{}
		deployOpts := []client.ListOption{
			client.InNamespace(backup.Spec.Namespace),
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
		err := r.Get(ctx, types.NamespacedName{Name: pvcSourceName, Namespace: backup.Spec.Namespace}, pvc)
		if err != nil {
			return nil, err
		}
		resourceRequirements = pvc.Spec.Resources
	} else {
		stsList := &appsv1.StatefulSetList{}
		stsOpts := []client.ListOption{
			client.InNamespace(backup.Spec.Namespace),
			client.MatchingLabels(map[string]string{"app.kubernetes.io/node-name": backup.Spec.Chain, "app.kubernetes.io/node-node": backup.Spec.Node}),
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
			Namespace: backup.Spec.Namespace,
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
								"--namespace", backup.Spec.Namespace,
								"--chain", backup.Spec.Chain,
								"--node", backup.Spec.Node,
								"--deploy-method", string(backup.Spec.DeployMethod),
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

func labelsForBackup(backup *citacloudv1.Backup) map[string]string {
	return map[string]string{"app.kubernetes.io/node-name": backup.Spec.Chain, "app.kubernetes.io/node-node": backup.Spec.Node}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.Backup{}).
		Owns(&v1.Job{}).
		Complete(r)
}
