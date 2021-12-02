/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ElasticWebSpec defines the desired state of ElasticWeb
type ElasticWebSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ElasticWeb. Edit elasticweb_types.go to remove/update
	// service image
	Image string `json:"image"`
	// service port
	Port *int32 `json:"port"`
	// single pod qps
	// +kubebuilder:validation:Minimum=1000
	// +kubebuilder:validation:Maximum=2000
	SinglePodQPS *int32 `json:"singlePodQPS"`
	// total qps
	TotalQPS *int32 `json:"totalQPS"`
}

// ElasticWebStatus defines the observed state of ElasticWeb
type ElasticWebStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	RealQPS *int32 `json:"realQPS"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ElasticWeb is the Schema for the elasticwebs API
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Port",type=integer,JSONPath=`.spec.port`
// +kubebuilder:printcolumn:name="SinglePodQPS",type=integer,JSONPath=`.spec.singlePodQPS`
// +kubebuilder:printcolumn:name="TotalQPS",type=integer,JSONPath=`.spec.totalQPS`
// +kubebuilder:printcolumn:name="RealQPS",type=integer,JSONPath=`.status.realQPS`
// +kubebuilder:subresource:status
type ElasticWeb struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElasticWebSpec   `json:"spec,omitempty"`
	Status ElasticWebStatus `json:"status,omitempty"`
}

func (e *ElasticWeb) String() string {
	var realQPS string
	if e.Status.RealQPS == nil {
		realQPS = "nil"
	} else {
		realQPS = strconv.Itoa(int(*(e.Status.RealQPS)))
	}
	return fmt.Sprintf("Image [%s]. Port [%d], SinglePodQPS [%d], TotalQPS [%d], RealQPS [%s]",
		e.Spec.Image, *(e.Spec.Port), *(e.Spec.SinglePodQPS), *(e.Spec.TotalQPS), realQPS)
}

//+kubebuilder:object:root=true

// ElasticWebList contains a list of ElasticWeb
type ElasticWebList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElasticWeb `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ElasticWeb{}, &ElasticWebList{})
}
