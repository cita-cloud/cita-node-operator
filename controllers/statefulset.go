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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *CitaNodeReconciler) ReconcileStatefulSet(ctx context.Context, node *citacloudv1.CitaNode) (*NodeValue, error) {
	logger := log.FromContext(ctx)
	old := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: node.Name, Namespace: node.Namespace}, old)
	if errors.IsNotFound(err) {
		newObj := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      node.Name,
				Namespace: node.Namespace,
			},
		}
		if err = r.generateStatefulSet(ctx, node, newObj); err != nil {
			return nil, err
		}
		logger.Info("create node statefulset....")
		return nil, r.Create(ctx, newObj)
	} else if err != nil {
		return nil, err
	}

	cur := old.DeepCopy()
	if err := r.generateStatefulSet(ctx, node, cur); err != nil {
		return nil, err
	}
	t1Copy := old.Spec.Template.Spec.Containers
	t2Copy := cur.Spec.Template.Spec.Containers

	nv := &NodeValue{}
	if IsEqual(t1Copy, t2Copy) {
		if *old.Spec.Replicas == *cur.Spec.Replicas {
			logger.Info("the statefulset part has not changed, go pass")
			return nil, nil
		} else if *old.Spec.Replicas > *cur.Spec.Replicas {
			nv.Status = citacloudv1.Stopping
		} else {
			nv.Status = citacloudv1.Starting
		}
	} else {
		nv.Status = citacloudv1.Upgrading
	}

	// currently only update the changes under Containers
	old.Spec.Template.Spec.Containers = cur.Spec.Template.Spec.Containers
	old.Spec.Replicas = cur.Spec.Replicas
	logger.Info("update node statefulset...")
	return nv, r.Update(ctx, old)
}

func getNetworkCmdStr(enableTLS bool) []string {
	if enableTLS {
		return []string{
			"network",
			"run",
			"-c",
			fmt.Sprintf("%s/%s", NodeConfigVolumeMountPath, NodeConfigFile),
			"--stdout"}
	} else {
		return []string{
			"network",
			"run",
			"-c",
			fmt.Sprintf("%s/%s", NodeConfigVolumeMountPath, NodeConfigFile),
			"-l",
			fmt.Sprintf("%s/%s", LogConfigVolumeMountPath, NetworkLogConfigFile)}
	}
}

func getConsensusCmdStr(consensusType citacloudv1.ConsensusType) []string {
	if consensusType == citacloudv1.BFT {
		return []string{
			"consensus",
			"run",
			"-c",
			fmt.Sprintf("%s/%s", NodeConfigVolumeMountPath, NodeConfigFile),
			"-l",
			fmt.Sprintf("%s/%s", LogConfigVolumeMountPath, ConsensusLogConfigFile)}
	} else if consensusType == citacloudv1.Raft {
		return []string{
			"consensus",
			"run",
			"-c",
			fmt.Sprintf("%s/%s", NodeConfigVolumeMountPath, NodeConfigFile),
			"--stdout"}
	} else {
		return []string{}
	}
}

