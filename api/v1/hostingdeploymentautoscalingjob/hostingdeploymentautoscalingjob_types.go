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

// HostingDeploymentAutoscalingJobSpec defines the desired state of the cluster for HostingDeploymentAutoscalingJob
type HostingDeploymentAutoscalingJobSpec struct {
	Region *string `json:"region"`

	MinCapacity *int64 `json:"minCapacity,omitempty"`
	MaxCapacity *int64 `json:"maxCapacity,omitempty"`

	ScalableDimension *string `json:"scalableDimension,omitempty"`

	// The autoscaling policy name. This is optional for the SageMaker K8s operator. If it is empty,
	// the operator will populate it with a generated name.
	// +kubebuilder:validation:MaxLength=63
	PolicyName *string `json:"policyName,omitempty"`

	// The autoscaling policy type. This is optional for the SageMaker K8s operator. If it is empty,
	// the operator will populate it with TargetTrackingScaling
	PolicyType *string `json:"policyType,omitempty"`

	// TODO: should this be a pointer instead
	ResourceID []*commonv1.AutoscalingResource `json:"resourceId,omitempty"`

	ServiceNamespace *string `json:"serviceNamespace,omitempty"`

	// A custom SageMaker endpoint to use when communicating with SageMaker.
	// +kubebuilder:validation:Pattern=^(https|http)://.*$
	SageMakerEndpoint                        *string                                     `json:"sageMakerEndpoint,omitempty"`
	TargetTrackingScalingPolicyConfiguration *commonv1.TargetTrackingScalingPolicyConfig `json:"targetTrackingScalingPolicyConfiguration,omitempty"`
}

// HostingDeploymentAutoscalingJobStatus defines the observed state of HostingDeploymentAutoscalingJob
type HostingDeploymentAutoscalingJobStatus struct {

	// Review: Do we want to add the resourceIDList here
	PolicyName                            string `json:"policyName,omitempty"`
	HostingDeploymentAutoscalingJobStatus string `json:"hostingDeploymentAutoscalingJobStatus,omitempty"`

	// Field to store additional information, for example if
	// we are unable to check the status we update this.
	Additional string `json:"additional,omitempty"`

	// The last time that we checked the status of the job.
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.hostingDeploymentAutoscalingJobStatus"
// +kubebuilder:printcolumn:name="Creation-Time",type="string", format="date",JSONPath=".metadata.creationTimestamp"

// HostingDeploymentAutoscalingJob is the Schema for the hostingdeploymentautoscalingjobs API
type HostingDeploymentAutoscalingJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostingDeploymentAutoscalingJobSpec   `json:"spec"`
	Status HostingDeploymentAutoscalingJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HostingDeploymentAutoscalingJobList contains a list of HostingDeploymentAutoscalingJob
type HostingDeploymentAutoscalingJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostingDeploymentAutoscalingJob `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&HostingDeploymentAutoscalingJob{}, &HostingDeploymentAutoscalingJobList{})
}
