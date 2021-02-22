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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	prulev1alpha1 "github.com/hybridapp-io/ham-placement/pkg/apis/core/v1alpha1"
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

type Relationship struct {
	Label            string `json:"label"`
	Source           string `json:"source"`
	SourceCluster    string `json:"sourceCluster"`
	SourceNamespace  string `json:"sourceNamespace"`
	SourceApiGroup   string `json:"sourceApiGroup"`
	SourceApiVersion string `json:"sourceApiVersion"`
	SourceKind       string `json:"sourceKind"`
	SourceName       string `json:"sourceName"`
	Dest             string `json:"dest"`
	DestCluster      string `json:"destCluster,omitempty"`
	DestNamespace    string `json:"destNamespace,omitempty"`
	DestApiGroup     string `json:"destApiGroup,omitempty"`
	DestApiVersion   string `json:"destApiVersion,omitempty"`
	DestKind         string `json:"destKind,omitempty"`
	DestName         string `json:"destName,omitempty"`
	DestUID          string `json:"destUID,omitempty"`
}

func (r *ReconcileApplication) isAppDiscoveryEnabled(app *sigappv1beta1.Application) bool {
	if _, enabled := app.GetAnnotations()[hdplv1alpha1.AnnotationHybridDiscovery]; !enabled ||
		app.GetAnnotations()[hdplv1alpha1.AnnotationHybridDiscovery] != hdplv1alpha1.HybridDiscoveryEnabled {
		return false
	}

	return true
}

func (r *ReconcileApplication) isCreateAssemblerEnabled(app *sigappv1beta1.Application) bool {
	if _, enabled := app.GetAnnotations()[toolsv1alpha1.AnnotationCreateAssembler]; !enabled ||
		app.GetAnnotations()[toolsv1alpha1.AnnotationCreateAssembler] != toolsv1alpha1.HybridDiscoveryCreateAssembler {
		return false
	}

	return true
}

func (r *ReconcileApplication) generateName(name string) string {
	return name + "-"
}

func (r *ReconcileApplication) updateDiscoveryAnnotations(app *sigappv1beta1.Application) error {

	if _, ok := app.GetAnnotations()[hdplv1alpha1.AnnotationHybridDiscovery]; ok {
		app.Annotations[hdplv1alpha1.AnnotationHybridDiscovery] = hdplv1alpha1.HybridDiscoveryCompleted
	}
	if _, ok := app.GetAnnotations()[toolsv1alpha1.AnnotationCreateAssembler]; ok {
		app.Annotations[toolsv1alpha1.AnnotationCreateAssembler] = toolsv1alpha1.AssemblerCreationCompleted
	}
	err := r.Update(context.TODO(), app)
	return err
}

