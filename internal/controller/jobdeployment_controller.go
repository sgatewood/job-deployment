/*
Copyright 2025.

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

package controller

import (
	"context"
	"github.com/google/go-cmp/cmp"
	apiv1alpha1 "github.com/sgatewood/job-deployment/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

// JobDeploymentReconciler reconciles a JobDeployment object
type JobDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=api.sgatewood.dev,resources=jobdeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=api.sgatewood.dev,resources=jobdeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=api.sgatewood.dev,resources=jobdeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch/v1,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch/v1,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch/v1,resources=jobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the JobDeployment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *JobDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	parent := &apiv1alpha1.JobDeployment{}
	if err := r.Get(ctx, req.NamespacedName, parent); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("parent doesn't exist -- nothing to do")
			return ctrl.Result{}, nil
		}
		l.Error(err, "could not get parent")
		return ctrl.Result{Requeue: true}, err
	}
	l = l.WithValues("name", req.Name, "namespace", req.Namespace)

	child := &batchv1.Job{}
	if err := r.Get(ctx, req.NamespacedName, child); err != nil {
		if apierrors.IsNotFound(err) {
			return r.createChild(ctx, parent, child)
		}
		l.Error(err, "could not get child")
		return ctrl.Result{Requeue: true}, err
	}
	return r.deleteChildIfSpecDiffers(ctx, parent, child)
}

func (r *JobDeploymentReconciler) createChild(ctx context.Context, parent *apiv1alpha1.JobDeployment, child *batchv1.Job) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("creating child")

	child.Name = parent.Name
	child.Namespace = parent.Namespace
	child.Spec = parent.Spec.JobSpec

	if err := controllerutil.SetOwnerReference(parent, child, r.Scheme); err != nil {
		l.Error(err, "could not set owner refs")
		return ctrl.Result{Requeue: true}, err
	}

	if err := r.Create(ctx, child); err != nil {
		l.Error(err, "could not create child")
		return ctrl.Result{Requeue: true}, err
	}

	l.Info("created child")
	return ctrl.Result{}, nil
}

func (r *JobDeploymentReconciler) deleteChildIfSpecDiffers(ctx context.Context, parent *apiv1alpha1.JobDeployment, child *batchv1.Job) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	if diff := cmp.Diff(parent.Spec.JobSpec, child.Spec); diff != "" {
		return r.deleteChild(ctx, child)
	}

	l.Info("child is up to date")
	return ctrl.Result{}, nil
}

func (r *JobDeploymentReconciler) deleteChild(ctx context.Context, child *batchv1.Job) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	l.Info("child spec differs -- deleting child")
	if child.GetDeletionTimestamp() != nil {
		l.Info("still deleting child")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	deletePolicy := metav1.DeletePropagationForeground
	if err := r.Delete(ctx, child, &client.DeleteOptions{PropagationPolicy: &deletePolicy}); err != nil {
		l.Error(err, "could not delete child")
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.JobDeployment{}).
		Named("jobdeployment").
		Complete(r)
}
