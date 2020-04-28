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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
	prulev1 "github.com/open-cluster-management/multicloud-operators-placementrule/pkg/apis/apps/v1"

	corev1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/core/v1alpha1"

	deployerv1alpha1 "github.com/IBM/deployer-operator/pkg/apis/app/v1alpha1"
	hdplv1 "github.com/IBM/hybriddeployable-operator/pkg/apis/app/v1alpha1"
)

func (r *ReconcileApplicationAssembler) generateHybridDeployableFromDeployable(instance *corev1alpha1.ApplicationAssembler,
	obj *corev1.ObjectReference, appID string) error {
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

	newtpl := &hdplv1.HybridTemplate{}
	newtpl.DeployerType = hdplv1.DefaultDeployerType

	annotations := dpl.GetAnnotations()
	if annotations != nil && annotations[deployerv1alpha1.DeployerType] != "" {
		newtpl.DeployerType = annotations[deployerv1alpha1.DeployerType]
	}

	newtpl.Template = dpl.Spec.Template

	key.Name = r.genHybridDeployableName(instance, dpl)
	key.Namespace = instance.Namespace
	hdpl := &hdplv1.HybridDeployable{}

	err = r.Get(context.TODO(), key, hdpl)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error("Failed to work with api server for hybrid deployable with error:", err)
			return err
		}

		hdpl.Name = key.Name
		hdpl.Namespace = key.Namespace
	}

	labels := hdpl.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[corev1alpha1.LabelApplicationPrefix+appID] = appID
	hdpl.SetLabels(labels)

	htpls := []hdplv1.HybridTemplate{*newtpl}

	for _, htpl := range hdpl.Spec.HybridTemplates {
		if htpl.DeployerType != newtpl.DeployerType {
			htpls = append(htpls, *(htpl.DeepCopy()))
		}
	}

	hdpl.Spec.HybridTemplates = htpls

	err = r.patchObject(hdpl, dpl)
	if err != nil {
		klog.Error("Failed to patch object with error: ", err)
	}

	err = r.genPlacementRuleForHybridDeployable(hdpl, dpl)
	if err != nil {
		klog.Error("Failed to generate placementrule for hybrid deployable with error:", err)
		return err
	}

	if hdpl.UID != "" {
		err = r.Update(context.TODO(), hdpl)
	} else {
		err = r.Create(context.TODO(), hdpl)
	}

	return err
}

func (r *ReconcileApplicationAssembler) genPlacementRuleForHybridDeployable(hdpl *hdplv1.HybridDeployable, dpl *dplv1.Deployable) error {
	prule := &prulev1.PlacementRule{}
	key := types.NamespacedName{Namespace: hdpl.Namespace, Name: hdpl.Name}

	err := r.Get(context.TODO(), key, prule)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error("Failed to check placementrule with error:", err)
			return err
		}

		prule.Name = key.Name
		prule.Namespace = key.Namespace
	}

	objcluster := prulev1.GenericClusterReference{
		Name: dpl.Namespace,
	}

	prule.Spec.Clusters = []prulev1.GenericClusterReference{
		objcluster,
	}

	objdecision := prulev1.PlacementDecision{
		ClusterName:      dpl.Namespace,
		ClusterNamespace: dpl.Namespace,
	}
	prule.Status.Decisions = []prulev1.PlacementDecision{
		objdecision,
	}

	hdpl.Spec.Placement = &hdplv1.HybridPlacement{}
	hdpl.Spec.Placement.PlacementRef = &corev1.ObjectReference{Name: prule.Name}

	if prule.UID != "" {
		err = r.Update(context.TODO(), prule)
		_ = r.Status().Update(context.TODO(), prule)

		return err
	}

	err = r.Create(context.TODO(), prule)
	_ = r.Status().Update(context.TODO(), prule)

	return err
}
