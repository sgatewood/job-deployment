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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv1alpha1 "github.com/sgatewood/job-deployment/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("JobDeployment Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ns",
			},
		}
		namespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: ns.Name,
		}
		parent := &apiv1alpha1.JobDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespacedName.Namespace,
			},
			Spec: apiv1alpha1.JobDeploymentSpec{
				JobSpec: v1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: "Never",
							Containers: []corev1.Container{
								{
									Name:  "hello",
									Image: "busybox",
									Command: []string{
										"echo",
										"Hello world",
									},
								},
							},
						},
					},
				},
			},
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind JobDeployment")
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			Expect(k8sClient.Create(ctx, parent)).To(Succeed())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &apiv1alpha1.JobDeployment{}
			err := k8sClient.Get(ctx, namespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance JobDeployment")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &JobDeploymentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			child := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, namespacedName, child)).To(Succeed())

			Expect(child.Spec.Template.Spec.RestartPolicy).To(Equal(parent.Spec.JobSpec.Template.Spec.RestartPolicy))

			containers := child.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			container := containers[0]
			Expect(container.Name).To(Equal(parent.Spec.JobSpec.Template.Spec.Containers[0].Name))
			Expect(container.Image).To(Equal(parent.Spec.JobSpec.Template.Spec.Containers[0].Image))
			Expect(container.Command).To(Equal(parent.Spec.JobSpec.Template.Spec.Containers[0].Command))
		})
	})
})
