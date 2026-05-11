/*
Copyright 2024 liuchong.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
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
