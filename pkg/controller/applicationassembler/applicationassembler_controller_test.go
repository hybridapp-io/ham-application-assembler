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
	"time"

	. "github.com/onsi/gomega"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
	prulev1 "github.com/open-cluster-management/multicloud-operators-placementrule/pkg/apis/apps/v1"
)

var (
	timeout = time.Second * 2

	payload = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "payload",
		},
	}

	deployableKey = types.NamespacedName{
		Name:      "foo-deployable",
		Namespace: "foo-cluster",
	}

	deployable = &dplv1.Deployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployableKey.Name,
			Namespace: deployableKey.Namespace,
		},
		Spec: dplv1.DeployableSpec{
			Template: &runtime.RawExtension{
				Object: payload,
			},
		},
	}
)

var (
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
					Cluster: deployable.Namespace,
					Components: []*corev1.ObjectReference{
						{
							APIVersion: "apps.open-cluster-management.io/v1",
							Kind:       "Deployable",
							Name:       deployable.Name,
							Namespace:  deployable.Namespace,
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

	// create the deployable object
	dpl := deployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dpl)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), dpl)

	// Create the ApplicationAssembler object and expect the Reconcile and HybridDeployable to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
}

func TestReconcile_WithDeployable_ApplicationAndHybridDeployableAndPlacementRuleCreated(t *testing.T) {
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

	dplybl := deployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dplybl)).NotTo(HaveOccurred())
	defer c.Delete(context.Background(), dplybl)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	appKey := types.NamespacedName{Name: applicationAssemblerKey.Name, Namespace: applicationAssemblerKey.Namespace}
	app := &sigappv1beta1.Application{}
	g.Expect(c.Get(context.TODO(), appKey, app)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), app)

	hybrddplyblKey := types.NamespacedName{Name: "configmap-" + "-" + payload.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), hybrddplybl)

	pruleKey := types.NamespacedName{Name: "configmap-" + "-" + payload.Name, Namespace: applicationAssemblerKey.Namespace}
	prule := &prulev1.PlacementRule{}
	g.Expect(c.Get(context.TODO(), pruleKey, prule)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), prule)
}

func TestReconcile_WithDeployableAndPlacementRule_ApplicationAndHybridDeployableCreated(t *testing.T) {
	g := NewWithT(t)

	clusterName := "bar-cluster"
	placementRuleName := "foo-app-foo-deployable"
	placementRuleNamespace := applicationAssemblerKey.Namespace

	placementRule := &prulev1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placementRuleName,
			Namespace: placementRuleNamespace,
		},
		Spec: prulev1.PlacementRuleSpec{
			GenericPlacementFields: prulev1.GenericPlacementFields{
				ClusterSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": clusterName},
				},
			},
		},
	}

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

	prule := placementRule.DeepCopy()
	g.Expect(c.Create(context.TODO(), prule)).NotTo(HaveOccurred())
	defer c.Delete(context.Background(), prule)

	dplybl := deployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dplybl)).NotTo(HaveOccurred())
	defer c.Delete(context.Background(), dplybl)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	appKey := types.NamespacedName{Name: applicationAssemblerKey.Name, Namespace: applicationAssemblerKey.Namespace}
	app := &sigappv1beta1.Application{}
	g.Expect(c.Get(context.TODO(), appKey, app)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), app)

	hybrddplyblKey := types.NamespacedName{Name: "configmap-" + "-" + payload.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())
}

