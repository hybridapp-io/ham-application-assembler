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

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	"github.com/hybridapp-io/ham-application-assembler/pkg/utils"
	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	prulev1alpha1 "github.com/hybridapp-io/ham-placement/pkg/apis/core/v1alpha1"
	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
)

var (
	serviceName      = "mysql-svc"
	stsName          = "mysql-sts"
	applicationName  = "wordpress-01"
	appLabelSelector = "app.kubernetes.io/name"

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

	sts = &apps.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      stsName,
			Namespace: "default",
		},
		Spec: apps.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{appLabelSelector: applicationName},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{appLabelSelector: applicationName},
				},
			},
		},
	}

	applicationAssembler1Key = types.NamespacedName{
		Name:      "foo",
		Namespace: "default",
	}

	applicationAssembler2Key = types.NamespacedName{
		Name:      "bar",
		Namespace: "default",
	}

	applicationAssembler1 = &toolsv1alpha1.ApplicationAssembler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationAssembler1Key.Name,
			Namespace: applicationAssembler1Key.Namespace,
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

	applicationAssembler2 = &toolsv1alpha1.ApplicationAssembler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationAssembler2Key.Name,
			Namespace: applicationAssembler2Key.Namespace,
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

	applicationAssemblerHub = &toolsv1alpha1.ApplicationAssembler{
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
				{
					APIVersion: sts.APIVersion,
					Kind:       sts.Kind,
					Name:       sts.Name,
					Namespace:  sts.Namespace,
				},
			},
		},
	}

	fooDeployer = &prulev1alpha1.Deployer{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo",
			Namespace:   "default",
			Annotations: map[string]string{prulev1alpha1.DeployerInCluster: "true"},
		},
		Spec: prulev1alpha1.DeployerSpec{
			Type:  "foo",
			Scope: apiextensions.ClusterScoped,
		},
	}
)

func TestHubComponents(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: applicationAssembler1Key}

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
	defer func() {
		g.Expect(c.Delete(context.TODO(), svc)).NotTo(HaveOccurred())
	}()

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler1.DeepCopy()
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

	hybrddplyblKey := types.NamespacedName{Name: "service-" + service.Namespace + "-" + service.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())
}

func TestHubComponentsAnnotations(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest1 = reconcile.Request{NamespacedName: applicationAssembler1Key}
	var expectedRequest2 = reconcile.Request{NamespacedName: applicationAssembler2Key}
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
	instance1 := applicationAssembler1.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance1)).NotTo(HaveOccurred())
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
		g.Expect(c.Delete(context.TODO(), instance1)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest1)))

	app1 := &sigappv1beta1.Application{}
	app1Key := types.NamespacedName{Name: applicationAssembler1Key.Name, Namespace: applicationAssembler1Key.Namespace}
	g.Expect(c.Get(context.TODO(), app1Key, app1)).NotTo(HaveOccurred())

	instance2 := applicationAssembler2.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance2)).NotTo(HaveOccurred())
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
		g.Expect(c.Delete(context.TODO(), instance2)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest2)))

	app2 := &sigappv1beta1.Application{}
	app2Key := types.NamespacedName{Name: applicationAssembler2Key.Name, Namespace: applicationAssembler2Key.Namespace}
	g.Expect(c.Get(context.TODO(), app2Key, app2)).NotTo(HaveOccurred())

	hybrddplyblKey := types.NamespacedName{Name: "service-" + service.Namespace + "-" + service.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())

	labels := hybrddplybl.Labels
	g.Expect(labels[toolsv1alpha1.LabelApplicationPrefix+string(app1.GetUID())]).To(Equal(string(app1.GetUID())))
	g.Expect(labels[toolsv1alpha1.LabelApplicationPrefix+string(app2.GetUID())]).To(Equal(string(app2.GetUID())))

}

func TestHubComponentsPlacementBySingleDeployer(t *testing.T) {
	g := NewWithT(t)

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

	sts := sts.DeepCopy()
	g.Expect(c.Create(context.TODO(), sts)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), sts)

	deployer := fooDeployer.DeepCopy()
	deployer.Spec.Capabilities = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*"},
			Verbs:     []string{"*"},
		},
	}
	g.Expect(c.Create(context.TODO(), deployer)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), deployer)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssemblerHub.DeepCopy()
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

	svcHdplKey := types.NamespacedName{Name: "service-" + service.Namespace + "-" + service.Name, Namespace: applicationAssemblerKey.Namespace}
	svcHdpl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), svcHdplKey, svcHdpl)).NotTo(HaveOccurred())
	g.Expect(svcHdpl.Spec.Placement.PlacementRef).NotTo(BeNil())

	stsHdplKey := types.NamespacedName{Name: "statefulset-" + sts.Namespace + "-" + sts.Name, Namespace: applicationAssemblerKey.Namespace}
	stsHdpl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), stsHdplKey, stsHdpl)).NotTo(HaveOccurred())

	g.Expect(stsHdpl.Spec.HybridTemplates).To(HaveLen(1))
	g.Expect(stsHdpl.Spec.HybridTemplates[0].DeployerType).To(Equal(fooDeployer.Spec.Type))

	g.Expect(stsHdpl.Spec.Placement.PlacementRef).NotTo(BeNil())

}

