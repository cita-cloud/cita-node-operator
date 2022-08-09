package controllers

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type JobRbac struct {
	client.Client
	Log            logr.Logger
	Namespace      string
	ServiceAccount string
	Role           string
	RoleBinding    string
}

func newJobRbac(client client.Client, log logr.Logger, namespace, serviceAccount, role, roleBinding string) *JobRbac {
	return &JobRbac{
		Client:         client,
		Log:            log,
		Namespace:      namespace,
		ServiceAccount: serviceAccount,
		Role:           role,
		RoleBinding:    roleBinding,
	}
}

func (r *JobRbac) Ensure(ctx context.Context) error {
	err := r.ensureServiceAccount(ctx)
	if err != nil {
		return err
	}
	err = r.ensureRole(ctx)
	if err != nil {
		return err
	}
	err = r.ensureRoleBinding(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *JobRbac) ensureServiceAccount(ctx context.Context) error {
	// Check if the serviceAccount already exists, if not create a new one
	foundServiceAccount := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: r.ServiceAccount, Namespace: r.Namespace}, foundServiceAccount)
	if err != nil && errors.IsNotFound(err) {
		sa := r.generateServiceAccount()
		r.Log.Info("creating a new service account")
		err = r.Create(ctx, sa)
		if err != nil {
			r.Log.Error(err, "failed to create new service account")
			return err
		}
	} else if err != nil {
		r.Log.Error(err, "failed to get service account")
		return err
	} else {
		// todo reconcile
	}
	return nil
}

func (r *JobRbac) ensureRole(ctx context.Context) error {
	// Check if the role already exists, if not create a new one
	foundRole := &rbacv1.ClusterRole{}
	err := r.Get(ctx, types.NamespacedName{Name: r.Role}, foundRole)
	if err != nil && errors.IsNotFound(err) {
		role := r.generateRole()
		r.Log.Info("creating a new role")
		err = r.Create(ctx, role)
		if err != nil {
			r.Log.Error(err, "failed to create new role")
			return err
		}
	} else if err != nil {
		r.Log.Error(err, "failed to get role")
		return err
	} else {
		// todo reconcile
	}
	return nil
}

func (r *JobRbac) ensureRoleBinding(ctx context.Context) error {
	// Check if the role binding already exists, if not create a new one
	foundRoleBinding := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, types.NamespacedName{Name: r.RoleBinding}, foundRoleBinding)
	if err != nil && errors.IsNotFound(err) {
		roleBinding := r.generateRoleBinding()
		r.Log.Info("creating a new role binding")
		err = r.Create(ctx, roleBinding)
		if err != nil {
			r.Log.Error(err, "failed to create new role binding")
			return err
		}
	} else if err != nil {
		r.Log.Error(err, "failed to get role binding")
		return err
	} else {
		var existServiceAccount bool
		for _, subject := range foundRoleBinding.Subjects {
			if subject.Name == r.ServiceAccount && subject.Namespace == r.Namespace {
				existServiceAccount = true
			}
		}
		if !existServiceAccount {
			foundRoleBinding.Subjects = append(foundRoleBinding.Subjects, rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      r.ServiceAccount,
				Namespace: r.Namespace,
			})
			err := r.Update(ctx, foundRoleBinding)
			if err != nil {
				r.Log.Error(err, "failed to update role binding")
				return err
			}
		}
	}
	return nil
}

func (r *JobRbac) generateServiceAccount() *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.ServiceAccount,
			Namespace: r.Namespace,
		},
	}
	return sa
}

func (r *JobRbac) generateRole() *rbacv1.ClusterRole {
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
	podPR := rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch", "update"},
		APIGroups: []string{""},
		Resources: []string{"pods"},
	}

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Role,
		},
		Rules: []rbacv1.PolicyRule{
			stsPR,
			depPR,
			podPR,
		},
	}
	return role
}

func (r *JobRbac) generateRoleBinding() *rbacv1.ClusterRoleBinding {
	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.RoleBinding,
			//Namespace: r.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      r.ServiceAccount,
				Namespace: r.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     r.Role,
		},
	}
	return roleBinding
}
