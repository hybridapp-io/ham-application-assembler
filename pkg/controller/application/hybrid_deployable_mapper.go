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

	hdplv1alpha1 "github.com/IBM/hybriddeployable-operator/pkg/apis/app/v1alpha1"

	"github.com/hybridapp-io/ham-application-assembler/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type hybridDeployableMapper struct {
	client.Client
}

func (h *hybridDeployableMapper) Map(obj handler.MapObject) []reconcile.Request {
	var requests []reconcile.Request

	hdpl, err := h.convertObjectToHybridDeployable(&obj.Meta)
	if err != nil {
		klog.Error("Cannot convert object into deployable ", obj.Meta.GetName())
		return requests
	}

	app, err := h.locateAppForHybridDeployable(hdpl, hdpl.Namespace)
	if err != nil {
		klog.Error("Cannot retrieve the application for hybrid deployable ", obj.Meta.GetName()+" with error: ", err)
		return requests
	}

	if app != nil {
		// reconcile the app
		appKey := types.NamespacedName{
			Name:      app.Name,
			Namespace: app.Namespace,
		}

		requests = append(requests, reconcile.Request{NamespacedName: appKey})
	}

	return requests
}

func (h *hybridDeployableMapper) locateAppForHybridDeployable(hdpl *hdplv1alpha1.HybridDeployable, namespace string) (*sigappv1beta1.Application, error) {
	// traverse the appSelector cache
	apps := &sigappv1beta1.ApplicationList{}
	err := h.List(context.TODO(), apps, client.InNamespace(namespace))

	if err != nil {
		return nil, err
	}

	for _, app := range apps.Items {
		selector, _ := utils.ConvertLabel(app.Spec.Selector)
		if selector.Matches(labels.Set(hdpl.GetLabels())) {
			//locate the app in the same NS, or in all namespaces
			if hdpl.GetNamespace() == app.Namespace {

				return &app, nil
			}
		}
	}
	return nil, nil
}

func (h *hybridDeployableMapper) convertObjectToHybridDeployable(metaobj *metav1.Object) (*hdplv1alpha1.HybridDeployable, error) {
	uc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(metaobj)
	if err != nil {
		klog.Error("Failed to convert object to unstructured with error:", err)
		return nil, err
	}

	dpl := &hdplv1alpha1.HybridDeployable{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uc, dpl)
	if err != nil {
		klog.Error("Failed to convert unstructured to hybrid deployable with error:", err)
		return nil, err
	}

	return dpl, nil
}
