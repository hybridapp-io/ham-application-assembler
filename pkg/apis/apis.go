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

package apis

import (
	sigappapis "github.com/kubernetes-sigs/application/pkg/apis"
	crds "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	hdplapis "github.com/hybridapp-io/ham-deployable-operator/pkg/apis"
	hprlapis "github.com/hybridapp-io/ham-placement/pkg/apis"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	manifestwork "github.com/open-cluster-management/api/work/v1"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	var err error

	err = sigappapis.AddToScheme(s)
	if err != nil {
		return err
	}

	err = hdplapis.AddToScheme(s)
	if err != nil {
		return err
	}

	err = crds.AddToScheme(s)
	if err != nil {
		return err
	}

	err = hprlapis.AddToScheme(s)
	if err != nil {
		return err
	}

	err = manifestwork.AddToScheme(s)
	if err != nil {
		return err
	}
	err = managedclusterv1.AddToScheme(s)
	if err != nil {
		return err
	}

	return AddToSchemes.AddToScheme(s)
}
