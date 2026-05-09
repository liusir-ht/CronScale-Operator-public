/*
Copyright 2024 liuchong.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CronScaleSpec defines the desired state of CronScale
type CronScaleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of CronScale. Edit cronscale_types.go to remove/update
	Foo            string         `json:"foo,omitempty"`
	Minus          CronScaleMinus `json:"minus,omitempty"`
	Add            CronScaleAdd   `json:"add,omitempty"`
	DeploymentName string         `json:"deploymentName,omitempty"`
	ImagePullTime  string         `json:"imagePullTime,omitempty"`
}

type CronScaleMinus struct {
	//期望副本数
	TargetReplicas int32 `json:"targetReplicas,omitempty"`
	//操作时间
	ScaleTime string `json:"scaleTime,omitempty"`
}
type CronScaleAdd struct {
	//期望副本数
	TargetReplicas int32 `json:"targetReplicas,omitempty"`
	//操作时间
	ScaleTime string `json:"scaleTime,omitempty"`
}

// CronScaleStatus defines the observed state of CronScale
type CronScaleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=cronscales,singular=cronscale,scope=Namespaced,shortName=crs

// CronScale is the Schema for the cronscales API
type CronScale struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronScaleSpec   `json:"spec,omitempty"`
	Status CronScaleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CronScaleList contains a list of CronScale
type CronScaleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronScale `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CronScale{}, &CronScaleList{})
}
