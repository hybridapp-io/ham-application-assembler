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

package customresourcedefinition

import (
	"context"
	"testing"
	"time"

	"github.com/hybridapp-io/ham-application-assembler/pkg/utils"
	. "github.com/onsi/gomega"
	crds "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	timeout = time.Second * 5

	group    = "test.hybridapp.io"
	version  = "v1alpha1"
	kind     = "TestResource"
	resource = "testresources"

	// crd
	crdKey = types.NamespacedName{
		Name: resource + "." + group,
	}

	crd = &crds.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: resource + "." + group,
		},
		Spec: crds.CustomResourceDefinitionSpec{
			Group:   group,
			Names: crds.CustomResourceDefinitionNames{
				Singular: "testresource",
				Plural:   resource,
				Kind:     kind,
			},
			Scope: "Namespaced",
			Versions: []crds.CustomResourceDefinitionVersion{
				{
					Name: version,
					Served: true,
					Storage: true,
					Schema: &crds.CustomResourceValidation{
						OpenAPIV3Schema: &crds.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
		},
	}
)

func TestReconcile(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: crdKey}

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

	utils.BuildGVKGVRMap(mgr.GetConfig())
	_, ok := utils.GVKGVRMap[schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}]
	g.Expect(ok).To(Equal(false))

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	testCRD := crd.DeepCopy()
	g.Expect(c.Create(context.TODO(), testCRD)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), testCRD); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// at this point the GVKGVR map should have been refreshed
	gvr, ok := utils.GVKGVRMap[schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}]
	g.Expect(ok).To(Equal(true))
	g.Expect(gvr).NotTo(BeNil())
	g.Expect(gvr.Group).To(Equal(group))
	g.Expect(gvr.Resource).To(Equal(resource))
	g.Expect(gvr.Version).To(Equal(version))
}
