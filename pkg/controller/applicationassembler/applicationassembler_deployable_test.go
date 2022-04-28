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
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	prulev1alpha1 "github.com/hybridapp-io/ham-placement/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
)

func TestCreateDeployables(t *testing.T) {
	g := NewWithT(t)

	var (
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
			Name:      "foo-app",
			Namespace: "default",
		}

		applicationAssembler = &toolsv1alpha1.ApplicationAssembler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      applicationAssemblerKey.Name,
				Namespace: applicationAssemblerKey.Namespace,
			},
			Spec: toolsv1alpha1.ApplicationAssemblerSpec{
				ManagedClustersComponents: []*toolsv1alpha1.ClusterComponent{
					{
						Cluster: mc.Name,
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

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer func() {
		// cleanup hdpl
		dplList := &hdplv1alpha1.DeployableList{}
		g.Expect(c.List(context.TODO(), dplList)).NotTo(HaveOccurred())
		for _, hdpl := range dplList.Items {
			hdpl := hdpl
			g.Expect(c.Delete(context.TODO(), &hdpl)).NotTo(HaveOccurred())
		}
		// cleanup hpr
		hprList := &prulev1alpha1.PlacementRuleList{}
		g.Expect(c.List(context.TODO(), hprList)).NotTo(HaveOccurred())
		for _, hpr := range hprList.Items {
			hpr := hpr
			g.Expect(c.Delete(context.TODO(), &hpr)).NotTo(HaveOccurred())
		}
		// cleanup the appasm
		g.Expect(c.Delete(context.TODO(), instance)).NotTo(HaveOccurred())

	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	hybrddplyblKey := types.NamespacedName{Name: mc.Name + "-service-" + mcService.Namespace + "-" + mcService.Name, Namespace: applicationAssemblerKey.Namespace}

	nameLabel := map[string]string{
		hdplv1alpha1.HostingHybridDeployable: hybrddplyblKey.Name,
	}
	dplList := &dplv1.DeployableList{}
	g.Expect(c.List(context.TODO(), dplList, &client.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set(nameLabel))})).NotTo(HaveOccurred())
	g.Expect(dplList.Items).To(HaveLen(0))
}

func TestDeployableTemplates(t *testing.T) {
	g := NewWithT(t)

	var (
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
		mcServiceDeployable = &dplv1.Deployable{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mcServiceName,
				Namespace: mcName,
				Annotations: map[string]string{
					hdplv1alpha1.AnnotationHybridDiscovery: "true",
				},
			},
			Spec: dplv1.DeployableSpec{
				Template: &runtime.RawExtension{
					Object: &mcService,
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
				ManagedClustersComponents: []*toolsv1alpha1.ClusterComponent{
					{
						Cluster: mc.Name,
						Components: []*corev1.ObjectReference{
							{
								APIVersion: mcServiceDeployable.APIVersion,
								Kind:       mcServiceDeployable.Kind,
								Name:       mcServiceDeployable.Name,
								Namespace:  mcServiceDeployable.Namespace,
							},
						},
					},
				},
			},
		}
	)
	managedCluster := corev1.ObjectReference{
		Name:       mcName,
		APIVersion: "cluster.open-cluster-management.io/v1",
	}
	placementRuleName := "foo-app-foo-deployable"
	placementRuleNamespace := applicationAssemblerKey.Namespace

	placementRule := &prulev1alpha1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placementRuleName,
			Namespace: placementRuleNamespace,
		},
		Spec: prulev1alpha1.PlacementRuleSpec{
			Targets: []corev1.ObjectReference{
				managedCluster,
			},
		},
	}
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
	prule := placementRule.DeepCopy()
	g.Expect(c.Create(context.TODO(), prule)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.Background(), prule); err != nil {
			klog.Error(err)
		}
	}()

	dpl1 := mcServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
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
			hdpl := hdpl
			g.Expect(c.Delete(context.TODO(), &hdpl)).NotTo(HaveOccurred())
		}
		// cleanup hpr
		hprList := &prulev1alpha1.PlacementRuleList{}
		g.Expect(c.List(context.TODO(), hprList)).NotTo(HaveOccurred())
		for _, hpr := range hprList.Items {
			hpr := hpr
			g.Expect(c.Delete(context.TODO(), &hpr)).NotTo(HaveOccurred())
		}
		// cleanup the appasm
		g.Expect(c.Delete(context.TODO(), instance)).NotTo(HaveOccurred())

	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
	hybrddplyblKey := types.NamespacedName{Name: mc.Name + "-service-" + mcService.Namespace + "-" + mcService.Name, Namespace: applicationAssemblerKey.Namespace}

	hdpl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hdpl)).NotTo(HaveOccurred())

	// validate the template attributes
	obj := &unstructured.Unstructured{}
	err = json.Unmarshal(hdpl.Spec.HybridTemplates[0].Template.Raw, obj)
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	mcServiceInTemplate := &corev1.Service{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, mcServiceInTemplate)
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	g.Expect(mcServiceInTemplate.UID).To(BeEmpty())
	g.Expect(mcServiceInTemplate.ManagedFields).To(BeNil())
	g.Expect(mcServiceInTemplate.SelfLink).To(BeEmpty())
	g.Expect(mcServiceInTemplate.ResourceVersion).To(BeEmpty())
}
