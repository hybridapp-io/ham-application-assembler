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
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	prulev1alpha1 "github.com/hybridapp-io/ham-placement/pkg/apis/core/v1alpha1"
	workapiv1 "github.com/open-cluster-management/api/work/v1"
)

var (
	timeout = time.Second * 2

	payload = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payload",
			Namespace: "payload",
		},
	}

	mcName = "mc"
	mc     = &managedclusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: mcName,
		},
	}
	mcNS = corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: mcName,
		},
	}
	placementRuleName = "foo-app-foo-deployable"
	deployableKey     = types.NamespacedName{
		Name:      "foo-deployable",
		Namespace: mcName,
	}

	manifestwork = &workapiv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployableKey.Name,
			Namespace: deployableKey.Namespace,
		},
		Spec: workapiv1.ManifestWorkSpec{
			Workload: workapiv1.ManifestsTemplate{
				Manifests: []workapiv1.Manifest{
					{
						runtime.RawExtension{
							Object: payload,
						},
					},
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
			ManagedClustersComponents: []*toolsv1alpha1.ClusterComponent{
				{
					Cluster: manifestwork.Namespace,
					Components: []*corev1.ObjectReference{
						{
							APIVersion: "work.open-cluster-management.io/v1",
							Kind:       "ManifestWork",
							Name:       manifestwork.Name,
							Namespace:  manifestwork.Namespace,
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

	ns := mcNS.DeepCopy()
	g.Expect(c.Create(context.TODO(), ns)).To(Succeed())

	cluster := mc.DeepCopy()
	g.Expect(c.Create(context.TODO(), cluster)).To(Succeed())

	// create the manifestwork object
	mwork := manifestwork.DeepCopy()
	g.Expect(c.Create(context.TODO(), mwork)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), mwork); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the ApplicationAssembler object and expect the Reconcile and HybridDeployable to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer func() {
		// cleanup hdpl
		dplList := &hdplv1alpha1.DeployableList{}
		g.Expect(c.List(context.TODO(), dplList)).NotTo(HaveOccurred())
		for _, hdpl := range dplList.Items {
			hdpl := hdpl
			g.Expect(c.Delete(context.TODO(), &hdpl)).NotTo(HaveOccurred())
		}
		// cleanup hpr
		hprList := &prulev1alpha1.PlacementRuleList{}
		g.Expect(c.List(context.TODO(), hprList)).NotTo(HaveOccurred())
		for _, hpr := range hprList.Items {
			hpr := hpr
			g.Expect(c.Delete(context.TODO(), &hpr)).NotTo(HaveOccurred())
		}
		// cleanup the appasm
		g.Expect(c.Delete(context.TODO(), instance)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
}

func TestReconcile_WithDeployable_ApplicationAndHybridDeployableAndPlacementRuleCreated(t *testing.T) {
	g := NewWithT(t)

	var c client.Client

	var expectedRequest = reconcile.Request{NamespacedName: applicationAssemblerKey}
	managedCluster := corev1.ObjectReference{
		Name:       mcName,
		APIVersion: "cluster.open-cluster-management.io/v1",
	}
	placementRuleNamespace := applicationAssemblerKey.Namespace

	placementRule := &prulev1alpha1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placementRuleName,
			Namespace: placementRuleNamespace,
		},
		Spec: prulev1alpha1.PlacementRuleSpec{
			Targets: []corev1.ObjectReference{
				managedCluster,
			},
		},
	}
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
	defer func() {
		if err = c.Delete(context.Background(), prule); err != nil {
			klog.Error(err)
		}
	}()

	manwork := manifestwork.DeepCopy()
	g.Expect(c.Create(context.TODO(), manwork)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.Background(), manwork); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer func() {
		// cleanup hdpl
		dplList := &hdplv1alpha1.DeployableList{}
		g.Expect(c.List(context.TODO(), dplList)).NotTo(HaveOccurred())
		for _, hdpl := range dplList.Items {
			hdpl := hdpl
			g.Expect(c.Delete(context.TODO(), &hdpl)).NotTo(HaveOccurred())
		}
		// cleanup hpr
		hprList := &prulev1alpha1.PlacementRuleList{}
		g.Expect(c.List(context.TODO(), hprList)).NotTo(HaveOccurred())
		for _, hpr := range hprList.Items {
			hpr := hpr
			g.Expect(c.Delete(context.TODO(), &hpr)).NotTo(HaveOccurred())
		}
		// cleanup the appasm
		g.Expect(c.Delete(context.TODO(), instance)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
	appKey := types.NamespacedName{Name: applicationAssemblerKey.Name, Namespace: applicationAssemblerKey.Namespace}
	app := &sigappv1beta1.Application{}
	g.Expect(c.Get(context.TODO(), appKey, app)).NotTo(HaveOccurred())

	hybrddplyblKey := types.NamespacedName{Name: deployableKey.Namespace + "-configmap-" + payload.Namespace + "-" +
		payload.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())
}

func TestReconcile_WithDeployableAndPlacementRule_ApplicationAndHybridDeployableCreated(t *testing.T) {
	g := NewWithT(t)

	managedCluster := corev1.ObjectReference{
		Name:       mcName,
		APIVersion: "cluster.open-cluster-management.io/v1",
	}
	placementRuleNamespace := applicationAssemblerKey.Namespace

	placementRule := &prulev1alpha1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placementRuleName,
			Namespace: placementRuleNamespace,
		},
		Spec: prulev1alpha1.PlacementRuleSpec{
			Targets: []corev1.ObjectReference{
				managedCluster,
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
	defer func() {
		if err = c.Delete(context.Background(), prule); err != nil {
			klog.Error(err)
		}
	}()

	manwork := manifestwork.DeepCopy()
	g.Expect(c.Create(context.TODO(), manwork)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.Background(), manwork); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	instance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).NotTo(HaveOccurred())
	defer func() {
		// cleanup hdpl
		dplList := &hdplv1alpha1.DeployableList{}
		g.Expect(c.List(context.TODO(), dplList)).NotTo(HaveOccurred())
		for _, hdpl := range dplList.Items {
			hdpl := hdpl
			g.Expect(c.Delete(context.TODO(), &hdpl)).NotTo(HaveOccurred())
		}
		// cleanup hpr
		hprList := &prulev1alpha1.PlacementRuleList{}
		g.Expect(c.List(context.TODO(), hprList)).NotTo(HaveOccurred())
		for _, hpr := range hprList.Items {
			hpr := hpr
			g.Expect(c.Delete(context.TODO(), &hpr)).NotTo(HaveOccurred())
		}
		// cleanup the appasm
		g.Expect(c.Delete(context.TODO(), instance)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	appKey := types.NamespacedName{Name: applicationAssemblerKey.Name, Namespace: applicationAssemblerKey.Namespace}
	app := &sigappv1beta1.Application{}
	g.Expect(c.Get(context.TODO(), appKey, app)).NotTo(HaveOccurred())

	hybrddplyblKey := types.NamespacedName{Name: deployableKey.Namespace + "-configmap-" + payload.Namespace + "-" +
		payload.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())
}

func TestReconcile_WithHybridDeployableAndPlacementRule_ApplicationAndHybridDeployableCreated(t *testing.T) {
	g := NewWithT(t)

	clusterName := mcName
	placementRuleName := "foo-app-foo-deployable"
	placementRuleNamespace := applicationAssemblerKey.Namespace
	managedCluster := corev1.ObjectReference{
		Name:       mcName,
		APIVersion: "cluster.open-cluster-management.io/v1",
	}
	placementRule := &prulev1alpha1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placementRuleName,
			Namespace: placementRuleNamespace,
		},
		Spec: prulev1alpha1.PlacementRuleSpec{
			Targets: []corev1.ObjectReference{
				managedCluster,
			},
		},
	}

	deployerKey := types.NamespacedName{
		Name:      "foo-deployer",
		Namespace: mcName,
	}

	deployerType := "configmap"

	deployer := &prulev1alpha1.Deployer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployerKey.Name,
			Namespace: deployerKey.Namespace,
			Labels:    map[string]string{"deployer-type": deployerType},
		},
		Spec: prulev1alpha1.DeployerSpec{
			Type: deployerType,
		},
	}

	deployerSetKey := types.NamespacedName{
		Name:      clusterName,
		Namespace: "default",
	}

	deployerSet := &prulev1alpha1.DeployerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: deployerSetKey.Namespace,
		},
		Spec: prulev1alpha1.DeployerSetSpec{
			Deployers: []prulev1alpha1.DeployerSpecDescriptor{
				{
					Key: clusterName + "/" + deployer.GetName(),
					Spec: prulev1alpha1.DeployerSpec{
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
	defer func() {
		if err = c.Delete(context.Background(), prule); err != nil {
			klog.Error(err)
		}
	}()

	manwork := manifestwork.DeepCopy()
	g.Expect(c.Create(context.TODO(), manwork)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.Background(), manwork); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	// Create the ApplicationAssembler object and expect the Reconcile and Deployment to be created
	fooInstance := applicationAssembler.DeepCopy()
	g.Expect(c.Create(context.TODO(), fooInstance)).NotTo(HaveOccurred())
	defer func() {
		// cleanup the appasm
		g.Expect(c.Delete(context.TODO(), fooInstance)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	hybrddplyblKey := types.NamespacedName{Name: deployableKey.Namespace + "-configmap-" + payload.Namespace + "-" +
		payload.Name, Namespace: applicationAssemblerKey.Namespace}
	hybrddplybl := &hdplv1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), hybrddplyblKey, hybrddplybl)).NotTo(HaveOccurred())

	dplyr := deployer.DeepCopy()
	g.Expect(c.Create(context.TODO(), dplyr)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dplyr); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

	dset := deployerSet.DeepCopy()
	g.Expect(c.Create(context.TODO(), dset)).NotTo(HaveOccurred())
	defer func() {
		if err = c.Delete(context.TODO(), dset); err != nil {
			klog.Error(err)
			t.Fail()
		}
	}()

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
	defer func() {
		// cleanup hdpl
		dplList := &hdplv1alpha1.DeployableList{}
		g.Expect(c.List(context.TODO(), dplList)).NotTo(HaveOccurred())
		for _, hdpl := range dplList.Items {
			hdpl := hdpl
			g.Expect(c.Delete(context.TODO(), &hdpl)).NotTo(HaveOccurred())
		}
		// cleanup hpr
		hprList := &prulev1alpha1.PlacementRuleList{}
		g.Expect(c.List(context.TODO(), hprList)).NotTo(HaveOccurred())
		for _, hpr := range hprList.Items {
			hpr := hpr
			g.Expect(c.Delete(context.TODO(), &hpr)).NotTo(HaveOccurred())
		}
		// cleanup the appasm
		g.Expect(c.Delete(context.TODO(), barInstance)).NotTo(HaveOccurred())
	}()
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	barAppKey := types.NamespacedName{Name: barApplicationAssemblerKey.Name, Namespace: barApplicationAssemblerKey.Namespace}
	barApp := &sigappv1beta1.Application{}
	g.Expect(c.Get(context.TODO(), barAppKey, barApp)).NotTo(HaveOccurred())
}
