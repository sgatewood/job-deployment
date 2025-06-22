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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/sgatewood/job-deployment/internal/controller/status"
	"time"

	apiv1alpha1 "github.com/sgatewood/job-deployment/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const fieldManager = "sgatewood.dev/jobdeployment-controller"
const hashAnnotation = "job-spec-hash"

// hashJobSpec creates a SHA256 hash of the JobSpec
func hashJobSpec(spec batchv1.JobSpec) (string, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

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
		return ctrl.Result{}, err
	}
	l = l.WithValues("name", req.Name, "namespace", req.Namespace)

	child := &batchv1.Job{}
	if err := r.Get(ctx, req.NamespacedName, child); err != nil {
		if apierrors.IsNotFound(err) {
			return r.createChild(ctx, parent, child)
		}
		l.Error(err, "could not get child")
		return ctrl.Result{}, err
	}
	return r.deleteChildIfSpecDiffers(ctx, parent, child)
	// TODO: sync status
}

func (r *JobDeploymentReconciler) createChild(ctx context.Context, parent *apiv1alpha1.JobDeployment, child *batchv1.Job) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("creating child")

	defer func() {
		if err := r.applyParentStatus(ctx, parent, status.CreatingChild, batchv1.JobStatus{}); err != nil {
			l.Error(err, "could not apply parent status")
		}
	}()

	child.Name = parent.Name
	child.Namespace = parent.Namespace
	child.Spec = parent.Spec.JobSpec

	if err := controllerutil.SetControllerReference(parent, child, r.Scheme); err != nil {
		l.Error(err, "could not set owner refs")
		return ctrl.Result{}, err
	}

	parentSpecHash, err := hashJobSpec(parent.Spec.JobSpec)
	if err != nil {
		l.Error(err, "could not hash parent's job spec")
		return ctrl.Result{}, err
	}

	child.Annotations = map[string]string{
		hashAnnotation: parentSpecHash,
	}

	if err := r.Create(ctx, child, &client.CreateOptions{
		FieldManager: fieldManager,
	}); err != nil {
		l.Error(err, "could not create child")
		return ctrl.Result{}, err
	}

	l.Info("created child")
	return ctrl.Result{Requeue: true}, nil
}

func isController(parent *apiv1alpha1.JobDeployment, child *batchv1.Job) bool {
	for _, ref := range child.OwnerReferences {
		if (ref.Controller != nil && *ref.Controller) &&
			ref.APIVersion == parent.APIVersion &&
			ref.Kind == parent.Kind &&
			ref.Name == parent.Name &&
			ref.UID == parent.UID {
			return true
		}
	}
	return false
}

func (r *JobDeploymentReconciler) deleteChildIfSpecDiffers(ctx context.Context, parent *apiv1alpha1.JobDeployment, child *batchv1.Job) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	if !isController(parent, child) {
		return ctrl.Result{}, fmt.Errorf("we don't own job '%s' in namespace '%s'", child.Name, child.Namespace)
	}

	expectedHash, err := hashJobSpec(parent.Spec.JobSpec)
	if err != nil {
		l.Error(err, "could not hash parent's job spec")
		return ctrl.Result{}, err
	}

	actualHash, found := child.Annotations[hashAnnotation]
	if !found || expectedHash != actualHash {
		l.Info("hash annotation differs", "expected", expectedHash, "actual", actualHash)
		return r.deleteChild(ctx, parent, child)
	}

	l.Info("child is up to date")
	if err := r.applyParentStatus(ctx, parent, status.OK, child.Status); err != nil {
		l.Error(err, "could not update parent status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *JobDeploymentReconciler) deleteChild(ctx context.Context, parent *apiv1alpha1.JobDeployment, child *batchv1.Job) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	defer func() {
		if err := r.applyParentStatus(ctx, parent, status.CreatingChild, child.Status); err != nil {
			l.Error(err, "could not apply parent status")
		}
	}()

	l.Info("child spec differs -- deleting child")
	if child.GetDeletionTimestamp() != nil {
		l.Info("still deleting child")
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	deletePolicy := metav1.DeletePropagationForeground
	if err := r.Delete(ctx, child, &client.DeleteOptions{PropagationPolicy: &deletePolicy}); err != nil {
		l.Error(err, "could not delete child")
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
}

func (r *JobDeploymentReconciler) applyParentStatus(ctx context.Context, parent *apiv1alpha1.JobDeployment, status status.JobDeploymentStatus, jobStatus batchv1.JobStatus) error {
	parent.Status = apiv1alpha1.JobDeploymentStatus{
		JobStatus: jobStatus,
		Status:    string(status),
	}
	return r.Status().Update(ctx, parent)
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.JobDeployment{}).
		Named("jobdeployment").
		Complete(r)
}
