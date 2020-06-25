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

package customresourcedefinition

import (
	"context"

	"github.com/hybridapp-io/ham-application-assembler/pkg/utils"
	crds "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	packageInfoLogLevel = 3
)

// Add creates a new CustomResourceDefinition Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	utils.BuildGVKGVRMap(mgr.GetConfig())
	return &ReconcileCustomResourceDefinition{
		Client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("customresourcedefinition-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CustomResourceDefinition
	err = c.Watch(&source.Kind{Type: &crds.CustomResourceDefinition{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCustomResourceDefinition implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCustomResourceDefinition{}

// ReconcileCustomResourceDefinition reconciles a CustomResourceDefinition object
type ReconcileCustomResourceDefinition struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCustomResourceDefinition) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.V(packageInfoLogLevel).Info("Reconciling CustomResourceDefinition ", request)

	instance := &crds.CustomResourceDefinition{}

	err := r.Get(context.TODO(), request.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// not enough info in crd request to identify gvk/gvr. Rebuilding the entire map
			utils.RebuildGVKGVRMap(r.config)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	gvk := schema.GroupVersionKind{
		Group:   instance.Spec.Group,
		Version: instance.Spec.Version,
		Kind:    instance.Spec.Names.Kind,
	}
	if utils.GVKGVRMap != nil {
		if _, ok := utils.GVKGVRMap[gvk]; ok {
			return reconcile.Result{}, nil
		}

	}
	utils.RebuildGVKGVRMap(r.config)
	return reconcile.Result{}, nil
}
