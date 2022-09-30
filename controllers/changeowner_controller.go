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

// ChangeOwnerReconciler reconciles a ChangeOwner object
type ChangeOwnerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=changeowners,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=changeowners/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=changeowners/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ChangeOwner object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ChangeOwnerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("changeOwn crd %s in reconcile", req.NamespacedName))

	changeOwner := &citacloudv1.ChangeOwner{}
	if err := r.Get(ctx, req.NamespacedName, changeOwner); err != nil {
		logger.Info(fmt.Sprintf("the changeOwner crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set default
	if r.setDefaultSpec(changeOwner) {
		err := r.Update(ctx, changeOwner)
		if err != nil {
			logger.Error(err, "set default spce failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if r.setDefaultStatus(changeOwner) {
		err := r.Status().Update(ctx, changeOwner)
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
	err = r.Get(ctx, types.NamespacedName{Name: changeOwner.Name, Namespace: changeOwner.Namespace}, foundJob)

	if changeOwner.Status.Status == citacloudv1.JobComplete || changeOwner.Status.Status == citacloudv1.JobFailed {
		logger.Info(fmt.Sprintf("changeOwn status is finished: [%s]", changeOwner.Status.Status))
		// will delete job if job exist
		if err == nil && foundJob != nil {
			go CleanJob(ctx, r.Client, foundJob, changeOwner.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	if err != nil && errors.IsNotFound(err) {
		// Define a new job
		job, err := r.jobForChangeOwner(ctx, changeOwner)
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
		logger.Error(err, fmt.Sprintf("failed to get job %s/%s", changeOwner.Namespace, changeOwner.Name))
		return ctrl.Result{}, err
	}
	cur := changeOwner.DeepCopy()
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
	if !IsEqual(cur, changeOwner) {
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

func (r *ChangeOwnerReconciler) setDefaultSpec(owner *citacloudv1.ChangeOwner) bool {
	updateFlag := false
	if owner.Spec.Action == "" {
		owner.Spec.Action = chainpkg.StopAndStart
		updateFlag = true
	}
	if owner.Spec.Image == "" {
		owner.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if owner.Spec.PullPolicy == "" {
		owner.Spec.PullPolicy = corev1.PullIfNotPresent
		updateFlag = true
	}
	if owner.Spec.TTLSecondsAfterFinished == 0 {
		owner.Spec.TTLSecondsAfterFinished = 30
		updateFlag = true
	}
	if owner.Spec.Uid == 0 {
		owner.Spec.Uid = 1000
		updateFlag = true
	}
	if owner.Spec.Gid == 0 {
		owner.Spec.Gid = 1000
		updateFlag = true
	}
	return updateFlag
}

func (r *ChangeOwnerReconciler) setDefaultStatus(owner *citacloudv1.ChangeOwner) bool {
	updateFlag := false
	if owner.Status.Status == "" {
		owner.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	if owner.Status.StartTime == nil {
		startTime := owner.CreationTimestamp
		owner.Status.StartTime = &startTime
		updateFlag = true
	}
	return updateFlag
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChangeOwnerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.ChangeOwner{}).
		Owns(&v1.Job{}, builder.WithPredicates(jobPredicate())).
		Complete(r)
}

func (r *ChangeOwnerReconciler) jobForChangeOwner(ctx context.Context, owner *citacloudv1.ChangeOwner) (*v1.Job, error) {
	labels := LabelsForNode(owner.Spec.Chain, owner.Spec.Node)

	var pvcSourceName string

	if owner.Spec.DeployMethod == chainpkg.PythonOperator {
		deploy := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: owner.Namespace,
			Name:      owner.Spec.Node,
		}, deploy)
		if err != nil {
			return nil, err
		}
		volumes := deploy.Spec.Template.Spec.Volumes
		for _, volume := range volumes {
			if volume.Name == "datadir" {
				pvcSourceName = volume.PersistentVolumeClaim.ClaimName
				break
			}
		}
	} else {
		pvcSourceName = fmt.Sprintf("datadir-%s-0", owner.Spec.Node)
	}
	volumes := []corev1.Volume{
		{
			Name: citacloudv1.ChangeOwnerVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcSourceName,
					ReadOnly:  false,
				},
			},
		},
	}

	nodeKey, err := GetNodeLabelKeyByType(owner.Spec.DeployMethod)
	if err != nil {
		return nil, err
	}

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      owner.Name,
			Namespace: owner.Namespace,
			Labels:    labels,
		},
		Spec: v1.JobSpec{
			BackoffLimit: pointer.Int32(0),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity:           SetAffinity(owner.Spec.PodAffinityFlag, nodeKey, owner.Spec.Node),
					ServiceAccountName: citacloudv1.CITANodeJobServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "cita-node-cli",
							Image:           owner.Spec.Image,
							ImagePullPolicy: owner.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: []string{
								"chown",
								"--namespace", owner.Namespace,
								"--chain", owner.Spec.Chain,
								"--node", owner.Spec.Node,
								"--deploy-method", string(owner.Spec.DeployMethod),
								"--action", string(owner.Spec.Action),
								"--uid", strconv.FormatInt(owner.Spec.Uid, 10),
								"--gid", strconv.FormatInt(owner.Spec.Gid, 10),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      citacloudv1.ChangeOwnerVolumeName,
									MountPath: citacloudv1.ChangeOwnerVolumePath,
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
	if err := ctrl.SetControllerReference(owner, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}
