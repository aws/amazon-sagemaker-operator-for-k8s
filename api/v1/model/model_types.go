/*
Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.

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

// This annotation signals to the generator that there are types in this file
// that need DeepCopy methods.
// +kubebuilder:object:generate=true

// This annotation signals that the types in this file should be a CRD in the group.
// +groupName=sagemaker.aws.amazon.com

package v1

import (
	commonv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec defines the desired state of Model
type ModelSpec struct {
	Containers []*commonv1.ContainerDefinition `json:"containers,omitempty"`

	EnableNetworkIsolation *bool `json:"enableNetworkIsolation,omitempty"`

	ExecutionRoleArn *string `json:"executionRoleArn"`

	PrimaryContainer *commonv1.ContainerDefinition `json:"primaryContainer,omitempty"`

	Tags []commonv1.Tag `json:"tags,omitempty"`

	VpcConfig *commonv1.VpcConfig `json:"vpcConfig,omitempty"`

	Region *string `json:"region"`

	SageMakerEndpoint *string `json:"sageMakerEndpoint,omitempty"`
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {

	// The status of the model.
	Status string `json:"status,omitempty"`

	// The name of the model in SageMaker.
	SageMakerModelName string `json:"sageMakerModelName,omitempty"`

	// The last time this status was updated.
	LastCheckTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// The Model ARN of the SageMaker model
	ModelArn string `json:"modelArn,omitempty"`

	// Field to store additional information, for example if
	// we are unable to check the status in sagemaker we update this.
	Additional string `json:"additional,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Sage-Maker-Model-Name",type="string", JSONPath=".status.sageMakerModelName"

// Model is the Schema for the hostingdeployments API
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelSpec   `json:"spec,omitempty"`
	Status ModelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ModelList contains a list of Model
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&Model{}, &ModelList{})
}
