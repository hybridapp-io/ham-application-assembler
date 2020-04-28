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

	"github.com/hybridapp-io/ham-application-assembler/pkg/utils"
	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
)

const (
	packageInfoLogLevel = 3
)

var (
	applicationAssemblerGVK = schema.GroupVersionKind{
		Group:   toolsv1alpha1.SchemeGroupVersion.Group,
		Version: toolsv1alpha1.SchemeGroupVersion.Version,
		Kind:    "ApplicationAssembler",
	}

	applicationGVK = schema.GroupVersionKind{
		Group:   "app.k8s.io",
		Version: "v1beta1",
		Kind:    "Application",
	}
)

func (r *ReconcileApplication) isAppDiscoveryEnabled(app *sigappv1beta1.Application) bool {
	if _, enabled := app.GetAnnotations()[toolsv1alpha1.AnnotationDiscover]; !enabled ||
		app.GetAnnotations()[toolsv1alpha1.AnnotationDiscover] != toolsv1alpha1.DiscoveryEnabled {
		return false
	}

	return true
}

func (r *ReconcileApplication) isCreateAssemblerEnabled(app *sigappv1beta1.Application) bool {
	if _, enabled := app.GetAnnotations()[toolsv1alpha1.AnnotationCreateAssembler]; !enabled ||
		app.GetAnnotations()[toolsv1alpha1.AnnotationCreateAssembler] != toolsv1alpha1.DiscoveryEnabled {
		return false
	}

	return true
}

func (r *ReconcileApplication) generateName(name string) string {
	return name + "-"
}

func (r *ReconcileApplication) updateAnnotation(app *sigappv1beta1.Application, annotation string, newValue string) error {

	if _, ok := app.GetAnnotations()[annotation]; ok {
		app.Annotations[annotation] = newValue
		err := r.Update(context.TODO(), app)
		return err
	}
	return nil
}

func (r *ReconcileApplication) fetchApplicationComponents(app *sigappv1beta1.Application) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	for _, gk := range app.Spec.ComponentGroupKinds {
		mapping, err := r.restMapper.RESTMapping(schema.GroupKind{
			Group: utils.StripVersion(gk.Group),
			Kind:  gk.Kind,
		})
		if err != nil {
			klog.Error("No mapping found for GK ", gk)
			return nil, err
		}

		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(mapping.GroupVersionKind)

		if list, err = r.dynamicClient.Resource(mapping.Resource).List(metav1.ListOptions{
			LabelSelector: labels.Set(app.Spec.Selector.MatchLabels).String(),
		}); err != nil {
			klog.Error("Failed to retrieve the list of resources for GK ", gk)
			return nil, err
		}

		for _, u := range list.Items {
			resource := u
			resources = append(resources, &resource)
		}

		// remote components wrapped by deployables if discovery annotation is enabled
		if r.isAppDiscoveryEnabled(app) {
			dplList := &dplv1.DeployableList{}
			err = r.List(context.TODO(), dplList, &client.ListOptions{LabelSelector: labels.Set(app.Spec.Selector.MatchLabels).AsSelector()})
			if err != nil {
				klog.Error("Failed to retrieve the list of deployables for GK ", gk)
				return nil, err
			}
			for _, dpl := range dplList.Items {
				dplTemplate := &unstructured.Unstructured{}
				err = json.Unmarshal(dpl.Spec.Template.Raw, dplTemplate)
				if err != nil {
					klog.Info("Failed to unmarshal object with error", err)
					return nil, err
				}

				if dplTemplate.GetKind() == gk.Kind && dplTemplate.GetAPIVersion() == utils.GetAPIVersion(mapping) {
					uc, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&dpl)
					ucObj := &unstructured.Unstructured{}
					ucObj.SetUnstructuredContent(uc)
					resources = append(resources, ucObj)
				}
			}
		}
	}
	return resources, nil
}

func (r *ReconcileApplication) buildObjectStatuses(resources []*unstructured.Unstructured) []sigappv1beta1.ObjectStatus {
	var objectStatuses []sigappv1beta1.ObjectStatus
	for _, resource := range resources {
		os := sigappv1beta1.ObjectStatus{
			Group: resource.GroupVersionKind().Group,
			Kind:  resource.GetKind(),
			Name:  resource.GetName(),
			Link:  resource.GetSelfLink(),
		}
		objectStatuses = append(objectStatuses, os)
	}
	return objectStatuses
}

func (r *ReconcileApplication) updateApplicationStatus(app *sigappv1beta1.Application) error {
	resources, err := r.fetchApplicationComponents(app)
	if err != nil {
		return err
	}

	objectStatuses := r.buildObjectStatuses(resources)
	newAppStatus := app.Status.DeepCopy()
	newAppStatus.ComponentList = sigappv1beta1.ComponentList{
		Objects: objectStatuses,
	}
	newAppStatus.ObservedGeneration = app.Generation

	// equality.Semantic.DeepEqual does not work well for arrays
	if r.objectsDeepEquals(newAppStatus.ComponentList.Objects, app.Status.ComponentList.Objects) {
		return nil
	}

	app.Status = *newAppStatus

	// update the app status
	err = r.Status().Update(context.TODO(), app)

	return err
}

func (r *ReconcileApplication) objectsDeepEquals(oldStatus []sigappv1beta1.ObjectStatus, newStatus []sigappv1beta1.ObjectStatus) bool {
	var matchedNew = 0

	var matchedOld = 0

	for _, newStatus := range newStatus {
		for _, oldStatus := range oldStatus {
			if oldStatus.Name == newStatus.Name &&
				oldStatus.Kind == newStatus.Kind &&
				oldStatus.Group == newStatus.Group &&
				oldStatus.Link == newStatus.Link {
				matchedNew++
				break
			}
		}
	}

	if matchedNew == len(newStatus) {
		for _, oldStatus := range oldStatus {
			for _, newStatus := range newStatus {
				if oldStatus.Name == newStatus.Name &&
					oldStatus.Kind == newStatus.Kind &&
					oldStatus.Group == newStatus.Group &&
					oldStatus.Link == newStatus.Link {
					matchedOld++
					break
				}
			}
		}

		return matchedOld == len(oldStatus)
	}
	return false
}
