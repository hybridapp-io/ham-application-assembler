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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	corev1alpha1 "github.com/hybridapp-io/ham-application-assembler/pkg/apis/core/v1alpha1"
	"github.com/hybridapp-io/ham-application-assembler/pkg/utils"
)

const (
	packageInfoLogLevel   = 3
	packageDetailLogLevel = 5
)

// Add creates a new ApplicationAssembler Controller and adds it to the Manager. The Manager will set fields on the Controller
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

	return &ReconcileApplicationAssembler{Client: mgr.GetClient(), scheme: mgr.GetScheme(),
		gvkGVRMap: utils.BuildGVKGVRMap(mgr.GetConfig()), dynamicClient: dynamicclient}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("applicationassembler-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ApplicationAssembler
	err = c.Watch(&source.Kind{Type: &corev1alpha1.ApplicationAssembler{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newaa := e.ObjectNew.(*corev1alpha1.ApplicationAssembler)
			oldaa := e.ObjectOld.(*corev1alpha1.ApplicationAssembler)

			if !reflect.DeepEqual(oldaa.Spec, newaa.Spec) {
				return true
			}

			if !reflect.DeepEqual(oldaa.Labels, newaa.Labels) {
				return true
			}

			if !reflect.DeepEqual(oldaa.Annotations, newaa.Annotations) {
				return true
			}

			return false
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileApplicationAssembler implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileApplicationAssembler{}

// ReconcileApplicationAssembler reconciles a ApplicationAssembler object
type ReconcileApplicationAssembler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	dynamicClient dynamic.Interface
	gvkGVRMap     map[schema.GroupVersionKind]schema.GroupVersionResource
	scheme        *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ApplicationAssembler object and makes changes based on the state read
// and what is in the ApplicationAssembler.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileApplicationAssembler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.Info("Reconciling ApplicationAssembler ", request)

	// Fetch the ApplicationAssembler instance
	instance := &corev1alpha1.ApplicationAssembler{}

	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	app, err := r.getOrCreateApplication(instance)
	if err != nil {
		klog.Error("Failed to get or create application with error: ", err)
		return r.concludeReconcile(instance, err)
	}

	appID, err := r.updateApplication(instance, app)
	if err != nil {
		klog.Error("Failed to modify application with error: ", err)
		return r.concludeReconcile(instance, err)
	}

	err = r.generateHybridDeployables(instance, appID)
	if err != nil {
		klog.Error("Failed to generate hybriddeployables for application with error: ", err)
		return r.concludeReconcile(instance, err)
	}

	return r.concludeReconcile(instance, err)
}

func (r *ReconcileApplicationAssembler) concludeReconcile(instance *corev1alpha1.ApplicationAssembler, err error) (reconcile.Result, error) {
	if err != nil {
		instance.Status.Phase = corev1alpha1.ApplicationAssemblerPhaseFailed
		instance.Status.Reason = err.Error()
	} else {
		instance.Status.Phase = ""
		instance.Status.Reason = ""
		instance.Status.Message = ""
	}

	now := metav1.Now()
	instance.Status.LastUpdateTime = &now

	updateErr := r.Status().Update(context.TODO(), instance)
	if updateErr != nil {
		klog.Error("Failed to update status error: ", updateErr)
		// latest error , reconcile on this one as well
		err = updateErr
	}

	klog.V(packageInfoLogLevel).Info("Reconcile ApplicationAssembler:", instance.GetNamespace()+"/"+instance.GetName(), " finished with error:", err)

	return reconcile.Result{}, err

}
