/*
Copyright 2023.

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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	rhinooprapiv1alpha1 "github.com/OpenRHINO/RHINO-Operator/api/v1alpha1"
	kbatchv1 "k8s.io/api/batch/v1"
)

var _ = Describe("RhinoJob controller", func() {
	Context("RhinoJob controller test", func() {

		const RhinoJobName = "test-rhinojob"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      RhinoJobName,
				Namespace: RhinoJobName,
			},
		}

		namespacedName := types.NamespacedName{Name: RhinoJobName, Namespace: RhinoJobName}
		namespacedNameLauncher := types.NamespacedName{Name: RhinoJobName + "-launcher", Namespace: RhinoJobName}
		namespacedNameWorkers := types.NamespacedName{Name: RhinoJobName + "-workers", Namespace: RhinoJobName}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))

			By("Setting the Image ENV VAR which stores the Operand image")
			err = os.Setenv("RHINOJOB_IMAGE", "openrhino/image:test")
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			// TODO(user): Attention if you improve this code by adding other context test you MUST
			// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)

			By("Removing the Image ENV VAR which stores the Operand image")
			_ = os.Unsetenv("RHINOJOB_IMAGE")
		})

		It("should successfully reconcile a custom resource for RhinoJob", func() {
			By("Creating the custom resource for the Kind RhinoJob")
			rhinojob := &rhinooprapiv1alpha1.RhinoJob{}
			err := k8sClient.Get(ctx, namespacedName, rhinojob)
			if err != nil && errors.IsNotFound(err) {
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				rhinojobTTL := int32(300)
				rhinojobParallelism := int32(2)
				rhinojob := &rhinooprapiv1alpha1.RhinoJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      RhinoJobName,
						Namespace: namespace.Name,
					},
					Spec: rhinooprapiv1alpha1.RhinoJobSpec{
						Image:       "zhuhe0321/integration",
						TTL:         &rhinojobTTL,
						Parallelism: &rhinojobParallelism,
						AppExec:     "./integration",
						AppArgs:     []string{"1", "10", "1"},
					},
				}

				err = k8sClient.Create(ctx, rhinojob)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &rhinooprapiv1alpha1.RhinoJob{}
				return k8sClient.Get(ctx, namespacedName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if the launcher job was successfully created in the reconciliation")
			Eventually(func() error {
				found := &kbatchv1.Job{}
				return k8sClient.Get(ctx, namespacedNameLauncher, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if the workers job was successfully created in the reconciliation")
			Eventually(func() error {
				found := &kbatchv1.Job{}
				return k8sClient.Get(ctx, namespacedNameWorkers, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Get current rhinojob object")
			err = k8sClient.Get(ctx, namespacedName, rhinojob)
			Expect(err).To(Not(HaveOccurred()))

			By("Checking the final status of launcher and workers jobs")
			Eventually(func() error {
				foundLauncherJob := &kbatchv1.Job{}
				foundWorkersJob := &kbatchv1.Job{}

				err1 := k8sClient.Get(ctx, namespacedNameLauncher, foundLauncherJob)
				if err1 != nil {
					return err1
				}
				err2 := k8sClient.Get(ctx, namespacedNameWorkers, foundWorkersJob)
				if err2 != nil {
					return err2
				}

				if foundLauncherJob.Status.Succeeded == 1 && foundWorkersJob.Status.Succeeded == *rhinojob.Spec.Parallelism {
					return nil
				}
				return fmt.Errorf("Jobs not completed")
			}, 2*time.Minute, time.Second).Should(Succeed())

			By("Checing the final status of rhinojob")
			Eventually(func() error {
				err := k8sClient.Get(ctx, namespacedName, rhinojob)
				if err != nil {
					return err
				}

				if rhinojob.Status.JobStatus == rhinooprapiv1alpha1.Completed {
					return nil
				}
				return fmt.Errorf("Rhinojob not completed")
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
