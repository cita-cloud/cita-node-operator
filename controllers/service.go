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

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *CitaNodeReconciler) ReconcileService(ctx context.Context, node *citacloudv1.CitaNode) error {
	logger := log.FromContext(ctx)
	old := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: GetNodePortServiceName(node.Name), Namespace: node.Namespace}, old)
	if errors.IsNotFound(err) {
		newObj := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GetNodePortServiceName(node.Name),
				Namespace: node.Namespace,
			},
		}
		if err = r.updateService(ctx, node, newObj); err != nil {
			return err
		}
		logger.Info("create node service....")
		return r.Create(ctx, newObj)
	} else if err != nil {
		return err
	}

	logger.Info("service update is currently not supported, go pass")
	return nil
}

func (r *CitaNodeReconciler) updateService(ctx context.Context, node *citacloudv1.CitaNode, service *corev1.Service) error {
	labels := MergeLabels(service.Labels, LabelsForNode(node.Spec.ChainName, node.Name))
	logger := log.FromContext(ctx)
	service.Labels = labels
	if err := ctrl.SetControllerReference(node, service, r.Scheme); err != nil {
		logger.Error(err, "node service SetControllerReference error")
		return err
	}

	service.Spec = corev1.ServiceSpec{
		Selector: labels,
		Ports: []corev1.ServicePort{
			// randomly generated nodePort
			{
				Name:       "network",
				Port:       NetworkPort,
				TargetPort: intstr.FromInt(NetworkPort),
			},
			{
				Name:       "rpc",
				Port:       ControllerPort,
				TargetPort: intstr.FromInt(ControllerPort),
			},
			{
				Name:       "call",
				Port:       ExecutorPort,
				TargetPort: intstr.FromInt(ExecutorPort),
			},
		},
		Type: corev1.ServiceTypeNodePort,
	}

	return nil
}
