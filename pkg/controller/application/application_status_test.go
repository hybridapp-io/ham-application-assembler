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

	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
)

func TestDiscoveredComponents(t *testing.T) {
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
	defer c.Delete(context.TODO(), dpl1)

	dpl2 := mc2ServiceDeployable.DeepCopy()
	g.Expect(c.Create(context.Background(), dpl2)).NotTo(HaveOccurred())
	defer c.Delete(context.Background(), dpl2)

	// Create the Application object and expect the deployables
	app := application.DeepCopy()
	g.Expect(c.Create(context.TODO(), app)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), app)
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
