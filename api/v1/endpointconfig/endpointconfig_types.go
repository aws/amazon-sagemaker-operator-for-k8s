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
	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EndpointConfigSpec defines the desired state of EndpointConfig
type EndpointConfigSpec struct {

	// +kubebuilder:validation:MinItems=1
	ProductionVariants []commonv1.ProductionVariant `json:"productionVariants"`

	KmsKeyId string `json:"kmsKeyId,omitempty"`

	Tags []commonv1.Tag `json:"tags,omitempty"`

	Region *string `json:"region"`

	SageMakerEndpoint *string `json:"sageMakerEndpoint,omitempty"`
}

// EndpointConfigStatus defines the observed state of EndpointConfig
type EndpointConfigStatus struct {

	// The status of the EndpointConfig
	Status string `json:"status,omitempty"`

	// The name of the EndpointConfig in SageMaker.
	SageMakerEndpointConfigName string `json:"sageMakerEndpointConfigName,omitempty"`

	// The last time this status was updated.
	LastCheckTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// The EndpointConfig ARN of the SageMaker EndpointConfig
	EndpointConfigArn string `json:"endpointConfigArn,omitempty"`

	// Field to store additional information, for example if
	// we are unable to check the status in sagemaker we update this.
	Additional string `json:"additional,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Sage-Maker-EndpointConfig-Name",type="string", JSONPath=".status.sageMakerEndpointConfigName"

// EndpointConfig is the Schema for the hostingdeployments API
type EndpointConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EndpointConfigSpec   `json:"spec,omitempty"`
	Status EndpointConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EndpointConfigList contains a list of EndpointConfig
type EndpointConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EndpointConfig `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&EndpointConfig{}, &EndpointConfigList{})
}
