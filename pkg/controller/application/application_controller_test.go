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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
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

	mc1 = &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc1Name,
			Namespace: mc1Name,
		},
	}

	mc1Service = corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc1ServiceName,
			Namespace: mc1ServiceName,
			Labels: map[string]string{
				selectorName: appName,
			},
		},
	}

	mc1ServiceDeployable = &dplv1.Deployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc1ServiceName,
			Namespace: mc1Name,
			Annotations: map[string]string{
				hdplv1alpha1.AnnotationHybridDiscovery: "true",
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

	mc2 = &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc2Name,
			Namespace: mc2Name,
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

	localCluster = &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      localClusterName,
			Namespace: localClusterName,
		},
	}

	// application
	applicationKey = types.NamespacedName{
		Name:      "wordpress",
		Namespace: "default",
	}

	application = &sigappv1beta1.Application{
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
