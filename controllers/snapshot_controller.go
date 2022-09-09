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
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

// SnapshotReconciler reconciles a Snapshot object
type SnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=snapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=snapshots/finalizers,verbs=update

func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("snapshot crd %s in reconcile", req.NamespacedName))

	snapshot := &citacloudv1.Snapshot{}
	if err := r.Get(ctx, req.NamespacedName, snapshot); err != nil {
		logger.Info(fmt.Sprintf("the snapshot crd %s has been deleted", req.NamespacedName))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// set default
	if r.setDefaultSpec(snapshot) {
		err := r.Update(ctx, snapshot)
		if err != nil {
			logger.Error(err, "set default spce failed")
			return ctrl.Result{}, err
		}
		//  requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if r.setDefaultStatus(snapshot) {
		err := r.Status().Update(ctx, snapshot)
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
	if err := jobRbac.Ensure(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Check if the job already exists, if not create a new one
	foundJob := &v1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, foundJob)

	if snapshot.Status.Status == citacloudv1.JobComplete || snapshot.Status.Status == citacloudv1.JobFailed {
		logger.Info(fmt.Sprintf("snapshot status is finished: [%s]", snapshot.Status.Status))
		// will delete job if job exist
		if err == nil && foundJob != nil {
			go CleanJob(ctx, r.Client, foundJob, snapshot.Spec.TTLSecondsAfterFinished)
		}
		return ctrl.Result{}, nil
	}

	if err != nil && errors.IsNotFound(err) {
		// Define a new job
		job, err := r.jobForSnapshot(ctx, snapshot)
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
		logger.Error(err, fmt.Sprintf("failed to get job %s/%s", snapshot.Namespace, snapshot.Name))
		return ctrl.Result{}, err
	}
	cur := snapshot.DeepCopy()
	if job.Status.Active == 1 {
		cur.Status.Status = citacloudv1.JobActive
	} else if job.Status.Failed == 1 {
		cur.Status.Status = citacloudv1.JobFailed
		endTime := job.Status.Conditions[0].LastTransitionTime
		cur.Status.EndTime = &endTime
	} else if job.Status.Succeeded == 1 {
		cur.Status.Status = citacloudv1.JobComplete
		// get snapshot size from pod annotations
		podList := &corev1.PodList{}
		podOpts := []client.ListOption{
			client.InNamespace(snapshot.Namespace),
			client.MatchingLabels(map[string]string{"job-name": snapshot.Name, "controller-uid": string(job.UID)}),
		}
		if err := r.List(ctx, podList, podOpts...); err != nil {
			return ctrl.Result{}, err
		}
		if len(podList.Items) != 1 {
			logger.Error(err, fmt.Sprintf("job's pod != 1"))
			return ctrl.Result{}, err
		}
		snapshotSizeStr := podList.Items[0].Annotations["snapshot-size"]
		snapshotSize, err := strconv.ParseInt(snapshotSizeStr, 10, 64)
		if err != nil {
			logger.Error(err, "parse snapshot size from string to int64 failed")
			return ctrl.Result{}, err
		}
		cur.Status.Actual = snapshotSize
		cur.Status.EndTime = job.Status.CompletionTime
	}
	if !IsEqual(cur, snapshot) {
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

func (r *SnapshotReconciler) setDefaultSpec(snapshot *citacloudv1.Snapshot) bool {
	updateFlag := false
	if snapshot.Spec.Image == "" {
		snapshot.Spec.Image = citacloudv1.DefaultImage
		updateFlag = true
	}
	if snapshot.Spec.PullPolicy == "" {
		snapshot.Spec.PullPolicy = corev1.PullIfNotPresent
		updateFlag = true
	}
	if snapshot.Spec.TTLSecondsAfterFinished == 0 {
		snapshot.Spec.TTLSecondsAfterFinished = 30
		updateFlag = true
	}
	return updateFlag
}

func (r *SnapshotReconciler) setDefaultStatus(snapshot *citacloudv1.Snapshot) bool {
	updateFlag := false
	if snapshot.Status.Status == "" {
		snapshot.Status.Status = citacloudv1.JobActive
		updateFlag = true
	}
	if snapshot.Status.StartTime == nil {
		startTime := snapshot.CreationTimestamp
		snapshot.Status.StartTime = &startTime
		updateFlag = true
	}
	return updateFlag
}

// SetupWithManager sets up the controller with the Manager.
func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.Snapshot{}).
		Complete(r)
}

func (r *SnapshotReconciler) jobForSnapshot(ctx context.Context, snapshot *citacloudv1.Snapshot) (*v1.Job, error) {
	labels := LabelsForNode(snapshot.Spec.Chain, snapshot.Spec.Node)

	var resourceRequirements corev1.ResourceRequirements
	var pvcSourceName string
	var crypto, consensus string

	if snapshot.Spec.DeployMethod == chainpkg.PythonOperator {
		deploy := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Name: snapshot.Spec.Node, Namespace: snapshot.Namespace}, deploy)
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
		if pvcSourceName == "" {
			return nil, fmt.Errorf("cann't get deployment's pvc")
		}
		pvc := &corev1.PersistentVolumeClaim{}
		err = r.Get(ctx, types.NamespacedName{Name: pvcSourceName, Namespace: snapshot.Namespace}, pvc)
		if err != nil {
			return nil, err
		}
		resourceRequirements = pvc.Spec.Resources
		crypto, consensus = filterCryptoAndConsensus(deploy.Spec.Template.Spec.Containers)
	} else {
		sts := &appsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{Name: snapshot.Spec.Node, Namespace: snapshot.Namespace}, sts)
		if err != nil {
			return nil, err
		}

		pvcs := sts.Spec.VolumeClaimTemplates
		for _, pvc := range pvcs {
			if pvc.Name == "datadir" {
				resourceRequirements = pvc.Spec.Resources
				break
			}
		}
		pvcSourceName = fmt.Sprintf("datadir-%s-0", snapshot.Spec.Node)
		crypto, consensus = filterCryptoAndConsensus(sts.Spec.Template.Spec.Containers)
	}
	destPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:        resourceRequirements,
			StorageClassName: pointer.String(snapshot.Spec.StorageClass),
		},
	}
	// bind
	if err := ctrl.SetControllerReference(snapshot, destPVC, r.Scheme); err != nil {
		return nil, err
	}
	err := r.Create(ctx, destPVC)
	if err != nil {
		return nil, err
	}

	volumes := []corev1.Volume{
		// source data dir
		{
			Name: citacloudv1.BackupSourceVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcSourceName,
					ReadOnly:  false,
				},
			},
		},
		// dest data dir
		{
			Name: citacloudv1.BackupDestVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: snapshot.Name,
					ReadOnly:  false,
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      citacloudv1.BackupSourceVolumeName,
			MountPath: citacloudv1.BackupSourceVolumePath,
		},
		{
			Name:      citacloudv1.BackupDestVolumeName,
			MountPath: citacloudv1.BackupDestVolumePath,
		},
	}

	if snapshot.Spec.DeployMethod == chainpkg.CloudConfig {
		volumes = append(volumes, corev1.Volume{
			// configmap
			Name: citacloudv1.ConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-config", snapshot.Spec.Node),
					},
				},
			}},
		)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      citacloudv1.ConfigName,
			MountPath: citacloudv1.ConfigMountPath,
		})
	}

	job := &v1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
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
							Image:           snapshot.Spec.Image,
							ImagePullPolicy: snapshot.Spec.PullPolicy,
							Command: []string{
								"/cita-node-cli",
							},
							Args: []string{
								"snapshot",
								"--namespace", snapshot.Namespace,
								"--chain", snapshot.Spec.Chain,
								"--node", snapshot.Spec.Node,
								"--deploy-method", string(snapshot.Spec.DeployMethod),
								"--block-height", strconv.FormatInt(snapshot.Spec.BlockHeight, 10),
								"--crypto", crypto,
								"--consensus", consensus,
							},
							VolumeMounts: volumeMounts,
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
	if err := ctrl.SetControllerReference(snapshot, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}
