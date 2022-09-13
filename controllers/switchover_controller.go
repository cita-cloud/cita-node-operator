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
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

// SwitchoverReconciler reconciles a Switchover object
type SwitchoverReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=switchovers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=switchovers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=switchovers/finalizers,verbs=update

func (r *SwitchoverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("backup crd %s in reconcile", req.NamespacedName))

	switchover := &citacloudv1.Switchover{}
	if err := r.Get(ctx, req.NamespacedName, switchover); err != nil {
		logger.Info(fmt.Sprintf("the switchover crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set default
	if r.setDefaultSpec(switchover) {
		err := r.Update(ctx, switchover)
		if err != nil {
			logger.Error(err, "set default spce failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if r.setDefaultStatus(switchover) {
		err := r.Status().Update(ctx, switchover)
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

	// find source node configmap
	sourceCm := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf("%s-config", switchover.Spec.SourceNode),
		Namespace: switchover.Namespace,
	}, sourceCm)
	if err != nil {
		logger.Error(err, "get source node configmap failed")
		return ctrl.Result{}, err
	}
	// find dest node configmap
	destCm := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf("%s-config", switchover.Spec.DestNode),
		Namespace: switchover.Namespace,
	}, destCm)
	if err != nil {
		logger.Error(err, "get dest node configmap failed")
		return ctrl.Result{}, err
	}

	// Check if the job already exists, if not create a new one
	foundJob := &v1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: switchover.Name, Namespace: switchover.Namespace}, foundJob)

	if switchover.Status.Status == citacloudv1.JobComplete || switchover.Status.Status == citacloudv1.JobFailed {
		logger.Info(fmt.Sprintf("switchover status is finished: [%s]", switchover.Status.Status))
		// will delete job if job exist
		if err == nil && foundJob != nil {
			go CleanJob(ctx, r.Client, foundJob, switchover.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	if err != nil && errors.IsNotFound(err) {
		// Define a new job
		job, err := r.jobForSwitchover(ctx, switchover)
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
		logger.Error(err, fmt.Sprintf("failed to get job %s/%s", switchover.Namespace, switchover.Name))
		return ctrl.Result{}, err
	}

	cur := switchover.DeepCopy()
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
	if !IsEqual(cur, switchover) {
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

func (r *SwitchoverReconciler) setDefaultSpec(switchover *citacloudv1.Switchover) bool {
	updateFlag := false
	if switchover.Spec.Image == "" {
		switchover.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if switchover.Spec.PullPolicy == "" {
		switchover.Spec.PullPolicy = corev1.PullIfNotPresent
		updateFlag = true
	}
	if switchover.Spec.TTLSecondsAfterFinished == 0 {
		switchover.Spec.TTLSecondsAfterFinished = 30
		updateFlag = true
	}
	return updateFlag
}

func (r *SwitchoverReconciler) setDefaultStatus(switchover *citacloudv1.Switchover) bool {
	updateFlag := false
	if switchover.Status.Status == "" {
		switchover.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	if switchover.Status.StartTime == nil {
		startTime := switchover.CreationTimestamp
		switchover.Status.StartTime = &startTime
		updateFlag = true
	}
	return updateFlag
}

func (r *SwitchoverReconciler) jobForSwitchover(ctx context.Context, switchover *citacloudv1.Switchover) (*v1.Job, error) {
	labels := LabelsForChain(switchover.Spec.Chain)

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      switchover.Name,
			Namespace: switchover.Namespace,
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
							Image:           switchover.Spec.Image,
							ImagePullPolicy: switchover.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: []string{
								"switchover",
								"--namespace", switchover.Namespace,
								"--chain", switchover.Spec.Chain,
								"--source-node", switchover.Spec.SourceNode,
								"--dest-node", switchover.Spec.DestNode,
							},
						},
					},
				},
			},
		},
	}
	// bind
	if err := ctrl.SetControllerReference(switchover, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwitchoverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.Switchover{}).
		Owns(&v1.Job{}, builder.WithPredicates(jobPredicate())).
		Complete(r)
}
