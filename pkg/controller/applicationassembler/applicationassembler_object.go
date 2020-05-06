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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
	subv1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
)

func (r *ReconcileApplicationAssembler) generateHybridDeployableFromObject(instance *toolsv1alpha1.ApplicationAssembler,
	objref *corev1.ObjectReference, appID string) error {
	objgvr, ok := r.gvkGVRMap[objref.GetObjectKind().GroupVersionKind()]
	if !ok {
		klog.Error("Failed to find right resource group for object:", objref)
	}

	ucobj, err := r.dynamicClient.Resource(objgvr).Namespace(objref.Namespace).Get(objref.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error("Failed to obtain component with error:", err, "object reference:", objref)
		return err
	}

	var key types.NamespacedName
	key.Name = r.genHybridDeployableName(instance, objref)
	key.Namespace = instance.Namespace
	hdpl := &hdplv1alpha1.Deployable{}

	labels := hdpl.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[toolsv1alpha1.LabelApplicationPrefix+appID] = appID
	hdpl.SetLabels(labels)

	err = r.Get(context.TODO(), key, hdpl)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error("Failed to work with api server for hybrid deployable with error:", err)
			return err
		}

		hdpl.Name = key.Name
		hdpl.Namespace = key.Namespace
	}

	newtpl, deployer, err := r.generateHybridTemplateFromObject(ucobj)
	if err != nil {
		klog.Error("Failed to generate hybrid template from object with error:", err)
		return err
	}

	htpls := []hdplv1alpha1.HybridTemplate{*newtpl}

	for _, htpl := range hdpl.Spec.HybridTemplates {
		if htpl.DeployerType != newtpl.DeployerType {
			htpls = append(htpls, *(htpl.DeepCopy()))
		}
	}

	hdpl.Spec.HybridTemplates = htpls
	deployerref := corev1.ObjectReference{
		Name:      deployer.Name,
		Namespace: deployer.Namespace,
	}
	hdpl.Spec.Placement = &hdplv1alpha1.HybridPlacement{}
	hdpl.Spec.Placement.Deployers = []corev1.ObjectReference{deployerref}

	err = r.patchObject(hdpl, ucobj)
	if err != nil {
		klog.Error("Failed to patch object with error: ", err)
	}

	if hdpl.UID != "" {
		err = r.Update(context.TODO(), hdpl)
	} else {
		err = r.Create(context.TODO(), hdpl)
	}

	return err
}

func (r *ReconcileApplicationAssembler) generateHybridDeployableFromObjectInManagedCluster(instance *toolsv1alpha1.ApplicationAssembler,
	obj *corev1.ObjectReference, appID string, cluster types.NamespacedName) error {

	hdplKey := types.NamespacedName{
		Name:      r.genHybridDeployableName(instance, obj),
		Namespace: instance.Namespace,
	}
	hdpl := &hdplv1alpha1.Deployable{}

	err := r.Get(context.TODO(), hdplKey, hdpl)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error("Failed to retrieve hybrid deployable ", hdplKey.String())
			return err
		}
		hdpl.Name = hdplKey.Name
		hdpl.Namespace = hdplKey.Namespace

		dpl := &dplv1.Deployable{}
		dpl.GenerateName = strings.ToLower(obj.Kind + "-" + obj.Namespace + "-" + obj.Name + "-")
		dpl.Namespace = cluster.Namespace
		annotations := make(map[string]string)
		annotations[toolsv1alpha1.AnnotationDiscover] = toolsv1alpha1.DiscoveryEnabled
		dpl.Annotations = annotations

		tpl := &unstructured.Unstructured{}
		tpl.SetAPIVersion(obj.APIVersion)
		tpl.SetKind(obj.Kind)
		tpl.SetName(obj.Name)
		tpl.SetNamespace(obj.Namespace)
		dpl.Spec.Template = &runtime.RawExtension{
			Object: tpl,
		}
		r.updateObject(hdpl, dpl)
		err := r.Client.Create(context.TODO(), dpl)
		if err != nil {
			klog.Error("Failed to create deployable ", cluster.Namespace+"/"+obj.Name)
			return err
		}

		return r.buildHybridDeployable(hdpl, dpl, appID)
	}
	return nil
}

func (r *ReconcileApplicationAssembler) generateHybridTemplateFromObject(ucobj *unstructured.Unstructured) (*hdplv1alpha1.HybridTemplate,
	*hdplv1alpha1.Deployer, error) {
	var err error

	var deployer *hdplv1alpha1.Deployer

	deployerlist := &hdplv1alpha1.DeployerList{}

	err = r.List(context.TODO(), deployerlist, &client.ListOptions{Namespace: ucobj.GetNamespace()})
	if err != nil {
		klog.Error("Failed to list deployers in namespace with error:", err)
		return nil, nil, err
	}

	for _, item := range deployerlist.Items {
		deployer = &item
	}

	// check deployerset for cluster namespace
	if deployer == nil {
		deployersetlist := &hdplv1alpha1.DeployerSetList{}

		err = r.List(context.TODO(), deployersetlist, &client.ListOptions{Namespace: ucobj.GetNamespace()})
		if err != nil {
			klog.Error("Failed to list deployers in namespace with error:", err)
			return nil, nil, err
		}

		for _, item := range deployersetlist.Items {
			if item.Spec.DefaultDeployer == "" && len(item.Spec.Deployers) > 0 {
				item.Spec.DefaultDeployer = item.Spec.Deployers[0].Key
			}

			for _, dply := range item.Spec.Deployers {
				if dply.Key == item.Spec.DefaultDeployer {
					deployer = &hdplv1alpha1.Deployer{}
					dply.Spec.DeepCopyInto(&deployer.Spec)

					break
				}
			}
		}
	}

	if deployer == nil {
		deployer = &hdplv1alpha1.Deployer{}
		deployer.Spec.Type = toolsv1alpha1.DefaultDeployerType
		deployer.Namespace = ucobj.GetNamespace()
	}

	htpl := &hdplv1alpha1.HybridTemplate{}
	htpl.DeployerType = deployer.Spec.Type

	annotations := ucobj.GetAnnotations()
	if annotations != nil && annotations[hdplv1alpha1.DeployerType] != "" {
		htpl.DeployerType = annotations[hdplv1alpha1.DeployerType]
	}

	htpl.Template = &runtime.RawExtension{}
	tplobj := ucobj.DeepCopy()
	r.prepareTemplate(tplobj)
	htpl.Template.Object = tplobj

	return htpl, deployer, nil
}

var (
	obsoleteAnnotations = []string{
		"kubectl.kubernetes.io/last-applied-configuration",
		dplv1.AnnotationHosting,
		subv1.AnnotationHosting,
		subv1.AnnotationSyncSource,
	}
	obsoleteLabels = []string{
		hdplv1alpha1.ControlledBy,
	}
)

func (r *ReconcileApplicationAssembler) prepareTemplate(template *unstructured.Unstructured) {
	var emptyuid types.UID

	template.SetUID(emptyuid)
	template.SetSelfLink("")
	template.SetResourceVersion("")
	template.SetGeneration(0)
	template.SetCreationTimestamp(metav1.Time{})

	annotations := template.GetAnnotations()
	if annotations != nil {
		for _, k := range obsoleteAnnotations {
			delete(annotations, k)
		}

		template.SetAnnotations(annotations)
	}

	labels := template.GetLabels()
	if labels != nil {
		for _, k := range obsoleteLabels {
			delete(labels, k)
		}

		template.SetLabels(labels)
	}

	delete(template.Object, "status")
}
