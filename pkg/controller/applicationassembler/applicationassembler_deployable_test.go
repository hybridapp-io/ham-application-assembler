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

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
)

func TestCreateDeployables(t *testing.T) {
	g := NewWithT(t)

	var (
		mcName        = "mc"
		mcServiceName = "mysql-svc-mc"

		mc = &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mcName,
				Namespace: mcName,
			},
		}

		mcService = corev1.Service{
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
						Cluster: mc.Namespace + "/" + mc.Name,
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
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	hybrddplyblKey := types.NamespacedName{Name: mc.Name + "-service-" + mcService.Namespace + "-" + mcService.Name, Namespace: applicationAssemblerKey.Namespace}

	nameLabel := map[string]string{
		hdplv1alpha1.HostingHybridDeployable: hybrddplyblKey.Name,
	}
	dplList := &dplv1.DeployableList{}
	g.Expect(c.List(context.TODO(), dplList, &client.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set(nameLabel))})).NotTo(HaveOccurred())
	g.Expect(dplList.Items).To(HaveLen(0))
}
