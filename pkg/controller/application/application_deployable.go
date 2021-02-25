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

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
)

// Locates a deployable wrapping an application in a managed cluster namespace
func (r *ReconcileApplication) locateAppDeployable(appKey types.NamespacedName, namespace string) (*dplv1.Deployable, error) {
	dpllist := &dplv1.DeployableList{}

	err := r.List(context.TODO(), dpllist, client.InNamespace(namespace))
	if err != nil {
		klog.Error("Failed to retrieve the list of deployables with error:", err)
		return nil, err
	}

	for _, dpl := range dpllist.Items {
		// source annotation is not powerful enough, we need to get deployables with a specific template type
		templateobj := &unstructured.Unstructured{}
		err = json.Unmarshal(dpl.Spec.Template.Raw, templateobj)
		if err != nil {
			klog.Info("Failed to unmarshal object with error", err)
			return nil, err
		}

		if templateobj.GetKind() != applicationGVK.Kind {
			continue
		}

		annotations := dpl.GetAnnotations()
		if srcobj, ok := annotations[hdplv1alpha1.SourceObject]; ok {
			if srcobj == appKey.String() {
				return dpl.DeepCopy(), nil
			}
		}
	}

	return nil, nil
}

// This function will reconcile the app deployable in all managed namespaces .
// Each managed cluster namespace will have its own app deployable which will be in charge
// of discovering the app components in that respective managed cluster
func (r *ReconcileApplication) reconcileAppDeployables(app *sigappv1beta1.Application) error {
	if app.Annotations[toolsv1alpha1.AnnotationDiscoveryTarget] != "" {
		return r.reconcileAppDeployableOnTarget(app)
	}
	// default discover across all managed clusters
	return r.reconcileAppDeployableOnAllTargets(app)
}

func (r *ReconcileApplication) reconcileAppDeployableOnAllTargets(app *sigappv1beta1.Application) error {

	// retrieve a list of clusters
	clusterList := &managedclusterv1.ManagedClusterList{}
	err := r.List(context.TODO(), clusterList)
	if err != nil {
		klog.Error("Failed to retrieve the list of managed clusters ")
		return err
	}
	for _, cluster := range clusterList.Items {
		ignored := false
		for _, clObjRef := range toolsv1alpha1.ClustersIgnoredForDiscovery {
			if clObjRef.Name == cluster.Name {
				ignored = true
				break
			}
		}
		// process only clusters which are not in the ignored list
		if !ignored {
			err = r.reconcileAppDeployable(app, cluster.Name)
			if err != nil {
				klog.Error("Failed to reconcile the application deployable in managed cluster namespace: ", cluster.Name)
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileApplication) reconcileAppDeployableOnTarget(app *sigappv1beta1.Application) error {

	targetJSON := app.Annotations[toolsv1alpha1.AnnotationDiscoveryTarget]
	targetObjectReference := &corev1.ObjectReference{}

	if err := json.Unmarshal([]byte(targetJSON), targetObjectReference); err != nil {
		klog.Error("Unable to unmarshal the value of the annotation ", toolsv1alpha1.AnnotationDiscoveryTarget, " with error: ", err)
		return err
	}
	cluster := &managedclusterv1.ManagedCluster{}
	if (targetObjectReference.Kind != "" && targetObjectReference.Kind != cluster.Kind) ||
		(targetObjectReference.APIVersion != "" && targetObjectReference.APIVersion != cluster.APIVersion) {
		klog.Error("Unsupported target kind ", targetObjectReference.Kind, " and version ", targetObjectReference.APIVersion)
		return nil
	}
	cluster.Name = targetObjectReference.Name

	ignored := false
	for _, clObjRef := range toolsv1alpha1.ClustersIgnoredForDiscovery {
		if clObjRef.Name == cluster.Name {
			ignored = true
			break
		}
	}
	// process only clusters which are not in the ignored list
	if !ignored {
		err := r.reconcileAppDeployable(app, cluster.Name)
		if err != nil {
			klog.Error("Failed to reconcile the application deployable in managed cluster namespace: ", cluster.Name)
			return err
		}
	}

	return nil
}

func (r *ReconcileApplication) deleteApplicationDeployables(appKey types.NamespacedName) error {

	// retrieve a list of clusters
	clusterList := &managedclusterv1.ManagedClusterList{}

	err := r.List(context.TODO(), clusterList)
	if err != nil {
		klog.Error("Failed to retrieve the list of managed clusters ")
		return err
	}
	for _, cluster := range clusterList.Items {
		dpl, err := r.locateAppDeployable(appKey, cluster.Name)
		if err != nil {
			klog.Error("Failed to locate application deployable with error: ", err)
			return err
		}

		if dpl != nil {
			err = r.Delete(context.TODO(), dpl)
			if err != nil {
				klog.Error("Failed to delete application deployable ", dpl.Namespace+"/"+dpl.Name+" with error:", err)
			}
		}
	}

	return nil
}

func (r *ReconcileApplication) reconcileAppDeployable(app *sigappv1beta1.Application, namespace string) error {
	appKey := types.NamespacedName{
		Name:      app.Name,
		Namespace: app.Namespace,
	}
	dpl, err := r.locateAppDeployable(appKey, namespace)
	if err != nil {
		klog.Error("Failed to locate application deployable with error: ", err)
		return err
	}
	if dpl == nil {
		dpl = &dplv1.Deployable{}
		dpl.GenerateName = r.generateName(app.GetName())
		dpl.Namespace = namespace
	}

	tplApp := app.DeepCopy()
	r.prepareDeployable(dpl, tplApp)
	r.prepareTemplate(tplApp, app.Namespace)

	dpl.Spec.Template = &runtime.RawExtension{
		Object: tplApp,
	}

	if dpl.UID == "" {
		err = r.Create(context.TODO(), dpl)
	} else {
		err = r.Update(context.TODO(), dpl)
	}

	if err != nil {
		klog.Error("Failed to reconcile app deployable with error: ", err)
		return err
	}

	return nil
}

func (r *ReconcileApplication) prepareDeployable(deployable *dplv1.Deployable, app *sigappv1beta1.Application) {
	labels := deployable.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	for key, value := range app.GetLabels() {
		labels[key] = value
	}

	deployable.SetLabels(labels)

	annotations := deployable.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[hdplv1alpha1.SourceObject] = types.NamespacedName{Namespace: app.GetNamespace(), Name: app.GetName()}.String()
	deployable.SetAnnotations(annotations)
}

func (r *ReconcileApplication) prepareTemplate(app *sigappv1beta1.Application, namespace string) {
	var emptyuid types.UID
	app.SetUID(emptyuid)
	app.SetSelfLink("")
	app.SetResourceVersion("")
	app.SetGeneration(0)
	app.SetCreationTimestamp(metav1.Time{})
	app.SetNamespace(namespace)
	app.SetOwnerReferences(nil)
}