func TestReconcile_WithHybridDeployableAndPlacementRule_ApplicationAndHybridDeployableCreated(t *testing.T) {
	g := NewWithT(t)

	clusterName := "foo-cluster"
	placementRuleName := "foo-app-foo-deployable"
	placementRuleNamespace := applicationAssemblerKey.Namespace

	placementRule := &prulev1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placementRuleName,
			Namespace: placementRuleNamespace,
		},
		Spec: prulev1.PlacementRuleSpec{
			GenericPlacementFields: prulev1.GenericPlacementFields{
				ClusterSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": clusterName},
				},
			},
		},
	}

	deployerKey := types.NamespacedName{
		Name:      "foo-deployer",
		Namespace: "foo-cluster",
	}

	deployerType := "configmap"

	deployer := &hdplv1alpha1.Deployer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployerKey.Name,
			Namespace: deployerKey.Namespace,
			Labels:    map[string]string{"deployer-type": deployerType},
		},
		Spec: hdplv1alpha1.DeployerSpec{
			Type: deployerType,
		},
	}

	deployerSetKey := types.NamespacedName{
		Name:      clusterName,
		Namespace: "default",
	}

	deployerSet := &hdplv1alpha1.DeployerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: deployerSetKey.Namespace,
		},
		Spec: hdplv1alpha1.DeployerSetSpec{
			Deployers: []hdplv1alpha1.DeployerSpecDescriptor{
				{
					Key: clusterName + "/" + deployer.GetName(),
					Spec: hdplv1alpha1.DeployerSpec{
						Type: deployerType,
					},
				},
			},
		},
	}

	barApplicationAssemblerKey := types.NamespacedName{
		Name:      "bar-app",
		Namespace: "default",
	}

	barApplicationAssembler := &toolsv1alpha1.ApplicationAssembler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      barApplicationAssemblerKey.Name,
			Namespace: barApplicationAssemblerKey.Namespace,
		},
		Spec: toolsv1alpha1.ApplicationAssemblerSpec{},
	}

	var c client.Client

	expectedRequest := reconcile.Request{NamespacedName: applicationAssemblerKey}

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

	prule := placementRule.DeepCopy()
	g.Expect(c.Create(context.TODO(), prule)).NotTo(HaveOccurred())
	defer c.Delete(context.Background(), prule)

	dplybl := deployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), dplybl)).NotTo(HaveOccurred())
	defer c.Delete(context.Background(), dplybl)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	fooInstance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), fooInstance)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), fooInstance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	hybrddplyblKey := types.NamespacedName{Name: "configmap-" + "-" + payload.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())

	dplyr := deployer.DeepCopy()
	g.Expect(c.Create(context.TODO(), dplyr)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), dplyr)

	dset := deployerSet.DeepCopy()
	g.Expect(c.Create(context.TODO(), dset)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), dset)

	barApplicationAssembler.Spec = toolsv1alpha1.ApplicationAssemblerSpec{
		HubComponents: []*corev1.ObjectReference{
			{
				APIVersion: "core.hybridapp.io/v1alpha1",
				Kind:       "Deployable",
				Name:       hybrddplybl.GetName(),
				Namespace:  hybrddplybl.GetNamespace(),
			},
		},
	}

	expectedRequest = reconcile.Request{NamespacedName: barApplicationAssemblerKey}

	barInstance := barApplicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), barInstance)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), barInstance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	barAppKey := types.NamespacedName{Name: barApplicationAssemblerKey.Name, Namespace: barApplicationAssemblerKey.Namespace}
	barApp := &sigappv1beta1.Application{}
	g.Expect(c.Get(context.TODO(), barAppKey, barApp)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), barApp)
}

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

	hybrddplyblKey := types.NamespacedName{Name: "service-" + mcService.Namespace + "-" + mcService.Name, Namespace: applicationAssemblerKey.Namespace}

	nameLabel := map[string]string{
		hdplv1alpha1.HostingHybridDeployable: hybrddplyblKey.Name,
	}
	dplList := &dplv1.DeployableList{}
	g.Expect(c.List(context.TODO(), dplList, &client.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set(nameLabel))})).NotTo(HaveOccurred())
	g.Expect(dplList.Items).To(HaveLen(1))
}

func TestHubComponents(t *testing.T) {
	g := NewWithT(t)

	var (
		serviceName = "mysql-svc-mc"

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

	svc := service.DeepCopy()
	g.Expect(c.Create(context.TODO(), svc)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), svc)

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	hybrddplyblKey := types.NamespacedName{Name: "service-" + service.Namespace + "-" + service.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())
}
