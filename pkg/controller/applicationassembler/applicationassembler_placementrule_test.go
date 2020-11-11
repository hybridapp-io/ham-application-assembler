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
	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	prulev1alpha1 "github.com/hybridapp-io/ham-placement/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestHPRTargets(t *testing.T) {
	g := NewWithT(t)

	var (
		mc1Name = "mc1"
		mc1NS   = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: mc1Name,
			},
		}
		mc1 = &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mc1Name,
				Namespace: mc1Name,
			},
		}

		mc2Name = "mc2"
		mc2NS   = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: mc2Name,
			},
		}
		mc2 = &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mc2Name,
				Namespace: mc2Name,
			},
		}

		mcServiceName = "mysql-svc-mc"
		mcService     = corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      mcServiceName,
				Namespace: "default",
			},
		}

		applicationAssemblerKey = types.NamespacedName{
			Name:      "hpr",
			Namespace: "default",
		}

		// assembler with components in two managed clusters
		applicationAssembler = &toolsv1alpha1.ApplicationAssembler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      applicationAssemblerKey.Name,
				Namespace: applicationAssemblerKey.Namespace,
			},
			Spec: toolsv1alpha1.ApplicationAssemblerSpec{
				ManagedClustersComponents: []*toolsv1alpha1.ClusterComponent{
					{
						Cluster: mc1.Namespace + "/" + mc1.Name,
						Components: []*corev1.ObjectReference{
							{
								APIVersion: mcService.APIVersion,
								Kind:       mcService.Kind,
								Name:       mcService.Name,
								Namespace:  mcService.Namespace,
							},
						},
					},
					{
						Cluster: mc2.Namespace + "/" + mc2.Name,
						Components: []*corev1.ObjectReference{
							{
								APIVersion: mcService.APIVersion,
								Kind:       mcService.Kind,
								Name:       mcService.Name,
								Namespace:  mcService.Namespace,
							},
						},
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

	g.Expect(c.Create(context.TODO(), mc1NS)).To(Succeed())
	g.Expect(c.Create(context.TODO(), mc2NS)).To(Succeed())

	g.Expect(c.Create(context.TODO(), mc1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), mc1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	g.Expect(c.Create(context.TODO(), mc2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), mc2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer func() {
		// cleanup hdpl
		dplList := &hdplv1alpha1.DeployableList{}
		g.Expect(c.List(context.TODO(), dplList)).NotTo(HaveOccurred())
		for _, hdpl := range dplList.Items {
			g.Expect(c.Delete(context.TODO(), &hdpl)).NotTo(HaveOccurred())
		}
		// cleanup hpr
		hprList := &prulev1alpha1.PlacementRuleList{}
		g.Expect(c.List(context.TODO(), hprList)).NotTo(HaveOccurred())
		for _, hpr := range hprList.Items {
			g.Expect(c.Delete(context.TODO(), &hpr)).NotTo(HaveOccurred())
		}
		// cleanup the appasm
		g.Expect(c.Delete(context.TODO(), instance)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
	hprList := &prulev1alpha1.PlacementRuleList{}
	g.Expect(c.List(context.TODO(), hprList)).NotTo(HaveOccurred())
	g.Expect(hprList.Items).To(HaveLen(2))
	g.Expect(hprList.Items[0].Spec.Targets).To(HaveLen(1))
	g.Expect(hprList.Items[1].Spec.Targets).To(HaveLen(1))
}
