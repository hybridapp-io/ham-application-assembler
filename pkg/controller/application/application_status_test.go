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
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"

	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
)

func TestDiscoveredComponentsInSameNamespace(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: applicationKey}
	var expectedRequest2 = reconcile.Request{NamespacedName: applicationKey2}

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

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces

	svc1 := mc1Service.DeepCopy()

	g.Expect(c.Create(context.TODO(), svc1)).NotTo(HaveOccurred())

	defer func() {
		if err = c.Delete(context.TODO(), svc1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	svc2 := mc2Service.DeepCopy()

	g.Expect(c.Create(context.Background(), svc2)).NotTo(HaveOccurred())

	defer func() {
		if err = c.Delete(context.Background(), svc2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object and expect the deployables

	app := application.DeepCopy()

	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())

	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// 0 resources should now be in the app status
	g.Expect(c.Get(context.TODO(), applicationKey, app)).NotTo(HaveOccurred())
	g.Expect(app.Status.ComponentList.Objects).To(HaveLen(0))

	app2 := application2.DeepCopy()

	g.Expect(c.Create(context.TODO(), app2)).NotTo(HaveOccurred())

	defer func() {
		if err = c.Delete(context.TODO(), app2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	//wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest2)))
	g.Expect(c.Get(context.TODO(), applicationKey2, app2)).NotTo(HaveOccurred())

	g.Expect(app2.Status.ComponentList.Objects).To(HaveLen(1))
}

func TestDiscoveredComponentsWithLabelSelector(t *testing.T) {
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

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces
	dpl1 := mc1ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl2 := mc2ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.Background(), dpl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.Background(), dpl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object and expect the deployables
	app := application.DeepCopy()
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// the two services should now be in the app status
	g.Expect(c.Get(context.TODO(), applicationKey, app)).NotTo(HaveOccurred())

	g.Expect(app.Status.ComponentList.Objects).To(HaveLen(2))
	components := []sigappv1beta1.ObjectStatus{
		{
			Group: toolsv1alpha1.ManifestworkGVK.Group,
			Kind:  toolsv1alpha1.ManifestworkGVK.Kind,
			Name:  dpl1.Name,
			Link:  dpl1.SelfLink,
		},
		{
			Group: toolsv1alpha1.ManifestworkGVK.Group,
			Kind:  toolsv1alpha1.ManifestworkGVK.Kind,
			Name:  dpl2.Name,
			Link:  dpl2.SelfLink,
		},
	}
	for _, comp := range app.Status.ComponentList.Objects {
		g.Expect(comp).To(BeElementOf(components))
	}
}

func TestDiscoveredComponentsWithNoSelector(t *testing.T) {
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

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces
	dpl1 := mc1ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl2 := mc2ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.Background(), dpl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.Background(), dpl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object and expect the deployables
	app := application.DeepCopy()
	app.Spec.Selector = nil
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// the two services should now be in the app status
	g.Expect(c.Get(context.TODO(), applicationKey, app)).NotTo(HaveOccurred())
	g.Expect(app.Status.ComponentList.Objects).To(HaveLen(0))
}

func TestDiscoveredComponentsWithNoLabelSelector(t *testing.T) {
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

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces
	dpl1 := mc1ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl2 := mc2ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.Background(), dpl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.Background(), dpl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object and expect the deployables
	app := application.DeepCopy()
	app.Spec.Selector.MatchLabels = nil
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// the two services should now be in the app status
	g.Expect(c.Get(context.TODO(), applicationKey, app)).NotTo(HaveOccurred())
	g.Expect(app.Status.ComponentList.Objects).To(HaveLen(0))
}

func TestDiscoveredComponentsWithEmptyLabelSelector(t *testing.T) {
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

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces
	dpl1 := mc1ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dpl2 := mc2ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.Background(), dpl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.Background(), dpl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object and expect the deployables
	app := application.DeepCopy()
	// empty label selector
	app.Spec.Selector.MatchLabels = make(map[string]string)
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// the two services should now be in the app status
	g.Expect(c.Get(context.TODO(), applicationKey, app)).NotTo(HaveOccurred())

	// label selector is provided but empty, reconcile will make it nil, so no components
	g.Expect(app.Status.ComponentList.Objects).To(HaveLen(0))
}

func TestRelatedResourcesConfigMap(t *testing.T) {
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

	// Stand up the infrastructure: managed cluster namespaces, deployables in mc namespaces
	hdpl1 := mc1Hdpl.DeepCopy()
	g.Expect(c.Create(context.TODO(), hdpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), hdpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	hpr1 := hpr1.DeepCopy()
	g.Expect(c.Create(context.TODO(), hpr1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), hpr1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Update decision of pr
	hpr1.Status.Decisions = []corev1.ObjectReference{
		{
			Kind: "ManagedCluster",
			Name: mc1Name,
		},
	}
	g.Expect(c.Status().Update(context.TODO(), hpr1)).NotTo(HaveOccurred())

	dpl1 := mc1ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dpl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	hdpl2 := imHdpl.DeepCopy()
	g.Expect(c.Create(context.TODO(), hdpl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), hdpl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	hpr2 := hpr2.DeepCopy()
	g.Expect(c.Create(context.TODO(), hpr2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), hpr2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Update decision of pr
	hpr2.Status.Decisions = []corev1.ObjectReference{
		{
			Namespace:  imDeployerNamespace,
			Kind:       "Deployer",
			Name:       imDeployerName,
			APIVersion: "core.hybridapp.io/v1alpha1",
		},
	}
	g.Expect(c.Status().Update(context.TODO(), hpr2)).NotTo(HaveOccurred())

	// Create vm resource using dynamic client
	gvr := schema.GroupVersionResource{
		Group:    "infra.management.ibm.com",
		Version:  "v1alpha1",
		Resource: "virtualmachines",
	}
	vmObj := &unstructured.Unstructured{}
	vmObj.SetName("vm-resource")
	vmObj.SetNamespace("default")
	vmObj.SetKind("VirtualMachine")
	vmObj.SetAPIVersion("infra.management.ibm.com/v1alpha1")
	vmObj.SetAnnotations(map[string]string{
		hdplv1alpha1.HostingHybridDeployable: "default/" + hdpl2.GetName(),
	})

	dynamicclient, err := dynamic.NewForConfig(mgr.GetConfig())
	vmObj, err = dynamicclient.Resource(gvr).Namespace("default").Create(context.TODO(), vmObj, metav1.CreateOptions{})

	// Create the Application object and expect the deployables
	app := hybridApp.DeepCopy()
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())

	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// the two hybrid deployables should now be in the app status
	g.Expect(c.Get(context.TODO(), applicationKey, app)).NotTo(HaveOccurred())

	g.Expect(app.Status.ComponentList.Objects).To(HaveLen(2))
	components := []sigappv1beta1.ObjectStatus{
		{
			Group: toolsv1alpha1.HybridDeployableGK.Group,
			Kind:  toolsv1alpha1.HybridDeployableGK.Kind,
			Name:  hdpl1.Name,
			Link:  hdpl1.SelfLink,
		},
		{
			Group: toolsv1alpha1.HybridDeployableGK.Group,
			Kind:  toolsv1alpha1.HybridDeployableGK.Kind,
			Name:  hdpl2.Name,
			Link:  hdpl2.SelfLink,
		},
	}
	for _, comp := range app.Status.ComponentList.Objects {
		g.Expect(comp).To(BeElementOf(components))
	}

	// Check that configmap was created
	relationshipsCM := relationshipsCM.DeepCopy()
	relationshipsCM.Reset()
	g.Expect(c.Get(context.TODO(), applicationKey, relationshipsCM)).NotTo(HaveOccurred())

	// Verify if all relationships exist
	expectedRelationships := []Relationship{
		{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  app.GetNamespace(),
			SourceAPIGroup:   app.GroupVersionKind().Group,
			SourceAPIVersion: app.GroupVersionKind().Version,
			SourceKind:       app.GroupVersionKind().Kind,
			SourceName:       app.GetName(),
			Dest:             "k8s",
			DestCluster:      "local-cluster",
			DestNamespace:    hdpl1.GetNamespace(),
			DestAPIGroup:     toolsv1alpha1.HybridDeployableGK.Group,
			DestAPIVersion:   "v1alpha1",
			DestKind:         toolsv1alpha1.HybridDeployableGK.Kind,
			DestName:         hdpl1.GetName(),
		},
		{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  app.GetNamespace(),
			SourceAPIGroup:   app.GroupVersionKind().Group,
			SourceAPIVersion: app.GroupVersionKind().Version,
			SourceKind:       app.GroupVersionKind().Kind,
			SourceName:       app.GetName(),
			Dest:             "k8s",
			DestCluster:      "local-cluster",
			DestNamespace:    hdpl2.GetNamespace(),
			DestAPIGroup:     toolsv1alpha1.HybridDeployableGK.Group,
			DestAPIVersion:   "v1alpha1",
			DestKind:         toolsv1alpha1.HybridDeployableGK.Kind,
			DestName:         hdpl2.GetName(),
		},
		{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  hdpl1.GetNamespace(),
			SourceAPIGroup:   toolsv1alpha1.HybridDeployableGK.Group,
			SourceAPIVersion: "v1alpha1",
			SourceKind:       toolsv1alpha1.HybridDeployableGK.Kind,
			SourceName:       hdpl1.GetName(),
			Dest:             "k8s",
			DestCluster:      "local-cluster",
			DestNamespace:    hpr1.GetNamespace(),
			DestAPIGroup:     "core.hybridapp.io",
			DestAPIVersion:   "v1alpha1",
			DestKind:         "PlacementRule",
			DestName:         hpr1.GetName(),
		},
		{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  hdpl1.GetNamespace(),
			SourceAPIGroup:   toolsv1alpha1.HybridDeployableGK.Group,
			SourceAPIVersion: "v1alpha1",
			SourceKind:       toolsv1alpha1.HybridDeployableGK.Kind,
			SourceName:       hdpl1.GetName(),
			Dest:             "k8s",
			DestCluster:      "local-cluster",
			DestNamespace:    dpl1.GetNamespace(),
			DestAPIGroup:     toolsv1alpha1.ManifestworkGVK.Group,
			DestAPIVersion:   toolsv1alpha1.ManifestworkGVK.Version,
			DestKind:         toolsv1alpha1.ManifestworkGVK.Kind,
			DestName:         dpl1.GetName(),
		},
		{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  dpl1.GetNamespace(),
			SourceAPIGroup:   toolsv1alpha1.ManifestworkGVK.Group,
			SourceAPIVersion: toolsv1alpha1.ManifestworkGVK.Version,
			SourceKind:       toolsv1alpha1.ManifestworkGVK.Kind,
			SourceName:       dpl1.GetName(),
			Dest:             "k8s",
			DestCluster:      dpl1.GetNamespace(),
			DestNamespace:    mc1Service.GetNamespace(),
			DestAPIGroup:     mc1Service.GroupVersionKind().Group,
			DestAPIVersion:   mc1Service.GroupVersionKind().Version,
			DestKind:         mc1Service.GroupVersionKind().Kind,
			DestName:         mc1Service.GetName(),
		},
		{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  hdpl2.GetNamespace(),
			SourceAPIGroup:   toolsv1alpha1.HybridDeployableGK.Group,
			SourceAPIVersion: "v1alpha1",
			SourceKind:       toolsv1alpha1.HybridDeployableGK.Kind,
			SourceName:       hdpl2.GetName(),
			Dest:             "k8s",
			DestCluster:      "local-cluster",
			DestNamespace:    hpr2.GetNamespace(),
			DestAPIGroup:     "core.hybridapp.io",
			DestAPIVersion:   "v1alpha1",
			DestKind:         "PlacementRule",
			DestName:         hpr2.GetName(),
		},
		{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  hdpl2.GetNamespace(),
			SourceAPIGroup:   toolsv1alpha1.HybridDeployableGK.Group,
			SourceAPIVersion: "v1alpha1",
			SourceKind:       toolsv1alpha1.HybridDeployableGK.Kind,
			SourceName:       hdpl2.GetName(),
			Dest:             "im",
			DestUID:          string(vmObj.GetUID()),
		},
	}

	// Need to convert the relationships from string to byte array to array of
	// structs
	actualRelationships := []Relationship{}
	relationshipsByteArray := []byte(relationshipsCM.Data["relationships"])
	err = json.Unmarshal(relationshipsByteArray, &actualRelationships)
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	for _, relationship := range expectedRelationships {
		g.Expect(relationship).To(BeElementOf(actualRelationships))
	}

	// app cleanup should also delete the configmap
	if err = c.Delete(context.TODO(), app); err != nil {
		klog.Error(err)
		t.Fail()
	}
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	err = c.Get(context.TODO(), applicationKey, relationshipsCM)
	g.Expect(errors.IsNotFound(err)).To(BeTrue())
}