func (r *ReconcileApplication) fetchApplicationComponents(app *sigappv1beta1.Application) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	for _, gk := range app.Spec.ComponentGroupKinds {
		// local components
		mapping, err := r.restMapper.RESTMapping(schema.GroupKind{
			Group: utils.StripVersion(gk.Group),
			Kind:  gk.Kind,
		})
		if err != nil {
			klog.Info("No mapping found for GK ", gk)
		} else {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(mapping.GroupVersionKind)
			// if selector is not provided, no components will be fetched
			if app.Spec.Selector != nil {
				if app.Spec.Selector.MatchLabels != nil {
					if list, err = r.dynamicClient.Resource(mapping.Resource).Namespace(app.Namespace).List(context.TODO(), metav1.ListOptions{
						LabelSelector: labels.Set(app.Spec.Selector.MatchLabels).String(),
					}); err != nil {
						klog.Error("Failed to retrieve the list of resources for GK ", gk)
						return nil, err
					}
				}
			}

			for _, u := range list.Items {
				resource := u
				// ignore the resource if it belongs to another hub, this helps the all-in-one poc scenario
				ra := resource.GetAnnotations()
				if ra != nil {
					if _, ok := ra[dplv1.AnnotationHosting]; ok {
						continue
					}
				}

				resources = append(resources, &resource)
			}
		}

		// remote components wrapped by deployables if discovery annotation is enabled
		if r.isAppDiscoveryEnabled(app) {
			dplList := &dplv1.DeployableList{}

			// if selector is not provided, no components will be fetched
			if app.Spec.Selector != nil {
				if app.Spec.Selector.MatchLabels != nil {
					err = r.List(context.TODO(), dplList, &client.ListOptions{LabelSelector: labels.Set(app.Spec.Selector.MatchLabels).AsSelector()})
					if err != nil {
						klog.Error("Failed to retrieve the list of deployables for GK ", gk)
						return nil, err
					}
				}
			}
			for i := range dplList.Items {
				dpl := dplList.Items[i]
				dplTemplate := &unstructured.Unstructured{}
				err = json.Unmarshal(dpl.Spec.Template.Raw, dplTemplate)
				if err != nil {
					klog.Info("Failed to unmarshal object with error", err)
					return nil, err
				}
				if (mapping != nil && dplTemplate.GetKind() == gk.Kind && dplTemplate.GetAPIVersion() == utils.GetAPIVersion(mapping)) ||
					(mapping == nil && dplTemplate.GetKind() == gk.Kind && utils.StripVersion(dplTemplate.GetAPIVersion()) == gk.Group) {

					ucMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&dpl)
					ucObj := &unstructured.Unstructured{}
					ucObj.SetUnstructuredContent(ucMap)
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
	if err != nil {
		return err
	}

	// Build configmap of resources related to application if it has hybrid
	// deployables
	hasHdpl := false
	// TODO: is this check sufficient?
	// if app.Spec.ComponentGroupKinds[0].Group == "core.hybridapp.io" && app.Spec.ComponentGroupKinds[0].Kind == "Deployable"
	for _, res := range resources {
		if res.GroupVersionKind().Group == "core.hybridapp.io" && res.GroupVersionKind().Kind == "Deployable" {
			hasHdpl = true
			break
		}
	}
	if hasHdpl {
		err = r.updateAppRelationships(app, resources)
	}

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

// updateAppRelationships updates the configmap resources related to the Hybrid
// App
func (r *ReconcileApplication) updateAppRelationships(app *sigappv1beta1.Application, resources []*unstructured.Unstructured) error {
	// build the new configmap
	relationshipsConfigmap, err := r.buildRelationshipsConfigmap(app, resources)

	// Update existing configmap or else create new
	// Configmap will have same name and namespace as associated application
	configmapKey := types.NamespacedName{
		Name:      app.GetName(),
		Namespace: app.GetNamespace(),
	}
	err = r.Get(context.TODO(), configmapKey, &corev1.ConfigMap{})
	// Create the configmap if not existing
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Create(context.TODO(), relationshipsConfigmap)

			return err
		}
		return err
	}

	// Update existing configmap
	err = r.Update(context.TODO(), relationshipsConfigmap)

	return err
}

// buildRelationshipsConfigmap builds a configmap of resources related to the
// app
func (r *ReconcileApplication) buildRelationshipsConfigmap(app *sigappv1beta1.Application, resources []*unstructured.Unstructured) (*corev1.ConfigMap, error) {

	relationships := []Relationship{}
	for _, resource := range resources {
		// Check if resource exists and add to relationships
		hdplKey := types.NamespacedName{
			Name:      resource.GetName(),
			Namespace: resource.GetNamespace(),
		}
		hdpl := hdplv1alpha1.Deployable{}
		err := r.Get(context.TODO(), hdplKey, &hdpl)
		if err != nil {
			continue
		}
		relationships = append(relationships, Relationship{
			Label:            "uses",
			Source:           "k8s",
			SourceCluster:    "local-cluster",
			SourceNamespace:  app.GetNamespace(),
			SourceApiGroup:   app.GroupVersionKind().Group,
			SourceApiVersion: app.GroupVersionKind().Version,
			SourceKind:       "application",
			SourceName:       app.GetName(),
			Dest:             "k8s",
			DestCluster:      "local-cluster",
			DestNamespace:    resource.GetNamespace(),
			DestApiGroup:     resource.GroupVersionKind().Group,
			DestApiVersion:   resource.GroupVersionKind().Version,
			DestKind:         resource.GroupVersionKind().Kind,
			DestName:         resource.GetName(),
		})

		// recursively find relationships of each resource
		relationships = r.addHdplRelationships(&hdpl, relationships)

	}

	// Convert into json and then into string map to store in configmap data
	relationshipsByteArray, err := json.Marshal(relationships)
	if err != nil {
		klog.Info("Failed to marshal object with error", err)
		return nil, err
	}
	relationshipsMap := map[string]string{"relationships": string(relationshipsByteArray)}
	relationshipsConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Data: relationshipsMap,
	}

	return relationshipsConfigmap, nil
}

