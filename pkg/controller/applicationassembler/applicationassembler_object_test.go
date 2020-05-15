// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package applicationassembler

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
)

func TestHubComponents(t *testing.T) {
	g := NewWithT(t)

	var (
		serviceName = "mysql-svc-mc"

		service = corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: "default",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Port: 3306,
					},
				},
			},
		}

		applicationAssemblerKey = types.NamespacedName{
			Name:      "foo-app",
			Namespace: "default",
		}

		applicationAssembler = &toolsv1alpha1.ApplicationAssembler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      applicationAssemblerKey.Name,
				Namespace: applicationAssemblerKey.Namespace,
			},
			Spec: toolsv1alpha1.ApplicationAssemblerSpec{
				HubComponents: []*corev1.ObjectReference{
					{
						APIVersion: service.APIVersion,
						Kind:       service.Kind,
						Name:       service.Name,
						Namespace:  service.Namespace,
					},
				},
			},
		}
	)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: applicationAssemblerKey}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	recFn, requests := SetupTestReconcile(rec)
	g.Expect(add(mgr, recFn)).NotTo(HaveOccurred())
	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	svc := service.DeepCopy()
	g.Expect(c.Create(context.TODO(), svc)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), svc)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	hybrddplyblKey := types.NamespacedName{Name: "service-" + service.Namespace + "-" + service.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())
}
