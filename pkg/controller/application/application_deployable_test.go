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

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
)

func TestApplicationDeployable(t *testing.T) {
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
	defer c.Delete(context.TODO(), cl1)

	cl2 := mc2.DeepCopy()
	g.Expect(c.Create(context.TODO(), cl2)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), cl2)

	// Create the Application object and expect the Reconcile and Deployable to be created
	app := application.DeepCopy()
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	dplList := &dplv1.DeployableList{}
	g.Expect(c.List(context.TODO(), dplList, &client.ListOptions{LabelSelector: labels.Set(selectorLabels).AsSelector()})).NotTo(HaveOccurred())
	g.Expect(dplList.Items).To(HaveLen(2))

	for _, dpl := range dplList.Items {
		g.Expect(dpl.Namespace).To(BeElementOf([]string{mc1Name, mc2Name}))
	}

	// app cleanup should also delete the app deployables
	c.Delete(context.TODO(), app)
	// wait for reconcile to finish
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	newList := &dplv1.DeployableList{}
	g.Expect(c.List(context.TODO(), newList, &client.ListOptions{LabelSelector: labels.Set(selectorLabels).AsSelector()})).NotTo(HaveOccurred())
	g.Expect(newList.Items).To(HaveLen(0))
}