// addHdplRelationships adds resources that are related to a Hybrid Deployable
// to the relationships configmap for the Hybrid App. Resources searched for
// include Hybrid PlacementRule, Deployable, and VirtualMachine
func (r *ReconcileApplication) addHdplRelationships(hdpl *hdplv1alpha1.Deployable, relationships []Relationship) []Relationship {
	// Get related Hybrid PlacementRules
	hprRef := hdpl.Spec.Placement.PlacementRef
	hprKey := types.NamespacedName{
		Name:      hprRef.Name,
		Namespace: hprRef.Namespace,
	}
	hpr := prulev1alpha1.PlacementRule{}
	err := r.Get(context.TODO(), hprKey, &hpr)
	// if PlacementRule missing, don't look for remaining resources
	if err != nil {
		return relationships
	}
	relationships = append(relationships, Relationship{
		Label:            "uses",
		Source:           "k8s",
		SourceCluster:    "local-cluster",
		SourceNamespace:  hdpl.GetNamespace(),
		SourceApiGroup:   hdpl.GroupVersionKind().Group,
		SourceApiVersion: hdpl.GroupVersionKind().Version,
		SourceKind:       hdpl.GroupVersionKind().Kind,
		SourceName:       hdpl.GetName(),
		Dest:             "k8s",
		DestCluster:      "local-cluster",
		DestNamespace:    hpr.GetNamespace(),
		DestApiGroup:     "core.hybridapp.io",
		DestApiVersion:   "v1alpha1",
		DestKind:         "PlacementRule",
		DestName:         hpr.GetName(),
	})

	for _, decision := range hpr.Status.Decisions {
		if decision.Kind == "Cluster" {
			// Get related Deployables
			dplList := &dplv1.DeployableList{}
			err = r.List(context.TODO(), dplList, &client.ListOptions{Namespace: decision.Namespace})
			if err == nil {
				for _, dpl := range dplList.Items {
					if dpl.Annotations[hdplv1alpha1.HostingHybridDeployable] == hdpl.Namespace+"/"+hdpl.Name {
						relationships = append(relationships, Relationship{
							Label:            "uses",
							Source:           "k8s",
							SourceCluster:    "local-cluster",
							SourceNamespace:  hdpl.GetNamespace(),
							SourceApiGroup:   hdpl.GroupVersionKind().Group,
							SourceApiVersion: hdpl.GroupVersionKind().Version,
							SourceKind:       hdpl.GroupVersionKind().Kind,
							SourceName:       hdpl.GetName(),
							Dest:             "k8s",
							DestCluster:      "local-cluster",
							DestNamespace:    dpl.GetNamespace(),
							DestApiGroup:     dpl.GroupVersionKind().Group,
							DestApiVersion:   dpl.GroupVersionKind().Version,
							DestKind:         dpl.GroupVersionKind().Kind,
							DestName:         dpl.GetName(),
						})
					}
				}
			}
		} else if decision.Kind == "Deployer" {
			// Get VirtualMachines
			// Look for GVKGVR mapping to determine whether VirtualMachine cdr exists
			gvr, ok := utils.GVKGVRMap[schema.GroupVersionKind{
				Group:   "infra.management.ibm.com",
				Version: "v1alpha1",
				Kind:    "VirtualMachine",
			}]
			if ok {
				vmList, err := r.dynamicClient.Resource(gvr).Namespace(hdpl.GetNamespace()).List(context.TODO(), metav1.ListOptions{})
				if err == nil {
					for _, vm := range vmList.Items {
						if vm.GetAnnotations()[hdplv1alpha1.HostingHybridDeployable] == hdpl.Namespace+"/"+hdpl.Name {
							relationships = append(relationships, Relationship{
								Label:            "uses",
								Source:           "k8s",
								SourceCluster:    "local-cluster",
								SourceNamespace:  hdpl.GetNamespace(),
								SourceApiGroup:   hdpl.GroupVersionKind().Group,
								SourceApiVersion: hdpl.GroupVersionKind().Version,
								SourceKind:       hdpl.GroupVersionKind().Kind,
								SourceName:       hdpl.GetName(),
								Dest:             "im",
								DestUID:          string(vm.GetUID()),
							})
						}
					}
				}
			}
		}
	}

	return relationships
}
