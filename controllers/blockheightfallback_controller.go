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
	chainpkg "github.com/cita-cloud/cita-node-operator/pkg/chain"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	foundServiceAccount := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: citacloudv1.FallbackJobServiceAccount, Namespace: bhf.Namespace}, foundServiceAccount)
	if err != nil && errors.IsNotFound(err) {
		sa := r.serviceAccountForBlockHeightFallback(bhf)
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
	err = r.Get(ctx, types.NamespacedName{Name: citacloudv1.FallbackJobClusterRole}, foundClusterRole)
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
	err = r.Get(ctx, types.NamespacedName{Name: citacloudv1.FallbackJobClusterRoleBinding}, foundClusterRoleBinding)
	if err != nil && errors.IsNotFound(err) {
		clusterRoleBinding := r.clusterRoleBindingForBlockHeightFallback(bhf)
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
			if subject.Name == citacloudv1.FallbackJobServiceAccount && subject.Namespace == bhf.Namespace {
				existServiceAccount = true
			}
		}
		if !existServiceAccount {
			foundClusterRoleBinding.Subjects = append(foundClusterRoleBinding.Subjects, rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      citacloudv1.FallbackJobServiceAccount,
				Namespace: bhf.Namespace,
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
	err = r.Get(ctx, types.NamespacedName{Name: bhf.Name, Namespace: bhf.Namespace}, foundJob)
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

func (r *BlockHeightFallbackReconciler) serviceAccountForBlockHeightFallback(bhf *citacloudv1.BlockHeightFallback) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      citacloudv1.FallbackJobServiceAccount,
			Namespace: bhf.Namespace,
		},
	}
	return sa
}

func (r *BlockHeightFallbackReconciler) clusterRoleForBlockHeightFallback() *rbacv1.ClusterRole {
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
			Name: citacloudv1.FallbackJobClusterRole,
		},
		Rules: []rbacv1.PolicyRule{
			stsPR,
			depPR,
		},
	}
	return clusterRole
}

func (r *BlockHeightFallbackReconciler) clusterRoleBindingForBlockHeightFallback(bhf *citacloudv1.BlockHeightFallback) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: citacloudv1.FallbackJobClusterRoleBinding,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      citacloudv1.FallbackJobServiceAccount,
				Namespace: bhf.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     citacloudv1.FallbackJobClusterRole,
		},
	}
	return clusterRoleBinding
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
					ServiceAccountName: citacloudv1.FallbackJobServiceAccount,
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
	if bhf.Spec.ChainDeployMethod == chainpkg.Helm {
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
	} else if bhf.Spec.ChainDeployMethod == chainpkg.PythonOperator {
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
	} else if bhf.Spec.ChainDeployMethod == chainpkg.CloudConfig {
		pvcMap, cmMap, err := r.getCloudConfigVolumes(ctx, bhf)
		if err != nil {
			return nil, err
		}
		vols := make([]corev1.Volume, 0)
		//  add pvc
		for node, pvcName := range pvcMap {
			vols = append(vols, corev1.Volume{
				Name: fmt.Sprintf("%s-data", node),
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
						ReadOnly:  false,
					},
				}})
		}
		// add configmap
		for node, cmName := range cmMap {
			vols = append(vols, corev1.Volume{
				Name: fmt.Sprintf("%s-config", node),
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cmName,
						},
					},
				}})
		}
		return vols, nil
	} else {
		return nil, fmt.Errorf("mismatched deploy method")
	}
}

func (r *BlockHeightFallbackReconciler) getVolumeMounts(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) ([]corev1.VolumeMount, error) {
	if bhf.Spec.ChainDeployMethod == chainpkg.Helm || bhf.Spec.ChainDeployMethod == chainpkg.PythonOperator {
		return []corev1.VolumeMount{
			{
				Name:      citacloudv1.VolumeName,
				MountPath: citacloudv1.VolumeMountPath,
			},
		}, nil
	} else if bhf.Spec.ChainDeployMethod == chainpkg.CloudConfig {
		pvcMap, cmMap, err := r.getCloudConfigVolumes(ctx, bhf)
		if err != nil {
			return nil, err
		}
		volumeMounts := make([]corev1.VolumeMount, 0)
		for node := range pvcMap {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      fmt.Sprintf("%s-data", node),
				MountPath: fmt.Sprintf("/%s-data", node),
			})
		}
		for node := range cmMap {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      fmt.Sprintf("%s-config", node),
				MountPath: fmt.Sprintf("/%s-config", node),
			})
		}
		return volumeMounts, err
	}
	return nil, nil
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

func (r *BlockHeightFallbackReconciler) getHelmPVC(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (string, error) {
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

func (r *BlockHeightFallbackReconciler) getPyPVC(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (string, error) {
	// find chain's statefuleset
	deployList := &appsv1.DeploymentList{}
	deployOpts := []client.ListOption{
		client.InNamespace(bhf.Spec.Namespace),
		client.MatchingLabels(map[string]string{"chain_name": bhf.Spec.ChainName}),
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

func (r *BlockHeightFallbackReconciler) getCloudConfigVolumes(ctx context.Context, bhf *citacloudv1.BlockHeightFallback) (map[string]string, map[string]string, error) {
	pvcMap := make(map[string]string, 0)
	cmMap := make(map[string]string, 0)
	if bhf.AllNodes() {
		stsList := &appsv1.StatefulSetList{}
		stsOpts := []client.ListOption{
			client.InNamespace(bhf.Spec.Namespace),
			client.MatchingLabels(map[string]string{"app.kubernetes.io/chain-name": bhf.Spec.ChainName}),
		}
		if err := r.List(ctx, stsList, stsOpts...); err != nil {
			return nil, nil, err
		}
		for _, sts := range stsList.Items {
			pvcMap[sts.Name] = fmt.Sprintf("datadir-%s-0", sts.Name)
			cmMap[sts.Name] = fmt.Sprintf("%s-config", sts.Name)
		}
		return pvcMap, cmMap, nil
	} else {
		for _, node := range chainpkg.GetNodeList(bhf.Spec.NodeList) {
			pvcMap[node] = fmt.Sprintf("datadir-%s-0", node)
			cmMap[node] = fmt.Sprintf("%s-config", node)
		}
		return pvcMap, cmMap, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BlockHeightFallbackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.BlockHeightFallback{}).
		Owns(&v1.Job{}).
		Complete(r)
}
