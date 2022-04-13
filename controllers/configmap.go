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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *CitaNodeReconciler) ReconcileConfigMap(ctx context.Context, node *citacloudv1.CitaNode) (bool, error) {
	logger := log.FromContext(ctx)
	old := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: node.Spec.ConfigMapRef, Namespace: node.Namespace}, old)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, fmt.Sprintf("configmap %s not fount", node.Spec.ConfigMapRef))
		}
		logger.Error(err, "happen error when get configmap ref")
	}
	cur := old.DeepCopy()

	// set labels
	cur.Labels = MergeLabels(cur.Labels, LabelsForNode(node.Spec.ChainName, node.Name))
	// set reference
	if err := ctrl.SetControllerReference(node, cur, r.Scheme); err != nil {
		logger.Error(err, "node configmap SetControllerReference error")
		return false, err
	}
	if IsEqual(old, cur) {
		// equal
		return false, nil
	}
	// not equal
	return true, r.Update(ctx, cur)
}

func (r *CitaNodeReconciler) ReconcileLogConfigMap(ctx context.Context, node *citacloudv1.CitaNode) (bool, error) {
	logger := log.FromContext(ctx)
	old := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: GetLogConfigName(node.Name), Namespace: node.Namespace}, old)
	if apierrors.IsNotFound(err) {
		newObj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GetLogConfigName(node.Name),
				Namespace: node.Namespace,
			},
		}
		if err = r.updateLogConfigMap(ctx, node, newObj); err != nil {
			return false, err
		}
		logger.Info("create log config configmap....")
		return false, r.Create(ctx, newObj)
	} else if err != nil {
		return false, err
	}

	cur := old.DeepCopy()
	if err := r.updateLogConfigMap(ctx, node, cur); err != nil {
		return false, err
	}
	if IsEqual(old, cur) {
		logger.Info("the log configmap part has not changed, go pass")
		return false, nil
	}

	logger.Info(fmt.Sprintf("should update node [%s/%s] log configmap...", node.Namespace, node.Name))
	return true, nil
}

func (r *CitaNodeReconciler) updateLogConfigMap(ctx context.Context, node *citacloudv1.CitaNode, configMap *corev1.ConfigMap) error {
	logger := log.FromContext(ctx)
	configMap.Labels = MergeLabels(configMap.Labels, LabelsForNode(node.Spec.ChainName, node.Name))
	if err := ctrl.SetControllerReference(node, configMap, r.Scheme); err != nil {
		logger.Error(err, "log configmap SetControllerReference error")
		return err
	}

	cnService := NewChainNodeServiceForLog(node)

	configMap.Data = map[string]string{
		ControllerLogConfigFile: cnService.GenerateControllerLogConfig(),
		ExecutorLogConfigFile:   cnService.GenerateExecutorLogConfig(),
		KmsLogConfigFile:        cnService.GenerateKmsLogConfig(),
		NetworkLogConfigFile:    cnService.GenerateNetworkLogConfig(),
		StorageLogConfigFile:    cnService.GenerateStorageLogConfig(),
		ConsensusLogConfigFile:  cnService.GenerateConsensusLogConfig(),
	}
	return nil
}
