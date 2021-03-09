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

package application

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	prulev1alpha1 "github.com/hybridapp-io/ham-placement/pkg/apis/core/v1alpha1"
)

var (
	timeout = time.Second * 2

	selectorName   = "app.kubernetes.io/name"
	appName        = "wordpress"
	selectorLabels = map[string]string{
		selectorName: appName,
	}

	mc1Name = "mc1"
	mc1NS   = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: mc1Name,
		},
	}

	mc2Name = "mc2"
	mc2NS   = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: mc2Name,
		},
	}

	localClusterName = "local-cluster"
	localClusterNS   = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: localClusterName,
		},
	}

	mc1ServiceName = "mysql-svc-mc1"
	mc2ServiceName = "webserver-svc-mc2"

	mc1 = &managedclusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: mc1Name,
		},
	}

	mc1ServicePort = corev1.ServicePort{
		Name: "mc1-service-port",
		Port: 8080,
	}

	mc2ServicePort = corev1.ServicePort{
		Name: "mc2-service-port",
		Port: 8080,
	}

	mc1Service = corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc1ServiceName,
			Namespace: mc1Name,
			Labels: map[string]string{
				selectorName: appName,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				mc1ServicePort,
			},
		},
	}

	mc1ServiceDeployable = &dplv1.Deployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc1ServiceName,
			Namespace: mc1Name,
			Annotations: map[string]string{
				hdplv1alpha1.AnnotationHybridDiscovery: "true",
				hdplv1alpha1.HostingHybridDeployable:   "default/" + mc1ServiceName,
			},
			Labels: map[string]string{
				selectorName: appName,
			},
		},
		Spec: dplv1.DeployableSpec{
			Template: &runtime.RawExtension{
				Object: &mc1Service,
			},
		},
	}

	mc2 = &managedclusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: mc2Name,
		},
	}

	mc2Service = corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc2ServiceName,
			Namespace: mc2Name,
			Labels: map[string]string{
				selectorName: appName,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				mc2ServicePort,
			},
		},
	}
	mc2ServiceDeployable = &dplv1.Deployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc2ServiceName,
			Namespace: mc2Name,
			Annotations: map[string]string{
				hdplv1alpha1.AnnotationHybridDiscovery: "true",
			},
			Labels: map[string]string{
				selectorName: appName,
			},
		},
		Spec: dplv1.DeployableSpec{
			Template: &runtime.RawExtension{
				Object: &mc2Service,
			},
		},
	}

	localCluster = &managedclusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: localClusterName,
		},
	}

	// application
	applicationKey = types.NamespacedName{
		Name:      appName,
		Namespace: "default",
	}

	applicationKey2 = types.NamespacedName{
		Name:      appName,
		Namespace: mc1Name,
	}

	application2 = &sigappv1beta1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: mc1Name,
			Labels:    selectorLabels,
			Annotations: map[string]string{
				hdplv1alpha1.AnnotationHybridDiscovery: hdplv1alpha1.HybridDiscoveryEnabled,
				hdplv1alpha1.AnnotationClusterScope:    "true",
			},
		},
		Spec: sigappv1beta1.ApplicationSpec{
			ComponentGroupKinds: []metav1.GroupKind{
				{
					Group: "v1",
					Kind:  "Service",
				},
				{
					Group: "apps",
					Kind:  "StatefulSet",
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					selectorName: appName,
				},
			},
		},
	}

	application = &sigappv1beta1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: "default",
			Labels:    selectorLabels,
			Annotations: map[string]string{
				hdplv1alpha1.AnnotationHybridDiscovery: hdplv1alpha1.HybridDiscoveryEnabled,
				hdplv1alpha1.AnnotationClusterScope:    "true",
			},
		},
		Spec: sigappv1beta1.ApplicationSpec{
			ComponentGroupKinds: []metav1.GroupKind{
				{
					Group: "v1",
					Kind:  "Service",
				},
				{
					Group: "apps",
					Kind:  "StatefulSet",
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					selectorName: appName,
				},
			},
		},
	}

	// hybrid deployable for remote resource
	hpr1Name        = "hpr-1"
	hpr1Namespace   = "default"
	mc1DeployerType = "kubernetes"
	hpr1            = &prulev1alpha1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpr1Name,
			Namespace: hpr1Namespace,
		},
		Spec: prulev1alpha1.PlacementRuleSpec{
			DeployerType: &mc1DeployerType,
			TargetLabels: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": mc1Name},
			},
		},
	}

	mc1HybridTemplate = hdplv1alpha1.HybridTemplate{
		DeployerType: mc1DeployerType,
		Template: &runtime.RawExtension{
			Object: &mc1Service,
		},
	}

	mc1Hdpl = &hdplv1alpha1.Deployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc1ServiceName,
			Namespace: "default",
			Labels: map[string]string{
				selectorName: appName,
			},
		},
		Spec: hdplv1alpha1.DeployableSpec{
			HybridTemplates: []hdplv1alpha1.HybridTemplate{
				mc1HybridTemplate,
			},
			Placement: &hdplv1alpha1.HybridPlacement{
				PlacementRef: &corev1.ObjectReference{
					Name:      hpr1Name,
					Namespace: hpr1Namespace,
				},
			},
		},
	}

	// hybrid deployable using deployer for ifrastructure management
	imDeployerName      = "imDeployer"
	imDeployerNamespace = "default"
	imDeployerType      = "ibminfra"

	hpr2Name      = "hpr-2"
	hpr2Namespace = "default"
	hpr2          = &prulev1alpha1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpr2Name,
			Namespace: hpr2Namespace,
		},
		Spec: prulev1alpha1.PlacementRuleSpec{
			DeployerType: &imDeployerType,
		},
	}

	// Dummy object for hybrid template
	vmObject = &corev1.ConfigMap{}

	imHybridTemplate = hdplv1alpha1.HybridTemplate{
		DeployerType: imDeployerType,
		Template: &runtime.RawExtension{
			Object: vmObject,
		},
	}

	imHdpl = &hdplv1alpha1.Deployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vm-test",
			Namespace: "default",
			Labels: map[string]string{
				selectorName: appName,
			},
		},
		Spec: hdplv1alpha1.DeployableSpec{
			HybridTemplates: []hdplv1alpha1.HybridTemplate{
				imHybridTemplate,
			},
			Placement: &hdplv1alpha1.HybridPlacement{
				PlacementRef: &corev1.ObjectReference{
					Name:      hpr2Name,
					Namespace: hpr2Namespace,
				},
			},
		},
	}

	hybridApp = &sigappv1beta1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: "default",
			Labels:    selectorLabels,
			Annotations: map[string]string{
				hdplv1alpha1.AnnotationHybridDiscovery: hdplv1alpha1.HybridDiscoveryEnabled,
			},
		},
		Spec: sigappv1beta1.ApplicationSpec{
			ComponentGroupKinds: []metav1.GroupKind{
				{
					Group: "core.hybridapp.io",
					Kind:  "Deployable",
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					selectorName: appName,
				},
			},
		},
	}

	// configmap for application relationships
	relationshipsCM = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: "default",
		},
	}
)

