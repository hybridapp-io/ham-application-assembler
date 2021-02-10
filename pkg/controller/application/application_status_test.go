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
	"k8s.io/klog"

	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
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
			Group: toolsv1alpha1.DeployableGVK.Group,
			Kind:  toolsv1alpha1.DeployableGVK.Kind,
			Name:  dpl1.Name,
			Link:  dpl1.SelfLink,
		},
		{
			Group: toolsv1alpha1.DeployableGVK.Group,
			Kind:  toolsv1alpha1.DeployableGVK.Kind,
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
			Group: toolsv1alpha1.DeployableGVK.Group,
			Kind:  toolsv1alpha1.DeployableGVK.Kind,
			Name:  dpl1.Name,
			Link:  dpl1.SelfLink,
		},
		{
			Group: toolsv1alpha1.DeployableGVK.Group,
			Kind:  toolsv1alpha1.DeployableGVK.Kind,
			Name:  dpl2.Name,
			Link:  dpl2.SelfLink,
		},
	}
	for _, comp := range app.Status.ComponentList.Objects {
		g.Expect(comp).To(BeElementOf(components))
	}

	// Check that configmap was created
	relationshipsConfigmap := configMap.DeepCopy()
	relationshipsConfigmap.Reset()
	g.Expect(c.Get(context.TODO(), applicationKey, relationshipsConfigmap)).NotTo(HaveOccurred())

	// Verify if all relationships exist
	expectedRelationships := []Relationship{
		Relationship{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  app.GetNamespace(),
			SourceApiGroup:   app.GroupVersionKind().Group,
			SourceApiVersion: app.GroupVersionKind().Version,
			SourceKind:       "application",
			SourceName:       app.GetName(),
			Dest:             "k8s",
			DestCluster:      dpl1.GetClusterName(),
			DestNamespace:    dpl1.GetNamespace(),
			DestApiGroup:     toolsv1alpha1.DeployableGVK.Group,
			DestApiVersion:   toolsv1alpha1.DeployableGVK.Version,
			DestKind:         toolsv1alpha1.DeployableGVK.Kind,
			DestName:         dpl1.GetName(),
		},
		Relationship{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  app.GetNamespace(),
			SourceApiGroup:   app.GroupVersionKind().Group,
			SourceApiVersion: app.GroupVersionKind().Version,
			SourceKind:       "application",
			SourceName:       app.GetName(),
			Dest:             "k8s",
			DestCluster:      dpl2.GetClusterName(),
			DestNamespace:    dpl2.GetNamespace(),
			DestApiGroup:     toolsv1alpha1.DeployableGVK.Group,
			DestApiVersion:   toolsv1alpha1.DeployableGVK.Version,
			DestKind:         toolsv1alpha1.DeployableGVK.Kind,
			DestName:         dpl2.GetName(),
		},
	}

	// Need to convert the relationships from string to byte array to array of
	// structs
	actualRelationships := []Relationship{}
	relationshipsByteArray := []byte(relationshipsConfigmap.Data["relationships"])
	err = json.Unmarshal(relationshipsByteArray, &actualRelationships)
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	for _, relationship := range expectedRelationships {
		g.Expect(relationship).To(BeElementOf(actualRelationships))
	}
}
