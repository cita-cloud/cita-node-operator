/*
Copyright 2022.

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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	citacloudv1 "github.com/cita-cloud/cita-node-operator/api/v1"
)

// CitaNodeReconciler reconciles a CitaNode object
type CitaNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=citanodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=citanodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.rivtower.com,resources=citanodes/finalizers,verbs=update

func (r *CitaNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("citanode %s in reconcile", req.NamespacedName))

	node := &citacloudv1.CitaNode{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		logger.Info(fmt.Sprintf("the citanode %s has been deleted", req.NamespacedName))
		NodeMap.Delete(req.NamespacedName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	updated, err := r.SetDefaultStatus(ctx, node)
	if updated || err != nil {
		return ctrl.Result{}, err
	}

	nv := NewNodeValue(node.Status.Status)
	NodeMap.Store(req.NamespacedName, nv)

	if err := r.ReconcileAllRecourse(ctx, node); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.SyncStatus(ctx, node); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CitaNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.CitaNode{}).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(r.statefulSetPredicates())).
		Complete(r)
}

func (r *CitaNodeReconciler) statefulSetPredicates() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(event event.UpdateEvent) bool {
			curSet := event.ObjectNew.(*appsv1.StatefulSet)
			oldSet := event.ObjectOld.(*appsv1.StatefulSet)
			if reflect.DeepEqual(curSet.Status, oldSet.Status) {
				return false
			}
			return true
		},
	}
}

func (r *CitaNodeReconciler) SetDefaultStatus(ctx context.Context, node *citacloudv1.CitaNode) (bool, error) {
	logger := log.FromContext(ctx)
	if node.Status.Status == "" {
		node.Status.Status = citacloudv1.Starting
		err := r.Client.Status().Update(ctx, node)
		if err != nil {
			logger.Error(err, fmt.Sprintf("set citanode default status [%s] failed", node.Status.Status))
			return false, err
		}
		logger.Info(fmt.Sprintf("set citanode default status [%s] success", node.Status.Status))
		return true, nil
	}
	return false, nil
}

func (r *CitaNodeReconciler) ReconcileAllRecourse(ctx context.Context, node *citacloudv1.CitaNode) error {
	var err error
	//var updateFlag bool
	// reconcile configmap
	_, err = r.ReconcileConfigMap(ctx, node)
	if err != nil {
		return err
	}
	// reconcile log configmap
	_, err = r.ReconcileLogConfigMap(ctx, node)
	if err != nil {
		return err
	}
	// reconcile service
	if err := r.ReconcileService(ctx, node); err != nil {
		return err
	}
	// reconcile configmap
	newNodeValue, err := r.ReconcileStatefulSet(ctx, node)
	if err != nil && newNodeValue == nil {
		return err
	}
	if newNodeValue == nil {
		// if create or no change
		return nil
	}
	// happen update
	key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}
	oldValue, ok := NodeMap.Load(key)
	if !ok {
		return fmt.Errorf("load %v error", key)
	}
	if oldValue.(*NodeValue).Status != newNodeValue.Status {
		NodeMap.Store(key, newNodeValue)
	}
	return nil
}

func (r *CitaNodeReconciler) SyncStatus(ctx context.Context, node *citacloudv1.CitaNode) error {
	logger := log.FromContext(ctx)
	key := types.NamespacedName{Name: node.Name, Namespace: node.Namespace}
	value, ok := NodeMap.Load(key)
	if !ok {
		return fmt.Errorf("load %v error", key)
	}
	if value.(*NodeValue).Status != node.Status.Status {
		logger.Info(fmt.Sprintf("update action: convert status from [%s] to [%s]", node.Status.Status, value.(*NodeValue).Status))
		node.Status.Status = value.(*NodeValue).Status
		return r.Status().Update(ctx, node)
	}
	if value.(*NodeValue).Status == citacloudv1.Starting {
		sts := &appsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{Name: node.Name, Namespace: node.Namespace}, sts)
		if err != nil {
			return err
		}
		if sts.Status.Replicas == sts.Status.ReadyReplicas {
			logger.Info("convert status from [Starting] to [Running]")
			node.Status.Status = citacloudv1.Running
			return r.Status().Update(ctx, node)
		}
	}
	if value.(*NodeValue).Status == citacloudv1.Stopping {
		pod := &corev1.Pod{}
		err := r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-0", node.Name), Namespace: node.Namespace}, pod)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("convert status from [Stopping] to [Stopped]")
				node.Status.Status = citacloudv1.Stopped
				return r.Status().Update(ctx, node)
			}
			return err
		}
	}
	if value.(*NodeValue).Status == citacloudv1.Running {
		pod := &corev1.Pod{}
		err := r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-0", node.Name), Namespace: node.Namespace}, pod)
		if err != nil {
			return err
		}
		for _, status := range pod.Status.ContainerStatuses {
			if !status.Ready {
				logger.Info("convert status from [Running] to [Error]")
				node.Status.Status = citacloudv1.Error
				return r.Status().Update(ctx, node)
			}
		}
	}
	if value.(*NodeValue).Status == citacloudv1.Error {
		pod := &corev1.Pod{}
		err := r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-0", node.Name), Namespace: node.Namespace}, pod)
		if err != nil {
			return err
		}
		allReady := true
		for _, status := range pod.Status.ContainerStatuses {
			if !status.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			logger.Info("convert status from [Error] to [Running]")
			node.Status.Status = citacloudv1.Running
			return r.Status().Update(ctx, node)
		}
	}
	return nil
}