func TestHubComponentsPlacementByDualDeployer(t *testing.T) {
	g := NewWithT(t)

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

	sts := sts.DeepCopy()
	g.Expect(c.Create(context.TODO(), sts)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), sts)

	stsDeployer := fooDeployer.DeepCopy()
	stsDeployer.Name = "sts-" + fooDeployer.Name
	stsDeployer.Spec.Capabilities = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apps"},
			Resources: []string{"statefulsets"},
			Verbs:     []string{"*"},
		},
	}
	g.Expect(c.Create(context.TODO(), stsDeployer)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), stsDeployer)

	svcDeployer := fooDeployer.DeepCopy()
	svcDeployer.Name = "svc-" + fooDeployer.Name
	svcDeployer.Spec.Capabilities = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{"*"},
		},
	}
	g.Expect(c.Create(context.TODO(), svcDeployer)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), svcDeployer)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssemblerHub.DeepCopy()
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

	svcHdplKey := types.NamespacedName{Name: "service-" + service.Namespace + "-" + service.Name, Namespace: applicationAssemblerKey.Namespace}
	svcHdpl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), svcHdplKey, svcHdpl)).NotTo(HaveOccurred())

	g.Expect(svcHdpl.Spec.HybridTemplates).To(HaveLen(1))
	g.Expect(svcHdpl.Spec.HybridTemplates[0].DeployerType).To(Equal(svcDeployer.Spec.Type))

	g.Expect(svcHdpl.Spec.Placement.PlacementRef).NotTo(BeNil())

	stsHdplKey := types.NamespacedName{Name: "statefulset-" + sts.Namespace + "-" + sts.Name, Namespace: applicationAssemblerKey.Namespace}
	stsHdpl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), stsHdplKey, stsHdpl)).NotTo(HaveOccurred())

	g.Expect(stsHdpl.Spec.HybridTemplates).To(HaveLen(1))
	g.Expect(stsHdpl.Spec.HybridTemplates[0].DeployerType).To(Equal(stsDeployer.Spec.Type))

	g.Expect(stsHdpl.Spec.Placement.PlacementRef).NotTo(BeNil())

}

func TestHubComponentsPlacementByDefaultDeployer(t *testing.T) {
	g := NewWithT(t)

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

	sts := sts.DeepCopy()
	g.Expect(c.Create(context.TODO(), sts)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), sts)

	stsDeployer := fooDeployer.DeepCopy()
	stsDeployer.Name = "sts-" + fooDeployer.Name
	stsDeployer.Spec.Capabilities = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"*"},
		},
	}
	g.Expect(c.Create(context.TODO(), stsDeployer)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), stsDeployer)

	svcDeployer := fooDeployer.DeepCopy()
	svcDeployer.Name = "svc-" + fooDeployer.Name
	svcDeployer.Spec.Capabilities = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"*"},
		},
	}
	g.Expect(c.Create(context.TODO(), svcDeployer)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), svcDeployer)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssemblerHub.DeepCopy()
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

	svcHdplKey := types.NamespacedName{Name: "service-" + service.Namespace + "-" + service.Name, Namespace: applicationAssemblerKey.Namespace}
	svcHdpl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), svcHdplKey, svcHdpl)).NotTo(HaveOccurred())

	g.Expect(svcHdpl.Spec.HybridTemplates).To(HaveLen(1))
	g.Expect(svcHdpl.Spec.HybridTemplates[0].DeployerType).To(Equal(toolsv1alpha1.DefaultDeployerType))

	stsHdplKey := types.NamespacedName{Name: "statefulset-" + sts.Namespace + "-" + sts.Name, Namespace: applicationAssemblerKey.Namespace}
	stsHdpl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), stsHdplKey, stsHdpl)).NotTo(HaveOccurred())

	g.Expect(stsHdpl.Spec.HybridTemplates).To(HaveLen(1))
	g.Expect(stsHdpl.Spec.HybridTemplates[0].DeployerType).To(Equal(toolsv1alpha1.DefaultDeployerType))
}

func TestComponentsNameLength(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

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

	newLongName := utils.TruncateString("very-long-name-exceeding-maximum-length-allowed-for-kubernetes-label-values", toolsv1alpha1.GeneratedDeployableNameLength)
	svc := service.DeepCopy()
	svc.Name = newLongName
	g.Expect(c.Create(context.TODO(), svc)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), svc)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance1 := applicationAssembler1.DeepCopy()
	instance1.Spec.HubComponents[0].Name = newLongName
	g.Expect(c.Create(context.TODO(), instance1)).NotTo(HaveOccurred())
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
		g.Expect(c.Delete(context.TODO(), instance1)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive())

	hdplName := utils.TruncateString("service-"+service.Namespace+"-"+newLongName, toolsv1alpha1.GeneratedDeployableNameLength)
	hybrddplyblKey := types.NamespacedName{Name: hdplName, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())
	g.Expect(hybrddplybl.Name).To(HaveLen(toolsv1alpha1.GeneratedDeployableNameLength))
}
