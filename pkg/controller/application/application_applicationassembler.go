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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
)

func (r *ReconcileApplication) createApplicationAssembler(app *sigappv1beta1.Application) error {
	appasm := &toolsv1alpha1.ApplicationAssembler{}

	appasm.SetGroupVersionKind(applicationAssemblerGVK)
	appasm.Name = app.GetName()
	appasm.Namespace = app.GetNamespace()

	// add the matching deployables
	resources, err := r.fetchApplicationComponents(app)
	if err != nil {
		return err
	}
	objectReferences := r.buildAssemblerComponents(resources)
	appasm.Spec.HubComponents = objectReferences

	return r.Create(context.TODO(), appasm)
}

func (r *ReconcileApplication) buildAssemblerComponents(resources []*unstructured.Unstructured) []*corev1.ObjectReference {

	var objectReferences []*corev1.ObjectReference
	for _, resource := range resources {
		or := corev1.ObjectReference{

			Kind:       resource.GetKind(),
			Name:       resource.GetName(),
			Namespace:  resource.GetNamespace(),
			APIVersion: resource.GetAPIVersion(),
		}
		objectReferences = append(objectReferences, &or)
	}
	return objectReferences

}
