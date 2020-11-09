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

// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

// ProcessingJobSpec defines the desired state of ProcessingJob
type ProcessingJobSpec struct {
	AppSpecification *commonv1.AppSpecification `json:"appSpecification"`

	// +kubebuilder:validation:MaxItems=100
	Environment []*commonv1.KeyValuePair `json:"environment,omitempty"`

	NetworkConfig *commonv1.ProcessingNetworkConfig `json:"networkConfig,omitempty"`

	// +kubebuilder:validation:MaxItems=10
	ProcessingInputs []*commonv1.ProcessingInput `json:"processingInputs,omitempty"`

	ProcessingOutputConfig *commonv1.ProcessingOutputConfig `json:"processingOutputConfig,omitempty"`

	ProcessingResources *commonv1.ProcessingResources `json:"processingResources"`

	// +kubebuilder:validation:MinLength=20
	// +kubebuilder:validation:MaxLength=2048
	RoleArn *string `json:"roleArn"`

	// +kubebuilder:validation:MinLength=1
	Region *string `json:"region"`

	StoppingCondition *commonv1.StoppingConditionNoSpot `json:"stoppingCondition,omitempty"`

	// +kubebuilder:validation:MaxItems=50
	Tags []*commonv1.Tag `json:"tags,omitempty"`

	// A custom SageMaker endpoint to use when communicating with SageMaker.
	// +kubebuilder:validation:Pattern="^(https|http)://.*$"
	SageMakerEndpoint *string `json:"sageMakerEndpoint,omitempty"`
}

// ProcessingJobStatus defines the observed state of ProcessingJob
type ProcessingJobStatus struct {

	// The status of the processing job.
	// https://docs.aws.amazon.com/sagemaker/latest/APIReference/API_DescribeProcessingJob.html#sagemaker-DescribeProcessingJob-response-ProcessingJobStatus
	ProcessingJobStatus string `json:"processingJobStatus,omitempty"`

	// Field to store additional information, for example if
	// we are unable to check the status we update this.
	Additional string `json:"additional,omitempty"`

	// The last time that we checked the status of the SageMaker job.
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// CloudWatch URL for log
	CloudWatchLogURL string `json:"cloudWatchLogUrl,omitempty"`

	//SageMaker processing job name
	SageMakerProcessingJobName string `json:"sageMakerProcessingJobName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.processingJobStatus"
// +kubebuilder:printcolumn:name="Creation-Time",type="string", format="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Sagemaker-Job-Name",type="string", JSONPath=".status.sageMakerProcessingJobName"

// ProcessingJob is the Schema for the processingjobs API
type ProcessingJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProcessingJobSpec   `json:"spec,omitempty"`
	Status ProcessingJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProcessingJobList contains a list of ProcessingJob
type ProcessingJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProcessingJob `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&ProcessingJob{}, &ProcessingJobList{})
}
