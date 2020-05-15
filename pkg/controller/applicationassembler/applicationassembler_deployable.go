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
	"encoding/json"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
	prulev1 "github.com/open-cluster-management/multicloud-operators-placementrule/pkg/apis/apps/v1"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
)

func (r *ReconcileApplicationAssembler) generateHybridDeployableFromDeployable(instance *toolsv1alpha1.ApplicationAssembler,
	obj *corev1.ObjectReference, appID string, cluster *types.NamespacedName) error {
	var err error

	dpl := &dplv1.Deployable{}
	key := types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Name,
	}

	if obj.Namespace == "" {
		obj.Namespace = instance.Namespace
	}

	err = r.Get(context.TODO(), key, dpl)
	if err != nil {
		klog.Error("Failed to obtain deployable object for application with error:", err)
		return err
	}

	// generate the hdpl name based on the template object if possible to avoid clutter around discovered deployables with long names
	if dpl.Spec.Template != nil {
		templateobj := &unstructured.Unstructured{}
		err = json.Unmarshal(dpl.Spec.Template.Raw, templateobj)
		if err != nil {
			klog.Info("Failed to unmarshal object with error", err)
			return err
		}
		key.Name = r.genHybridDeployableName(instance, &corev1.ObjectReference{
			Kind:      templateobj.GetKind(),
			Namespace: templateobj.GetNamespace(),
			Name:      templateobj.GetName(),
		}, cluster)
	} else {
		key.Name = r.genHybridDeployableName(instance, obj, cluster)
	}
	key.Namespace = instance.Namespace
	hdpl := &hdplv1alpha1.Deployable{}

	err = r.Get(context.TODO(), key, hdpl)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error("Failed to retrieve hybrid deployable with error: ", err)
			return err
		}

		hdpl.Name = key.Name
		hdpl.Namespace = key.Namespace
	}
	err = r.patchObject(hdpl, dpl)
	if err != nil {
		klog.Error("Failed to patch deployable : ", dpl.Namespace+"/"+dpl.Name, " with error: ", err)
		return err
	}
	return r.buildHybridDeployable(hdpl, dpl, appID)
}

func (r *ReconcileApplicationAssembler) buildHybridDeployable(hdpl *hdplv1alpha1.Deployable, dpl *dplv1.Deployable, appID string) error {

	newtpl := &hdplv1alpha1.HybridTemplate{}
	newtpl.DeployerType = hdplv1alpha1.DefaultDeployerType

	annotations := dpl.GetAnnotations()
	if annotations != nil && annotations[hdplv1alpha1.DeployerType] != "" {
		newtpl.DeployerType = annotations[hdplv1alpha1.DeployerType]
	}

	newtpl.Template = dpl.Spec.Template

	labels := hdpl.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[toolsv1alpha1.LabelApplicationPrefix+appID] = appID
	hdpl.SetLabels(labels)

	htpls := []hdplv1alpha1.HybridTemplate{*newtpl}

	for _, htpl := range hdpl.Spec.HybridTemplates {
		if htpl.DeployerType != newtpl.DeployerType {
			htpls = append(htpls, *(htpl.DeepCopy()))
		}
	}

	hdpl.Spec.HybridTemplates = htpls

	err := r.genPlacementRuleForHybridDeployable(hdpl, dpl.Namespace)
	if err != nil {
		klog.Error("Failed to generate placementrule for hybrid deployable ", hdpl.Namespace+"/"+hdpl.Name)
		return err
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

	return nil

}

func (r *ReconcileApplicationAssembler) genPlacementRuleForHybridDeployable(hdpl *hdplv1alpha1.Deployable, clusterNamespace string) error {

	key := types.NamespacedName{Namespace: hdpl.Namespace, Name: hdpl.Name}

	prule := &prulev1.PlacementRule{}
	prule.Spec.Clusters = []prulev1.GenericClusterReference{
		{
			Name: clusterNamespace,
		},
	}
	prule.Status.Decisions = []prulev1.PlacementDecision{
		{
			ClusterName:      clusterNamespace,
			ClusterNamespace: clusterNamespace,
		},
	}
	hdpl.Spec.Placement = &hdplv1alpha1.HybridPlacement{}

	pruleList := &prulev1.PlacementRuleList{}
	err := r.List(context.TODO(), pruleList, &client.ListOptions{Namespace: hdpl.Namespace})
	if err != nil {
		klog.Error("Failed to retrieve the list of placement rules for hybrid deployable ", key.String())
		return err
	}
	for _, placementRule := range pruleList.Items {
		if reflect.DeepEqual(placementRule.Spec, prule.Spec) && reflect.DeepEqual(placementRule.Status, prule.Status) {
			hdpl.Spec.Placement.PlacementRef = &corev1.ObjectReference{Name: placementRule.Name}
			return nil
		}
	}

	prule.Name = key.Name
	prule.Namespace = key.Namespace

	if err = r.Create(context.TODO(), prule); err != nil {
		klog.Error("Failed to create placement rule for hybrid deployable ", key.String())
		return err

	}
	if err = r.Status().Update(context.TODO(), prule); err != nil {
		klog.Error("Failed to update placement rule status for hybrid deployable ", key.String(), " with error ", err)
	}

	hdpl.Spec.Placement.PlacementRef = &corev1.ObjectReference{Name: prule.Name}

	return nil
}
