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

// HostingDeploymentSpec defines the desired state of HostingDeployment
type HostingDeploymentSpec struct {

	// +kubebuilder:validation:MinLength=1
	Region *string `json:"region"`

	KmsKeyId *string `json:"kmsKeyId,omitempty"`

	// +kubebuilder:validation:MinItems=1
	ProductionVariants []commonv1.ProductionVariant `json:"productionVariants"`

	Models []commonv1.Model `json:"models"`

	Containers []commonv1.ContainerDefinition `json:"containers"`

	Tags []commonv1.Tag `json:"tags,omitempty"`
}

// HostingDeploymentStatus defines the observed state of HostingDeployment
type HostingDeploymentStatus struct {
	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateEndpoint.html#SageMaker-CreateEndpoint-request-EndpointName
	EndpointName string `json:"endpointName,omitempty"`

	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateEndpoint.html#SageMaker-CreateEndpoint-request-EndpointConfigName
	EndpointConfigName string `json:"endpointConfigName,omitempty"`

	EndpointUrl string `json:"endpointUrl,omitempty"`

	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeEndpoint.html#SageMaker-DescribeEndpoint-response-EndpointStatus
	EndpointStatus string `json:"endpointStatus,omitempty"`

	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeEndpoint.html#SageMaker-DescribeEndpoint-response-EndpointArn
	EndpointArn string `json:"endpointArn,omitempty"`

	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeEndpoint.html#SageMaker-DescribeEndpoint-response-CreationTime
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeEndpoint.html#SageMaker-DescribeEndpoint-response-FailureReason
	FailureReason string `json:"failureReason,omitempty"`

	// This field contains additional information about failures.
	Additional string `json:"additional,omitempty"`

	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeEndpoint.html#API_DescribeEndpoint_ResponseSyntax
	LastModifiedTime *metav1.Time `json:"lastModifiedTime,omitempty"`

	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_ProductionVariantSummary.html
	ProductionVariants []*commonv1.ProductionVariantSummary `json:"productionVariants,omitempty"`

	ModelNames []*commonv1.KeyValuePair `json:"modelNames,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.endpointStatus"
// +kubebuilder:printcolumn:name="Sagemaker-endpoint-name",type="string", JSONPath=".status.endpointName"
// HostingDeployment is the Schema for the hostingdeployments API
type HostingDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostingDeploymentSpec   `json:"spec,omitempty"`
	Status HostingDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HostingDeploymentList contains a list of HostingDeployment
type HostingDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostingDeployment `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&HostingDeployment{}, &HostingDeploymentList{})
}
