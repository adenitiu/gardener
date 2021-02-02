// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seedadmission_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/seedadmission"
)

var _ = Describe("Pod Scheduler Name", func() {
	Describe("#DefaultShootControlPlanePodsSchedulerName", func() {
		var (
			ctx       context.Context
			request   admission.Request
			validator admission.Handler
		)

		BeforeEach(func() {
			ctx = context.Background()
			validator = admission.HandlerFunc(DefaultShootControlPlanePodsSchedulerName)

			request = admission.Request{}
			request.Operation = admissionv1.Create
			request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
		})

		It("should ignore other operations than CREATE", func() {
			request.Operation = admissionv1.Delete
			expectAllowed(validator.Handle(ctx, request), ContainSubstring("not CREATE"))
			request.Operation = admissionv1.Update
			expectAllowed(validator.Handle(ctx, request), ContainSubstring("not CREATE"))
			request.Operation = admissionv1.Connect
			expectAllowed(validator.Handle(ctx, request), ContainSubstring("not CREATE"))
		})

		It("should ignore other resources than Pods", func() {
			request.Kind = metav1.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"}
			expectAllowed(validator.Handle(ctx, request), ContainSubstring("not corev1.Pod"))
		})

		It("should ignore subresources", func() {
			request.SubResource = "logs"
			expectAllowed(validator.Handle(ctx, request), ContainSubstring("subresource"))
		})

		It("should default schedulerName", func() {
			expectPatched(validator.Handle(ctx, request), ContainSubstring("shoot control plane pod"), []jsonpatch.JsonPatchOperation{
				jsonpatch.NewOperation("replace", "/spec/schedulerName", "gardener-shoot-controlplane-scheduler"),
			})
		})
	})

	Describe("Integration Test", func() {
		var (
			c client.Client

			namespace = "shoot--foo--bar"
			pod       *corev1.Pod
		)

		BeforeEach(func() {
			c, err = client.New(restConfig, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			_, err = controllerutil.CreateOrPatch(ctx, c, ns, func() error {
				ns.SetLabels(map[string]string{
					"gardener.cloud/role": "shoot",
				})
				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test",
						Image: "foo:latest",
					}},
				},
			}
		})

		AfterEach(func() {
			Expect(client.IgnoreNotFound(c.Delete(ctx, pod))).To(Succeed())
		})

		Describe("ignored requests", func() {
			BeforeEach(func() {
				Expect(c.Create(ctx, pod)).To(Succeed())
			})

			It("Update does nothing", func() {
				expected := &corev1.Pod{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(pod), expected)).To(Succeed())
				Expect(c.Update(ctx, pod)).To(Succeed())
				Expect(pod).To(Equal(expected))
			})
			It("Delete does nothing", func() {
				Expect(c.Delete(ctx, pod)).To(Succeed())
			})
		})

		It("should set .spec.schedulerName", func() {
			Expect(c.Create(ctx, pod)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			Expect(pod.Spec.SchedulerName).To(Equal("gardener-shoot-controlplane-scheduler"))
		})
	})
})