func TestReconcile(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: applicationKey}

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

	ns1 := mc1NS.DeepCopy()
	g.Expect(c.Create(context.TODO(), ns1)).To(Succeed())

	ns2 := mc2NS.DeepCopy()
	g.Expect(c.Create(context.TODO(), ns2)).To(Succeed())

	localClusterNS := localClusterNS.DeepCopy()
	g.Expect(c.Create(context.TODO(), localClusterNS)).To(Succeed())

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	app := application.DeepCopy()
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
}

func Test_ApplicationAssemblerComponents_In_MultipleManagedCluster(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: applicationKey}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	appReconciler := newReconciler(mgr)
	appRecFn, appRequests := SetupTestReconcile(appReconciler)

	g.Expect(add(mgr, appRecFn)).NotTo(HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces

	cl1 := mc1.DeepCopy()
	g.Expect(c.Create(context.TODO(), cl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), cl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	cl2 := mc2.DeepCopy()
	g.Expect(c.Create(context.TODO(), cl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), cl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl1 := mc1ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl2 := mc2ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object and expect the hybrid deployables in its status
	app := application.DeepCopy()
	app.Annotations[toolsv1alpha1.AnnotationCreateAssembler] = toolsv1alpha1.HybridDiscoveryCreateAssembler
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// wait for reconcile to finish
	g.Eventually(appRequests, timeout).Should(Receive(Equal(expectedRequest)))

	appasm := &toolsv1alpha1.ApplicationAssembler{}
	g.Expect(c.Get(context.TODO(), applicationKey, appasm)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), appasm); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// validate the appasm components , 2 cluster components
	g.Expect(appasm.Spec.ManagedClustersComponents).To(HaveLen(2))

	components := []*corev1.ObjectReference{
		{
			Namespace:  dpl1.Namespace,
			Kind:       toolsv1alpha1.DeployableGVK.Kind,
			Name:       dpl1.Name,
			APIVersion: toolsv1alpha1.DeployableGVK.Group + "/" + toolsv1alpha1.DeployableGVK.Version,
		},
		{
			Namespace:  dpl2.Namespace,
			Kind:       toolsv1alpha1.DeployableGVK.Kind,
			Name:       dpl2.Name,
			APIVersion: toolsv1alpha1.DeployableGVK.Group + "/" + toolsv1alpha1.DeployableGVK.Version,
		},
	}
	for _, comp := range appasm.Spec.ManagedClustersComponents {
		g.Expect(comp.Components[0]).To(BeElementOf(components))
	}

	// validate the hybrid-discover-create-assembler annotation
	g.Expect(c.Get(context.TODO(), applicationKey, app)).NotTo(HaveOccurred())
	g.Expect(app.Annotations[toolsv1alpha1.AnnotationCreateAssembler]).To(Equal(toolsv1alpha1.AssemblerCreationCompleted))
}

func Test_ApplicationAssemblerComponents_In_SingleManagedCluster(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: applicationKey}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	appReconciler := newReconciler(mgr)
	appRecFn, appRequests := SetupTestReconcile(appReconciler)

	g.Expect(add(mgr, appRecFn)).NotTo(HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces

	cl1 := mc1.DeepCopy()
	g.Expect(c.Create(context.TODO(), cl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), cl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl1 := mc1ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl2 := mc2ServiceDeployable.DeepCopy()
	dpl2.Namespace = mc1Name
	g.Expect(c.Create(context.TODO(), dpl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object and expect the hybrid deployables in its status
	app := application.DeepCopy()
	app.Annotations[toolsv1alpha1.AnnotationCreateAssembler] = toolsv1alpha1.HybridDiscoveryCreateAssembler
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// wait for reconcile to finish
	g.Eventually(appRequests, timeout).Should(Receive(Equal(expectedRequest)))

	appasm := &toolsv1alpha1.ApplicationAssembler{}
	g.Expect(c.Get(context.TODO(), applicationKey, appasm)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), appasm); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// validate the appasm components , 1 cluster components
	g.Expect(appasm.Spec.ManagedClustersComponents).To(HaveLen(1))
	// 2 components in the first cluster component
	g.Expect(appasm.Spec.ManagedClustersComponents[0].Components).To(HaveLen(2))

	components := []*corev1.ObjectReference{
		{
			Namespace:  dpl1.Namespace,
			Kind:       toolsv1alpha1.DeployableGVK.Kind,
			Name:       dpl1.Name,
			APIVersion: toolsv1alpha1.DeployableGVK.Group + "/" + toolsv1alpha1.DeployableGVK.Version,
		},
		{
			Namespace:  dpl1.Namespace,
			Kind:       toolsv1alpha1.DeployableGVK.Kind,
			Name:       dpl2.Name,
			APIVersion: toolsv1alpha1.DeployableGVK.Group + "/" + toolsv1alpha1.DeployableGVK.Version,
		},
	}
	for _, comp := range appasm.Spec.ManagedClustersComponents[0].Components {
		g.Expect(comp).To(BeElementOf(components))
	}
}

func Test_ApplicationAssemblerComponents_ClusterScoped_False(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: applicationKey}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	appReconciler := newReconciler(mgr)
	appRecFn, appRequests := SetupTestReconcile(appReconciler)

	g.Expect(add(mgr, appRecFn)).NotTo(HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces

	cl1 := mc1.DeepCopy()
	g.Expect(c.Create(context.TODO(), cl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), cl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl1Service := mc1Service.DeepCopy()
	dpl1Service.Namespace = "default"

	dpl1 := mc1ServiceDeployable.DeepCopy()
	dpl1.Spec.Template.Object = dpl1Service

	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// this deployable should not be picked up by the assembler as it's
	// resource is not in the same namespace as the app
	dpl2 := mc2ServiceDeployable.DeepCopy()
	dpl2.Namespace = mc1Name
	g.Expect(c.Create(context.TODO(), dpl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object with create-assembler and clusterscoped
	// annotations set to true
	app := application.DeepCopy()
	app.Annotations[toolsv1alpha1.AnnotationCreateAssembler] = toolsv1alpha1.HybridDiscoveryCreateAssembler
	app.Annotations[hdplv1alpha1.AnnotationClusterScope] = "false"
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// wait for reconcile to finish
	g.Eventually(appRequests, timeout).Should(Receive(Equal(expectedRequest)))

	appasm := &toolsv1alpha1.ApplicationAssembler{}
	g.Expect(c.Get(context.TODO(), applicationKey, appasm)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), appasm); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// validate the appasm components , 1 cluster components
	g.Expect(appasm.Spec.ManagedClustersComponents).To(HaveLen(1))
	// 1 component in the first cluster component
	g.Expect(appasm.Spec.ManagedClustersComponents[0].Components).To(HaveLen(1))

	components := []*corev1.ObjectReference{
		{
			Namespace:  dpl1.Namespace,
			Kind:       toolsv1alpha1.DeployableGVK.Kind,
			Name:       dpl1.Name,
			APIVersion: toolsv1alpha1.DeployableGVK.Group + "/" + toolsv1alpha1.DeployableGVK.Version,
		},
	}
	for _, comp := range appasm.Spec.ManagedClustersComponents[0].Components {
		g.Expect(comp).To(BeElementOf(components))
	}
}
