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
	"reflect"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	prulev1alpha1 "github.com/hybridapp-io/ham-placement/pkg/apis/core/v1alpha1"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ReconcileApplicationAssembler) genPlacementRuleForHybridDeployable(hdpl *hdplv1alpha1.Deployable, deployerType *string,
	clusterName string) error {

	key := types.NamespacedName{Namespace: hdpl.Namespace, Name: hdpl.Name}

	prule := &prulev1alpha1.PlacementRule{}
	if deployerType != nil {
		prule.Spec.DeployerType = deployerType
	}
	if clusterName != "" {
		managedCluster := &managedclusterv1.ManagedCluster{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: clusterName}, managedCluster); err != nil {
			klog.Error("Cannot find managed cluster ", clusterName)
			return err
		}
		clusterManagedObject := corev1.ObjectReference{
			Name:       managedCluster.Name,
			APIVersion: managedCluster.APIVersion,
		}
		prule.Spec.Targets = make([]corev1.ObjectReference, 1)
		prule.Spec.Targets[0] = clusterManagedObject

	}
	hdpl.Spec.Placement = &hdplv1alpha1.HybridPlacement{}

	pruleList := &prulev1alpha1.PlacementRuleList{}
	err := r.List(context.TODO(), pruleList, &client.ListOptions{Namespace: hdpl.Namespace})
	if err != nil {
		klog.Error("Failed to retrieve the list of placement rules for hybrid deployable ", key.String(), " with error: ", err)
		return err
	}
	for _, placementRule := range pruleList.Items {
		if reflect.DeepEqual(placementRule.Spec, prule.Spec) {
			hdpl.Spec.Placement.PlacementRef = &corev1.ObjectReference{Name: placementRule.Name}
			return nil
		}
	}

	prule.Name = key.Name
	prule.Namespace = key.Namespace

	if err = r.Create(context.TODO(), prule); err != nil {
		klog.Error("Failed to create placement rule for hybrid deployable ", key.String(), " with error: ", err)
		return err

	}
	if err = r.Status().Update(context.TODO(), prule); err != nil {
		klog.Error("Failed to update placement rule status for hybrid deployable ", key.String(), " with error: ", err)
	}

	hdpl.Spec.Placement.PlacementRef = &corev1.ObjectReference{Name: prule.Name}

	return nil
}
