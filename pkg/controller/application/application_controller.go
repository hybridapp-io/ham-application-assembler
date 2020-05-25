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

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"

	"github.com/hybridapp-io/ham-application-assembler/pkg/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	toolsv1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/tools/v1alpha1"
)

// Add creates a new Application Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	dynamicclient, err := dynamic.NewForConfig(mgr.GetConfig())

	if err != nil {
		klog.Error("Failed to create dynamic client with error:", err)
		return nil
	}

	return &ReconcileApplication{
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		gvkGVRMap:     utils.BuildGVKGVRMap(mgr.GetConfig()),
		dynamicClient: dynamicclient,
		restMapper:    mgr.GetRESTMapper()}

}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("application-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Application
	err = c.Watch(&source.Kind{Type: &sigappv1beta1.Application{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &hdplv1alpha1.Deployable{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &hybridDeployableMapper{mgr.GetClient()},
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				// no need to watch the hdpl updates, we only care about create/delete for app status
				return false
			},
		},
	)
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileApplication implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileApplication{}

// ReconcileApplication reconciles a Application object
type ReconcileApplication struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	dynamicClient dynamic.Interface
	gvkGVRMap     map[schema.GroupVersionKind]schema.GroupVersionResource
	scheme        *runtime.Scheme
	restMapper    meta.RESTMapper
}

// Reconcile reads that state of the cluster for a Application object and makes changes based on the state read
// and what is in the Application.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileApplication) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.Info("Reconciling Application ", request)

	// Fetch the ApplicationAssembler instance
	app := &sigappv1beta1.Application{}

	err := r.Get(context.TODO(), request.NamespacedName, app)
	if err != nil {
		if errors.IsNotFound(err) {
			// Recommended way is to use finalizers for dependency cleanup. It is not going to work for our resources though
			// as a finalizer on the app will get propagated through its deployable onto the managed cluster , which will
			// prevent the app cleanup when the deployable is removed
			depErr := r.deleteApplicationDeployables(request.NamespacedName)

			return reconcile.Result{}, depErr
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	appKey := types.NamespacedName{
		Name:      app.GetName(),
		Namespace: app.GetNamespace(),
	}

	// if discovery annotation is not enabled, return. Otherwise go through deployable reconciliation
	if !r.isAppDiscoveryEnabled(app) {
		// update application status based on the app selector and list of component kinds
		err = r.updateApplicationStatus(app)
		if err != nil {
			klog.Error("Failed to reconcile status for application "+appKey.String()+" with error: ", err)
		}
		return reconcile.Result{}, err
	}

	// reconcile the application deployables which might trigger an app reconcile on all managed clusters
	err = r.reconcileAppDeployables(app)
	if err != nil {
		klog.Error("Failed to create deployable for application "+appKey.String()+" with error: ", err)
		return reconcile.Result{}, err
	}

	if r.isCreateAssemblerEnabled(app) {
		appasm := &toolsv1alpha1.ApplicationAssembler{}
		err := r.Get(context.TODO(), appKey, appasm)

		if err != nil {
			if errors.IsNotFound(err) {
				klog.Info("Creating application assembler for application  ", appKey.String())

				err = r.createApplicationAssembler(app)
				if err != nil {
					klog.Error("Failed to create application assemblers with error: ", err)
					return reconcile.Result{}, err
				}

				err = r.updateDiscoveryAnnotations(app)
				if err != nil {
					klog.Error("Failed to update annotation", toolsv1alpha1.AnnotationCreateAssembler, " with error: ", err)
					return reconcile.Result{}, err
				}

				err = r.deleteApplicationDeployables(request.NamespacedName)
				if err != nil {
					klog.Error("Failed to delete deployables for application "+appKey.String()+" with error: ", err)
				}
			}
		}
	}
	// update application status based on the app selector and list of component kinds
	err = r.updateApplicationStatus(app)
	if err != nil {
		klog.Error("Failed to reconcile status for application "+appKey.String()+" with error: ", err)
		return reconcile.Result{}, err
	}

	klog.V(packageInfoLogLevel).Info("Reconcile Application: ", request, " finished with error:", err)

	return reconcile.Result{}, err
}