func (r *CitaNodeReconciler) generateStatefulSet(ctx context.Context, node *citacloudv1.CitaNode, set *appsv1.StatefulSet) error {
	replica := int32(1)
	if node.Spec.DesiredState == citacloudv1.Stopped {
		replica = 0
	}
	labels := MergeLabels(set.Labels, LabelsForNode(node.Spec.ChainName, node.Name))
	logger := log.FromContext(ctx)
	set.Labels = labels
	if err := ctrl.SetControllerReference(node, set, r.Scheme); err != nil {
		logger.Error(err, "node statefulset SetControllerReference error")
		return err
	}

	set.Spec = appsv1.StatefulSetSpec{
		Replicas: pointer.Int32(replica),
		UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		},
		PodManagementPolicy: appsv1.ParallelPodManagement,
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		VolumeClaimTemplates: GeneratePVC(node),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:            "init",
						Image:           "busybox:stable",
						ImagePullPolicy: node.Spec.PullPolicy,
						Command:         []string{"/bin/sh"},
						Args:            []string{"-c", fmt.Sprintf("if [ ! -f \"%s/kms.db\" ]; then cp %s/kms.db %s;fi;", DataVolumeMountPath, KmsDBMountPath, DataVolumeMountPath)},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("10Mi"),
							},
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      NodeConfigVolumeName,
								MountPath: KmsDBMountPath,
							},
							{
								Name:      DataVolumeName,
								MountPath: DataVolumeMountPath,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:            NetworkContainer,
						Image:           node.Spec.NetworkImage,
						ImagePullPolicy: node.Spec.PullPolicy,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: NetworkPort,
								Protocol:      corev1.ProtocolTCP,
								Name:          "network",
							},
							{
								ContainerPort: 50000,
								Protocol:      corev1.ProtocolTCP,
								Name:          "grpc",
							},
						},
						Command:    getNetworkCmdStr(node.Spec.EnableTLS),
						WorkingDir: DataVolumeMountPath,
						VolumeMounts: []corev1.VolumeMount{
							// data volume
							{
								Name:      DataVolumeName,
								MountPath: DataVolumeMountPath,
							},
							// node config
							{
								Name:      NodeConfigVolumeName,
								MountPath: NodeConfigVolumeMountPath,
							},
							// log config
							{
								Name:      LogConfigVolumeName,
								MountPath: LogConfigVolumeMountPath,
							},
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
					{
						Name:            ConsensusContainer,
						Image:           node.Spec.ConsensusImage,
						ImagePullPolicy: node.Spec.PullPolicy,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 50001,
								Protocol:      corev1.ProtocolTCP,
								Name:          "grpc",
							},
						},
						Command:    getConsensusCmdStr(node.Spec.ConsensusType),
						WorkingDir: DataVolumeMountPath,
						VolumeMounts: []corev1.VolumeMount{
							// data volume
							{
								Name:      DataVolumeName,
								MountPath: DataVolumeMountPath,
							},
							// node config
							{
								Name:      NodeConfigVolumeName,
								MountPath: NodeConfigVolumeMountPath,
							},
							// log config
							{
								Name:      LogConfigVolumeName,
								MountPath: LogConfigVolumeMountPath,
							},
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
					{
						Name:            ExecutorContainer,
						Image:           node.Spec.ExecutorImage,
						ImagePullPolicy: node.Spec.PullPolicy,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: ExecutorPort,
								Protocol:      corev1.ProtocolTCP,
								Name:          "grpc",
							},
						},
						Command: []string{
							"executor",
							"run",
							"-c",
							fmt.Sprintf("%s/%s", NodeConfigVolumeMountPath, NodeConfigFile),
							"-l",
							fmt.Sprintf("%s/%s", LogConfigVolumeMountPath, ExecutorLogConfigFile),
						},
						WorkingDir: DataVolumeMountPath,
						VolumeMounts: []corev1.VolumeMount{
							// data volume
							{
								Name:      DataVolumeName,
								MountPath: DataVolumeMountPath,
							},
							// node config
							{
								Name:      NodeConfigVolumeName,
								MountPath: NodeConfigVolumeMountPath,
							},
							// log config
							{
								Name:      LogConfigVolumeName,
								MountPath: LogConfigVolumeMountPath,
							},
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
					{
						Name:            StorageContainer,
						Image:           node.Spec.StorageImage,
						ImagePullPolicy: node.Spec.PullPolicy,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 50003,
								Protocol:      corev1.ProtocolTCP,
								Name:          "grpc",
							},
						},
						Command: []string{
							"storage",
							"run",
							"-c",
							fmt.Sprintf("%s/%s", NodeConfigVolumeMountPath, NodeConfigFile),
							"-l",
							fmt.Sprintf("%s/%s", LogConfigVolumeMountPath, StorageLogConfigFile),
						},
						WorkingDir: DataVolumeMountPath,
						VolumeMounts: []corev1.VolumeMount{
							// data volume
							{
								Name:      DataVolumeName,
								MountPath: DataVolumeMountPath,
							},
							// node config
							{
								Name:      NodeConfigVolumeName,
								MountPath: NodeConfigVolumeMountPath,
							},
							// log config
							{
								Name:      LogConfigVolumeName,
								MountPath: LogConfigVolumeMountPath,
							},
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
					{
						Name:            ControllerContainer,
						Image:           node.Spec.ControllerImage,
						ImagePullPolicy: node.Spec.PullPolicy,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 50004,
								Protocol:      corev1.ProtocolTCP,
								Name:          "grpc",
							},
						},
						Command: []string{
							"controller",
							"run",
							"-c",
							fmt.Sprintf("%s/%s", NodeConfigVolumeMountPath, NodeConfigFile),
							"-l",
							fmt.Sprintf("%s/%s", LogConfigVolumeMountPath, ControllerLogConfigFile),
						},
						WorkingDir: DataVolumeMountPath,
						VolumeMounts: []corev1.VolumeMount{
							// data volume
							{
								Name:      DataVolumeName,
								MountPath: DataVolumeMountPath,
							},
							// node config
							{
								Name:      NodeConfigVolumeName,
								MountPath: NodeConfigVolumeMountPath,
							},
							// log config
							{
								Name:      LogConfigVolumeName,
								MountPath: LogConfigVolumeMountPath,
							},
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
					{
						Name:            KmsContainer,
						Image:           node.Spec.KmsImage,
						ImagePullPolicy: node.Spec.PullPolicy,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 50005,
								Protocol:      corev1.ProtocolTCP,
								Name:          "grpc",
							},
						},
						Command: []string{
							"kms",
							"run",
							"-c",
							fmt.Sprintf("%s/%s", NodeConfigVolumeMountPath, NodeConfigFile),
							"-l",
							fmt.Sprintf("%s/%s", LogConfigVolumeMountPath, KmsLogConfigFile),
						},
						WorkingDir: DataVolumeMountPath,
						VolumeMounts: []corev1.VolumeMount{
							// data volume
							{
								Name:      DataVolumeName,
								MountPath: DataVolumeMountPath,
							},
							// node config
							{
								Name:      NodeConfigVolumeName,
								MountPath: NodeConfigVolumeMountPath,
							},
							// log config
							{
								Name:      LogConfigVolumeName,
								MountPath: LogConfigVolumeMountPath,
							},
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
				},
				Volumes: GetVolumes(node),
			},
		},
	}

	//  set pvc reference to crd
	for i := range set.Spec.VolumeClaimTemplates {
		if err := ctrl.SetControllerReference(node, &set.Spec.VolumeClaimTemplates[i], r.Scheme); err != nil {
			logger.Error(err, "node statefulset pvc SetControllerReference error")
			return err
		}
	}

	return nil
}

func GeneratePVC(node *citacloudv1.CitaNode) []corev1.PersistentVolumeClaim {
	return []corev1.PersistentVolumeClaim{
		// data pvc
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DataVolumeName,
				Namespace: node.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *resource.NewQuantity(*node.Spec.StorageSize, resource.BinarySI),
					},
				},
				StorageClassName: node.Spec.StorageClassName,
			},
		},
	}
}

func GetVolumes(node *citacloudv1.CitaNode) []corev1.Volume {
	return []corev1.Volume{
		// node config configmap as a volume
		{
			Name: NodeConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: node.Spec.ConfigMapRef,
					},
				},
			},
		},
		// log config configmap as a volume
		{
			Name: LogConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: GetLogConfigName(node.Name),
					},
				},
			},
		},
	}
}
