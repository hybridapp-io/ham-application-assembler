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

	prulev1alpha1 "github.com/hybridapp-io/ham-placement/pkg/apis/core/v1alpha1"
	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
	subv1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
	"github.com/hybridapp-io/ham-application-assembler/pkg/utils"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	hdplv1alpha1utils "github.com/hybridapp-io/ham-deployable-operator/pkg/utils"
)

func (r *ReconcileApplicationAssembler) generateHybridDeployableFromObject(instance *toolsv1alpha1.ApplicationAssembler,
	objref *corev1.ObjectReference, appID string) error {
	objgvr, ok := utils.GVKGVRMap[objref.GetObjectKind().GroupVersionKind()]
	if !ok {
		klog.Error("Failed to find GVR for object:", objref.GetObjectKind().GroupVersionKind())

		// requeue , as the crd controller will refresh the GVKGVR map eventually
		return errors.NewNotFound(schema.GroupResource{
			Group:    objref.GroupVersionKind().Group,
			Resource: objref.GroupVersionKind().Kind,
		}, objref.Name)
	}

	ucobj, err := r.dynamicClient.Resource(objgvr).Namespace(objref.Namespace).Get(context.TODO(), objref.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error("Failed to obtain component with error:", err, "object reference:", objref)
		return err
	}

	var key types.NamespacedName
	key.Name = r.genHybridDeployableName(instance, objref, nil)
	key.Namespace = instance.Namespace
	hdpl := &hdplv1alpha1.Deployable{}

	err = r.Get(context.TODO(), key, hdpl)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error("Failed to retrieve hybrid deployable ", key.String(), " with error: ", err)
			return err
		}

		hdpl.Name = key.Name
		hdpl.Namespace = key.Namespace
	}

	labels := hdpl.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[toolsv1alpha1.LabelApplicationPrefix+appID] = appID
	hdpl.SetLabels(labels)
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
	err = r.genPlacementRuleForHybridDeployable(hdpl, &deployer.Spec.Type)
	if err != nil {
		klog.Error("Failed to generate placementrule for hybrid deployable ", hdpl.Namespace+"/"+hdpl.Name)
		return err
	}
	err = r.patchObject(hdpl, ucobj)
	if err != nil {
		klog.Error("Failed to patch object with error: ", err)
	}

	if hdpl.UID != "" {
		if err = r.Update(context.TODO(), hdpl); err != nil {
			klog.Error("Failed to update hybrid deployable ", hdpl.Namespace+"/"+hdpl.Name)
			return err
		}

	} else {
		if err = r.Create(context.TODO(), hdpl); err != nil {
			klog.Error("Failed to create hybrid deployable ", hdpl.Namespace+"/"+hdpl.Name)
			return err
		}
	}

	return err
}

func (r *ReconcileApplicationAssembler) generateHybridDeployableFromObjectInManagedCluster(instance *toolsv1alpha1.ApplicationAssembler,
	obj *corev1.ObjectReference, appID string, cluster *types.NamespacedName) error {

	hdplKey := types.NamespacedName{
		Name:      r.genHybridDeployableName(instance, obj, cluster),
		Namespace: instance.Namespace,
	}
	hdpl := &hdplv1alpha1.Deployable{}

	err := r.Get(context.TODO(), hdplKey, hdpl)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error("Failed to retrieve hybrid deployable ", hdplKey.String())
			return err
		}
		hdpl.Name = r.genHybridDeployableName(instance, obj, cluster)
		hdpl.Namespace = hdplKey.Namespace

		dpl := &dplv1.Deployable{}
		dpl.GenerateName = strings.ToLower(obj.Kind + "-" + obj.Namespace + "-" + obj.Name + "-")
		dpl.Namespace = cluster.Namespace
		annotations := make(map[string]string)
		annotations[hdplv1alpha1.AnnotationHybridDiscovery] = hdplv1alpha1.HybridDiscoveryEnabled
		dpl.Annotations = annotations

		tpl := &unstructured.Unstructured{}
		tpl.SetAPIVersion(obj.APIVersion)
		tpl.SetKind(obj.Kind)
		tpl.SetName(obj.Name)
		tpl.SetNamespace(obj.Namespace)
		dpl.Spec.Template = &runtime.RawExtension{
			Object: tpl,
		}
		hdpl.Annotations = dpl.Annotations

		return r.buildHybridDeployable(hdpl, dpl, appID)
	}

	labels := hdpl.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[toolsv1alpha1.LabelApplicationPrefix+appID] = appID
	hdpl.SetLabels(labels)
	if err = r.Update(context.TODO(), hdpl); err != nil {
		klog.Error("Failed to update hybrid deployable ", hdpl.Namespace+"/"+hdpl.Name)
		return err
	}

	return nil
}

func (r *ReconcileApplicationAssembler) selectDeployer(deployers []prulev1alpha1.Deployer, ucobj *unstructured.Unstructured) (*prulev1alpha1.Deployer, error) {
	gvr, ok := utils.GVKGVRMap[ucobj.GetObjectKind().GroupVersionKind()]
	if !ok {
		klog.Error("Failed to find GVR for object:", ucobj.GetObjectKind().GroupVersionKind())

		// requeue , as the crd controller will refresh the GVKGVR map eventually
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    ucobj.GroupVersionKind().Group,
			Resource: ucobj.GroupVersionKind().Kind,
		}, ucobj.GetName())
	}
	if ok {
		for i := range deployers {
			hubDeployer := deployers[i]
			// in cluster deployer
			if hdplv1alpha1utils.IsInClusterDeployer(&hubDeployer) {
				// cluster vs namespaced scope
				if hubDeployer.Spec.Scope == apiextensions.ClusterScoped || hubDeployer.ObjectMeta.Namespace == ucobj.GetNamespace() {
					if hubDeployer.Spec.Capabilities != nil {
						for _, capability := range hubDeployer.Spec.Capabilities {
							// group
							for _, apiGroup := range capability.APIGroups {
								if apiGroup == "*" || apiGroup == utils.StripVersion(ucobj.GetAPIVersion()) {
									for _, resource := range capability.Resources {
										if resource == "*" || resource == gvr.Resource {
											return &hubDeployer, nil
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return nil, nil
}

func (r *ReconcileApplicationAssembler) generateHybridTemplateFromObject(ucobj *unstructured.Unstructured) (*hdplv1alpha1.HybridTemplate,
	*prulev1alpha1.Deployer, error) {

	deployerlist := &prulev1alpha1.DeployerList{}
	if err := r.List(context.TODO(), deployerlist, &client.ListOptions{}); err != nil {
		klog.Error("Failed to list deployers")
		return nil, nil, err
	}
	deployer, err := r.selectDeployer(deployerlist.Items, ucobj)
	if err != nil {
		klog.Error("Error occurred while trying to select a deployer ", err)
		return nil, nil, err
	}
	if deployer == nil {
		deployer = &prulev1alpha1.Deployer{}
		deployer.Spec.Type = toolsv1alpha1.DefaultDeployerType
		deployer.Name = toolsv1alpha1.DefaultDeployerType
		deployer.Namespace = ucobj.GetNamespace()
	}

	htpl := &hdplv1alpha1.HybridTemplate{}
	htpl.DeployerType = deployer.Spec.Type

	annotations := ucobj.GetAnnotations()
	if annotations != nil && annotations[prulev1alpha1.DeployerType] != "" {
		htpl.DeployerType = annotations[prulev1alpha1.DeployerType]
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
