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
	workapiv1 "github.com/open-cluster-management/api/work/v1"
)

// Locates a man/Manifestwork wrapping an application in a managed cluster namespace
func (r *ReconcileApplication) locateAppManifestWork(appKey types.NamespacedName, namespace string) (*workapiv1.ManifestWork, int, error) {

	mworklist := &workapiv1.ManifestWorkList{}
	err := r.List(context.TODO(), mworklist, client.InNamespace(namespace))
	if err != nil {
		klog.Error("Failed to retrieve the list of manifestworks with error:", err)
		return nil, 0, err
	}

	for _, mwork := range mworklist.Items {
		// source annotation is not powerful enough, we need to get manifestworks with a specific manifest type
		for i, manifest := range mwork.Spec.Workload.Manifests {
			templateobj := &unstructured.Unstructured{}
			err = json.Unmarshal(manifest.Raw, templateobj)
			if err != nil {
				klog.Info("Failed to unmarshal object with error", err)
				return nil, 0, err
			}

			if templateobj.GetKind() != applicationGVK.Kind {
				continue
			}

			annotations := mwork.GetAnnotations()
			if srcobj, ok := annotations[hdplv1alpha1.SourceObject]; ok {
				if srcobj == appKey.String() {
					return mwork.DeepCopy(), i, nil
				}
			}
		}
	}

	return nil, 0, nil
}

// This function will reconcile the app manifestwork in all managed namespaces .
// Each managed cluster namespace will have its own app manifestwork which will be in charge
// of discovering the app components in that respective managed cluster
func (r *ReconcileApplication) reconcileAppManifestWorks(app *sigappv1beta1.Application) error {
	if app.Annotations[toolsv1alpha1.AnnotationDiscoveryTarget] != "" {
		return r.reconcileAppManifestWorkOnTarget(app)
	}
	// default discover across all managed clusters
	return r.reconcileAppManifestWorkOnAllTargets(app)
}

func (r *ReconcileApplication) reconcileAppManifestWorkOnAllTargets(app *sigappv1beta1.Application) error {

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
			err = r.reconcileAppManifestWork(app, cluster.Name)
			if err != nil {
				klog.Error("Failed to reconcile the application manifestwork in managed cluster namespace: ", cluster.Name)
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileApplication) reconcileAppManifestWorkOnTarget(app *sigappv1beta1.Application) error {

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
		err := r.reconcileAppManifestWork(app, cluster.Name)
		if err != nil {
			klog.Error("Failed to reconcile the application manifestwork in managed cluster namespace: ", cluster.Name)
			return err
		}
	}

	return nil
}

func (r *ReconcileApplication) deleteApplicationManifestWorks(appKey types.NamespacedName) error {

	// retrieve a list of clusters
	clusterList := &managedclusterv1.ManagedClusterList{}

	err := r.List(context.TODO(), clusterList)
	if err != nil {
		klog.Error("Failed to retrieve the list of managed clusters ")
		return err
	}
	for _, cluster := range clusterList.Items {
		manifestWork, _, err := r.locateAppManifestWork(appKey, cluster.Name)
		if err != nil {
			klog.Error("Failed to locate application manifestwork with error: ", err)
			return err
		}

		if manifestWork != nil {
			err = r.Delete(context.TODO(), manifestWork)
			if err != nil {
				klog.Error("Failed to delete application manifestwork ", manifestWork.Namespace+"/"+manifestWork.Name+" with error:", err)
			}
		}
	}

	return nil
}

func (r *ReconcileApplication) reconcileAppManifestWork(app *sigappv1beta1.Application, namespace string) error {
	appKey := types.NamespacedName{
		Name:      app.Name,
		Namespace: app.Namespace,
	}
	manifestWork, manifestNum, err := r.locateAppManifestWork(appKey, namespace)
	if err != nil {
		klog.Error("Failed to locate application manifestwork with error: ", err)
		return err
	}
	if manifestWork == nil {
		manifestWork = &workapiv1.ManifestWork{}

		manifestWork.GenerateName = r.generateName(app.GetName())
		manifestWork.Namespace = namespace
	}

	tplApp := app.DeepCopy()
	r.prepareManifestWork(manifestWork, tplApp)
	r.prepareTemplate(tplApp, app.Namespace)

	appManifest := workapiv1.Manifest{
		runtime.RawExtension{
			Object: tplApp,
		},
	}

	if len(manifestWork.Spec.Workload.Manifests) == 0 {
		manifestWork.Spec.Workload.Manifests = []workapiv1.Manifest{
			appManifest,
		}
	} else {
		manifestWork.Spec.Workload.Manifests[manifestNum] = appManifest
	}

	if manifestWork.UID == "" {
		err = r.Create(context.TODO(), manifestWork)
	} else {
		err = r.Update(context.TODO(), manifestWork)
	}

	if err != nil {
		klog.Error("Failed to reconcile app manifestwork with error: ", err)
		return err
	}

	return nil
}

func (r *ReconcileApplication) prepareManifestWork(manifestWork *workapiv1.ManifestWork, app *sigappv1beta1.Application) {
	labels := manifestWork.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	for key, value := range app.GetLabels() {
		labels[key] = value
	}

	manifestWork.SetLabels(labels)

	annotations := manifestWork.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[hdplv1alpha1.SourceObject] = types.NamespacedName{Namespace: app.GetNamespace(), Name: app.GetName()}.String()
	manifestWork.SetAnnotations(annotations)
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
