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

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workapiv1 "github.com/open-cluster-management/api/work/v1"
)

func TestApplicationManifestwork(t *testing.T) {
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

	// Stand up the infrastructure
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

	// Create the Application object and expect the Reconcile and Manifestwork to be created
	app := application.DeepCopy()
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	mworklist := &workapiv1.ManifestWorkList{}
	g.Expect(c.List(context.TODO(), mworklist, &client.ListOptions{LabelSelector: labels.Set(selectorLabels).AsSelector()})).NotTo(HaveOccurred())
	g.Expect(mworklist.Items).To(HaveLen(2))

	for _, mwork := range mworklist.Items {
		g.Expect(mwork.Namespace).To(BeElementOf([]string{mc1Name, mc2Name}))
	}

	// app cleanup should also delete the app manifestworks

	if err = c.Delete(context.TODO(), app); err != nil {
		klog.Error(err)
		t.Fail()
	}

	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	newList := &workapiv1.ManifestWorkList{}
	g.Expect(c.List(context.TODO(), newList, &client.ListOptions{LabelSelector: labels.Set(selectorLabels).AsSelector()})).NotTo(HaveOccurred())
	g.Expect(newList.Items).To(HaveLen(0))
}

func TestApplicationManifestworkIgnoredClusters(t *testing.T) {
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

	// Stand up the infrastructure
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

	localCluster := localCluster.DeepCopy()
	g.Expect(c.Create(context.TODO(), localCluster)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), localCluster); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the Application object and expect the Reconcile and Manifestwork to be created
	app := application.DeepCopy()
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	mworklist := &workapiv1.ManifestWorkList{}
	g.Expect(c.List(context.TODO(), mworklist, &client.ListOptions{})).NotTo(HaveOccurred())

	// ensure local-cluster is not generating any app manifestwork
	g.Expect(mworklist.Items).To(HaveLen(2))

	for _, mwork := range mworklist.Items {
		g.Expect(mwork.Namespace).To(BeElementOf([]string{mc1Name, mc2Name}))
	}

	// app cleanup should also delete the app manifestworks

	if err = c.Delete(context.TODO(), app); err != nil {
		klog.Error(err)
		t.Fail()
	}

	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	newList := &workapiv1.ManifestWorkList{}
	g.Expect(c.List(context.TODO(), newList, &client.ListOptions{LabelSelector: labels.Set(selectorLabels).AsSelector()})).NotTo(HaveOccurred())
	g.Expect(newList.Items).To(HaveLen(0))
}

func TestApplicationManifestworkTarget(t *testing.T) {
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

	// Stand up the infrastructure
	cl1 := mc1.DeepCopy()
	g.Expect(c.Create(context.TODO(), cl1)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), cl1); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()
	cl1ObjReference := corev1.ObjectReference{
		Name:       cl1.Name,
		Kind:       cl1.Kind,
		APIVersion: cl1.APIVersion,
	}
	cl2 := mc2.DeepCopy()
	g.Expect(c.Create(context.TODO(), cl2)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), cl2); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	targetJSON, err := json.Marshal(cl1ObjReference)
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	// Create the Application object and expect the Reconcile and Manifestwork to be created
	app := application.DeepCopy()
	app.Annotations[toolsv1alpha1.AnnotationDiscoveryTarget] = string(targetJSON)
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), app); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	mworklist := &workapiv1.ManifestWorkList{}
	g.Expect(c.List(context.TODO(), mworklist, &client.ListOptions{LabelSelector: labels.Set(selectorLabels).AsSelector()})).NotTo(HaveOccurred())
	g.Expect(mworklist.Items).To(HaveLen(1))

	g.Expect(mworklist.Items[0].Namespace).To(Equal(cl1.Name))

}
