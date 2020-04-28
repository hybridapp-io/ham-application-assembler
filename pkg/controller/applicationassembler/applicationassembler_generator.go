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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	sigappv1beta1 "github.com/kubernetes-sigs/application/pkg/apis/app/v1beta1"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
)

func (r *ReconcileApplicationAssembler) getOrCreateApplication(instance *toolsv1alpha1.ApplicationAssembler) (*sigappv1beta1.Application, error) {
	if instance == nil {
		return nil, nil
	}

	var app = &sigappv1beta1.Application{}

	appkey := types.NamespacedName{Name: instance.Spec.Application.Name, Namespace: instance.Spec.Application.Namespace}
	if appkey.Namespace == "" {
		appkey.Namespace = instance.Namespace
	}

	err := r.Get(context.TODO(), appkey, app)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error("Failed to get applications for assembler with error:", err)
			return nil, err
		}
		appns := instance.Spec.Application.Namespace
		if appns == "" {
			appns = instance.Namespace
		}

		app.Name = instance.Spec.Application.Name
		app.Namespace = appns

	}

	klog.V(packageDetailLogLevel).Info("found existing application", app)

	return app, nil
}

func (r *ReconcileApplicationAssembler) generateHybridDeployables(instance *toolsv1alpha1.ApplicationAssembler, appID string) error {
	var err error

	for _, obj := range instance.Spec.Components {
		if obj.GetObjectKind().GroupVersionKind().Empty() || obj.GetObjectKind().GroupVersionKind() == toolsv1alpha1.DeployableGVK {
			err = r.generateHybridDeployableFromDeployable(instance, obj, appID)
		} else {
			err = r.generateHybridDeployableFromObject(instance, obj, appID)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileApplicationAssembler) patchObject(hdpl *hdplv1alpha1.Deployable, metaobj metav1.Object) error {
	annotations := metaobj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[hdplv1alpha1.HostingHybridDeployable] = types.NamespacedName{Namespace: hdpl.Namespace, Name: hdpl.GetName()}.String()
	metaobj.SetAnnotations(annotations)

	labels := metaobj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[hdplv1alpha1.ControlledBy] = hdplv1alpha1.HybridDeployableController
	labels[hdplv1alpha1.HostingHybridDeployable] = hdpl.GetName()
	metaobj.SetLabels(labels)

	metaobj.SetGenerateName("")

	return r.Update(context.TODO(), metaobj.(runtime.Object))
}

func (r *ReconcileApplicationAssembler) genHybridDeployableName(instance *toolsv1alpha1.ApplicationAssembler,
	metaobj metav1.Object) string {
	if instance == nil || metaobj == nil {
		return ""
	}

	name := instance.Spec.Application.Name + "-"
	if metaobj.GetGenerateName() != "" {
		name += metaobj.GetGenerateName()
	} else {
		labels := metaobj.GetLabels()
		if _, ok := labels[hdplv1alpha1.HostingHybridDeployable]; ok && labels[hdplv1alpha1.HostingHybridDeployable] != "" {
			// if the object belongs to a hybriddeployable already, use same name
			name = labels[hdplv1alpha1.HostingHybridDeployable]
		} else {
			name += metaobj.GetName()
		}
	}

	if len(name) > 1 && name[len(name)-1] == '-' {
		name = name[:len(name)-1]
	}

	return name
}

func (r *ReconcileApplicationAssembler) updateApplication(instance *toolsv1alpha1.ApplicationAssembler, app *sigappv1beta1.Application) (string, error) {
	err := controllerutil.SetControllerReference(instance, app, r.scheme)
	if err != nil {
		klog.Error("Failed to set controller runtime with error: ", err)
	}

	if app.Spec.Selector == nil {
		app.Spec.Selector = &metav1.LabelSelector{}
	}

	if app.Spec.Selector.MatchLabels == nil {
		app.Spec.Selector.MatchLabels = make(map[string]string)
	}

	kindincluded := false

	for _, kind := range app.Spec.ComponentGroupKinds {
		if kind == toolsv1alpha1.HybridDeployableGK {
			kindincluded = true
			break
		}
	}

	if !kindincluded {
		app.Spec.ComponentGroupKinds = append(app.Spec.ComponentGroupKinds, toolsv1alpha1.HybridDeployableGK)
	}

	selectorLabels := map[string]string{
		toolsv1alpha1.LabelApplicationPrefix + string(app.UID): string(app.UID),
	}
	app.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: selectorLabels,
	}
	// rely on app reconcile for status
	app.Status = sigappv1beta1.ApplicationStatus{}
	// existing app
	if app.UID != "" {
		err = r.Update(context.TODO(), app)
		_ = r.Status().Update(context.TODO(), app)

		return string(app.UID), err
	}

	// new app
	err = r.Create(context.TODO(), app)
	_ = r.Status().Update(context.TODO(), app)

	return string(app.UID), err
}
