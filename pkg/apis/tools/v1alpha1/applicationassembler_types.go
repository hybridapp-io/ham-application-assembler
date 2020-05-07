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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	hdplv1alpha1 "github.com/hybridapp-io/ham-deployable-operator/pkg/apis/core/v1alpha1"
	dplv1 "github.com/open-cluster-management/multicloud-operators-deployable/pkg/apis/apps/v1"
)

var (
	// AnnotationDiscover defines the annotation used to indicate whether an application should be marked for discovery
	AnnotationDiscover = hdplv1alpha1.SchemeGroupVersion.Group + "/hybrid-discovered"

	//AnnotationClusterScope indicates whether discovery should look for resources cluster wide rather then in a specific namespace
	AnnotationClusterScope = hdplv1alpha1.SchemeGroupVersion.Group + "/hybrid-discover-clusterscoped"

	// AnnotationCreateAssembler defines the annotation used to indicate whether the discovery process should also create an application assembler CR.
	AnnotationCreateAssembler = SchemeGroupVersion.Group + "/hybrid-discover-create-assembler"

	// LabelApplicationPrefix defines the label prefix used as component selector
	LabelApplicationPrefix = SchemeGroupVersion.Group + "/application-"

	//HybridDeployableGK represents the GroupVersionKind structure for a hybrid deployable
	HybridDeployableGK = metav1.GroupKind{
		Group: hdplv1alpha1.SchemeGroupVersion.Group,
		Kind:  "Deployable",
	}

	//DeployableGVK represents the GroupVersionKind structure for a deployable
	DeployableGVK = schema.GroupVersionKind{
		Group:   dplv1.SchemeGroupVersion.Group,
		Version: dplv1.SchemeGroupVersion.Version,
		Kind:    "Deployable",
	}
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// DefaultDeployerType is the default type of a deployer, used when no explicit deployer type is provided
	DefaultDeployerType = "kubernetes"

	// DiscoveryEnabled indicates whether the discovery is enabled for an application CR
	DiscoveryEnabled = "true"

	//AssemblerCreationCompleted indicates the process of creating the assembler CR has finished successfully
	AssemblerCreationCompleted = "completed"
)

// ClusterComponent defines a list of components for a managed cluster identified by its namespace
type ClusterComponent struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Cluster    string                    `json:"cluster"`
	Components []*corev1.ObjectReference `json:"components,omitempty"`
}

// ApplicationAssemblerSpec defines the desired state of ApplicationAssembler
type ApplicationAssemblerSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	HubComponents             []*corev1.ObjectReference `json:"hubComponents,omitempty"`
	ManagedClustersComponents []*ClusterComponent       `json:"managedClustersComponents,omitempty"`
}

// ApplicationAssemblerPhase defines the application assembler phase
type ApplicationAssemblerPhase string

const (
	// ApplicationAssemblerPhaseUnknown means not processed by controller yet
	ApplicationAssemblerPhaseUnknown ApplicationAssemblerPhase = ""
	// ApplicationAssemblerPhaseCompleted means successfully generated hybrid application resources
	ApplicationAssemblerPhaseCompleted ApplicationAssemblerPhase = "Completed"
	// ApplicationAssemblerPhaseFailed means failed to generate hybrid application resources
	ApplicationAssemblerPhaseFailed ApplicationAssemblerPhase = "Failed"
)

// ApplicationAssemblerStatus defines the observed state of ApplicationAssembler
type ApplicationAssemblerStatus struct {
	Phase          ApplicationAssemblerPhase `json:"phase"`
	Reason         string                    `json:"reason,omitempty"`
	Message        string                    `json:"message,omitempty"`
	LastUpdateTime *metav1.Time              `json:"lastUpdateTime,omitempty"`

	HybridDeployables []*corev1.ObjectReference `json:"hybridDeployables,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApplicationAssembler is the Schema for the applicationassemblers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=applicationassemblers,scope=Namespaced
// +kubebuilder:resource:path=applicationassemblers,shortName=appasm
type ApplicationAssembler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationAssemblerSpec   `json:"spec,omitempty"`
	Status ApplicationAssemblerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApplicationAssemblerList contains a list of ApplicationAssembler
type ApplicationAssemblerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationAssembler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationAssembler{}, &ApplicationAssemblerList{})
}
