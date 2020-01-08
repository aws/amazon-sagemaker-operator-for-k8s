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

// TrainingJobSpec defines the desired state of TrainingJob
type TrainingJobSpec struct {
	AlgorithmSpecification *commonv1.AlgorithmSpecification `json:"algorithmSpecification"`

	EnableInterContainerTrafficEncryption *bool `json:"enableInterContainerTrafficEncryption,omitempty"`

	EnableNetworkIsolation *bool `json:"enableNetworkIsolation,omitempty"`

	EnableManagedSpotTraining *bool `json:"enableManagedSpotTraining,omitempty"`

	HyperParameters []*commonv1.KeyValuePair `json:"hyperParameters,omitempty"`

	// +kubebuilder:validation:MinItems=1
	InputDataConfig []commonv1.Channel `json:"inputDataConfig,omitempty"`

	OutputDataConfig *commonv1.OutputDataConfig `json:"outputDataConfig"`

	CheckpointConfig *commonv1.CheckpointConfig `json:"checkpointConfig,omitempty"`

	ResourceConfig *commonv1.ResourceConfig `json:"resourceConfig"`

	// +kubebuilder:validation:MinLength=20
	RoleArn *string `json:"roleArn"`

	// +kubebuilder:validation:MinLength=1
	Region *string `json:"region"`

	// A custom SageMaker endpoint to use when communicating with SageMaker.
	// +kubebuilder:validation:Pattern=^(https|http)://.*$
	SageMakerEndpoint *string `json:"sageMakerEndpoint,omitempty"`

	StoppingCondition *commonv1.StoppingCondition `json:"stoppingCondition"`

	Tags []commonv1.Tag `json:"tags,omitempty"`

	// The SageMaker training job name. This is optional for the SageMaker K8s operator. If it is empty,
	// the operator will populate it with a generated name.
	// +kubebuilder:validation:MaxLength=63
	TrainingJobName *string `json:"trainingJobName,omitempty"`

	VpcConfig *commonv1.VpcConfig `json:"vpcConfig,omitempty"`
}

// TrainingJobStatus defines the observed state of TrainingJob
type TrainingJobStatus struct {
	// The status of the training job.
	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeTrainingJob.html#SageMaker-DescribeTrainingJob-response-TrainingJobStatus
	TrainingJobStatus string `json:"trainingJobStatus,omitempty"`

	// The secondary, more granular status of the training job.
	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeTrainingJob.html#SageMaker-DescribeTrainingJob-response-SecondaryStatus
	SecondaryStatus string `json:"secondaryStatus,omitempty"`

	// Field to store additional information, for example if
	// we are unable to check the status we update this.
	Additional string `json:"additional,omitempty"`

	// The last time that we checked the status of the SageMaker job.
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// Cloud Watch url for training log
	CloudWatchLogUrl string `json:"cloudWatchLogUrl,omitempty"`

	//SageMaker training job name
	SageMakerTrainingJobName string `json:"sageMakerTrainingJobName,omitempty"`

	//Full path to the training artifact (model)
	ModelPath string `json:"modelPath,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.trainingJobStatus"
// +kubebuilder:printcolumn:name="Secondary-Status",type="string", JSONPath=".status.secondaryStatus"
// +kubebuilder:printcolumn:name="Creation-Time",type="string", format="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Sagemaker-Job-Name",type="string", JSONPath=".status.sageMakerTrainingJobName"

// TrainingJob is the Schema for the trainingjobs API
type TrainingJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrainingJobSpec   `json:"spec"`
	Status TrainingJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TrainingJobList contains a list of TrainingJob
type TrainingJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrainingJob `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&TrainingJob{}, &TrainingJobList{})
}
