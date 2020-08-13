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

// HostingAutoscalingPolicySpec defines the desired state of the cluster for HostingAutoscalingPolicy
type HostingAutoscalingPolicySpec struct {
	Region *string `json:"region"`

	MinCapacity *int64 `json:"minCapacity,omitempty"`
	MaxCapacity *int64 `json:"maxCapacity,omitempty"`

	ScalableDimension *string `json:"scalableDimension,omitempty"`

	// The autoscaling policy name. This is optional for the SageMaker K8s operator. If it is empty,
	// the operator will populate it with a generated name.
	// +kubebuilder:validation:MaxLength=256
	PolicyName *string `json:"policyName,omitempty"`

	// The autoscaling policy type. This is optional for the SageMaker K8s operator. If it is empty,
	// the operator will populate it with TargetTrackingScaling
	PolicyType *string `json:"policyType,omitempty"`

	// +kubebuilder:validation:MinItems=1
	ResourceID []*commonv1.AutoscalingResource `json:"resourceId,omitempty"`

	ServiceNamespace *string `json:"serviceNamespace,omitempty"`

	SuspendedState *commonv1.HAPSuspendedState `json:"suspendedState,omitempty"`

	// A custom SageMaker endpoint to use when communicating with SageMaker.
	// +kubebuilder:validation:Pattern="^(https|http)://.*$"
	SageMakerEndpoint                        *string                                     `json:"sageMakerEndpoint,omitempty"`
	TargetTrackingScalingPolicyConfiguration *commonv1.TargetTrackingScalingPolicyConfig `json:"targetTrackingScalingPolicyConfiguration,omitempty"`
}

// HostingAutoscalingPolicyStatus defines the observed state of HostingAutoscalingPolicy
type HostingAutoscalingPolicyStatus struct {

	// Review: Do we want to add the resourceIDList here
	PolicyName                     string `json:"policyName,omitempty"`
	HostingAutoscalingPolicyStatus string `json:"hostingAutoscalingPolicyStatus,omitempty"`

	// Field to store additional information, for example if
	// we are unable to check the status we update this.
	Additional string `json:"additional,omitempty"`

	// The last time that we checked the status of the job.
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.hostingAutoscalingPolicyStatus"
// +kubebuilder:printcolumn:name="Creation-Time",type="string", format="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:shortName="hap",path="hostingautoscalingpolicies"

// HostingAutoscalingPolicy is the Schema for the HostingAutoscalingPolicy API
type HostingAutoscalingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostingAutoscalingPolicySpec   `json:"spec"`
	Status HostingAutoscalingPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HostingAutoscalingPolicyList contains a list of HostingAutoscalingPolicies
type HostingAutoscalingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostingAutoscalingPolicy `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&HostingAutoscalingPolicy{}, &HostingAutoscalingPolicyList{})
}
