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

package utils

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

const (
	packageDetailLogLevel = 5
)

var (
	GVKGVRMap         map[schema.GroupVersionKind]schema.GroupVersionResource
	resourcePredicate = discovery.SupportsAllVerbs{Verbs: []string{"create", "update", "delete", "list", "watch"}}
)

func addResourceToMap(rl *metav1.APIResourceList,
	resMap map[schema.GroupVersionKind]schema.GroupVersionResource) map[schema.GroupVersionKind]schema.GroupVersionResource {
	for _, res := range rl.APIResources {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			klog.V(packageDetailLogLevel).Info("Skipping ", rl.GroupVersion, " with error:", err)
			continue
		}

		gvk := schema.GroupVersionKind{
			Kind:    res.Kind,
			Group:   gv.Group,
			Version: gv.Version,
		}
		gvr := schema.GroupVersionResource{
			Group:    gv.Group,
			Version:  gv.Version,
			Resource: res.Name,
		}

		resMap[gvk] = gvr
	}

	return resMap
}

// BuildGVKGVRMap builds or returns a cached GVK to GVR map
func BuildGVKGVRMap(config *rest.Config) map[schema.GroupVersionKind]schema.GroupVersionResource {
	if GVKGVRMap != nil {
		return GVKGVRMap
	}
	return RebuildGVKGVRMap(config)
}

// RebuildGVKGVRMap builds a GVK to GVR map from scratch
func RebuildGVKGVRMap(config *rest.Config) map[schema.GroupVersionKind]schema.GroupVersionResource {
	resources, err := discovery.NewDiscoveryClientForConfigOrDie(config).ServerPreferredResources()
	if err != nil {
		klog.Error("Failed to discover all server resources, continuing with err:", err)
		return nil
	}

	filteredResources := discovery.FilteredBy(resourcePredicate, resources)

	klog.V(packageDetailLogLevel).Info("Discovered resources: ", filteredResources)

	resMap := make(map[schema.GroupVersionKind]schema.GroupVersionResource)

	for _, rl := range filteredResources {
		resMap = addResourceToMap(rl, resMap)
	}
	//delayed initialization
	GVKGVRMap = resMap
	return GVKGVRMap
}

// StripVersion removes the version part of a GV
func StripVersion(gv string) string {
	if gv == "" {
		return gv
	}

	re := regexp.MustCompile(`^[vV][0-9].*`)
	// If it begins with only version, (group is nil), return empty string which maps to core group
	if re.MatchString(gv) {
		return ""
	}

	return strings.Split(gv, "/")[0]
}

// GetAPIVersion returns the APIVersion from a RESTMapping
func GetAPIVersion(mapping *meta.RESTMapping) string {
	var gvk = mapping.GroupVersionKind
	if gvk.Group == "" {
		return gvk.Version
	}
	return gvk.Group + "/" + gvk.Version

}

// ConvertLabel converts a LabelSelector struct into a Selector interface
func ConvertLabel(labelSelector *metav1.LabelSelector) (labels.Selector, error) {
	if labelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(labelSelector)

		if err != nil {
			return labels.Nothing(), err
		}

		return selector, nil
	}

	return labels.Everything(), nil
}
