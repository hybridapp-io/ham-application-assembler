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

	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	t "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	manifestwork "github.com/open-cluster-management/api/work/v1"
)

func (r *ReconcileApplication) createApplicationAssembler(app *sigappv1beta1.Application) error {
	appasm := &toolsv1alpha1.ApplicationAssembler{}

	appasm.SetGroupVersionKind(applicationAssemblerGVK)
	appasm.Name = app.GetName()
	appasm.Namespace = app.GetNamespace()
	appasm.Spec.HubComponents = make([]*corev1.ObjectReference, 0)
	appasm.Spec.ManagedClustersComponents = make([]*toolsv1alpha1.ClusterComponent, 0)

	// add the matching manifestworks
	resources, err := r.fetchApplicationComponents(app)
	if err != nil {
		return err
	}
	if err = r.buildAssemblerComponents(appasm, resources); err != nil {
		klog.Error("Failed to build application assembler components for application ", app.Namespace+"/"+app.Name)
		return err
	}

	return r.Create(context.TODO(), appasm)
}

func (r *ReconcileApplication) buildAssemblerComponents(appasm *toolsv1alpha1.ApplicationAssembler, resources []*unstructured.Unstructured) error {

	var mcComponents map[string][]*corev1.ObjectReference

	for _, resource := range resources {

		or := &corev1.ObjectReference{

			Kind:       resource.GetKind(),
			Name:       resource.GetName(),
			Namespace:  resource.GetNamespace(),
			APIVersion: resource.GetAPIVersion(),
		}
		if or.APIVersion == manifestwork.SchemeGroupVersion.String() && or.Kind == toolsv1alpha1.ManifestworkGVK.Kind {
			clusterName := resource.GetNamespace()
			// retrieve the cluster
			clusterKey := t.NamespacedName{
				Name: clusterName,
			}
			cluster := &managedclusterv1.ManagedCluster{}
			err := r.Get(context.TODO(), clusterKey, cluster)
			if err != nil {
				if errors.IsNotFound(err) {
					klog.Error("Managed cluster with name ", clusterName, " does not exist. ")
					continue
				} else {
					klog.Error("Failed to retrieve the managed cluster with name ", clusterName, " with error: ", err)
					return err
				}
			}
			if mcComponents == nil {
				mcComponents = make(map[string][]*corev1.ObjectReference)
			}
			if _, ok := mcComponents[clusterName]; !ok {
				comps := []*corev1.ObjectReference{or}
				mcComponents[clusterName] = comps

			} else {
				mcComponents[clusterName] = append(mcComponents[clusterName], or)
			}
		} else {
			if appasm.Spec.HubComponents == nil {
				appasm.Spec.HubComponents = make([]*corev1.ObjectReference, 0)
			}
			appasm.Spec.HubComponents = append(appasm.Spec.HubComponents, or)
		}
	}

	for cluster, components := range mcComponents {
		appasm.Spec.ManagedClustersComponents = append(appasm.Spec.ManagedClustersComponents, &toolsv1alpha1.ClusterComponent{
			Cluster:    cluster,
			Components: components,
		})
	}
	return nil
}
