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

// HyperparameterTuningJobSpec defines the desired state of HyperparameterTuningJob
// These are taken from aws-go-sdk-v2 and modified to use Kubebuilder validation and json omitempty instead of
// aws-go-sdk-v2 validation and required parameter notation, respectively.
type HyperparameterTuningJobSpec struct {
	HyperParameterTuningJobConfig *commonv1.HyperParameterTuningJobConfig `json:"hyperParameterTuningJobConfig"`

	HyperParameterTuningJobName *string `json:"hyperParameterTuningJobName,omitempty"`

	Tags []commonv1.Tag `json:"tags,omitempty"`

	TrainingJobDefinition *commonv1.HyperParameterTrainingJobDefinition `json:"trainingJobDefinition,omitempty"`

	WarmStartConfig *commonv1.HyperParameterTuningJobWarmStartConfig `json:"warmStartConfig,omitempty"`

	// +kubebuilder:validation:MinLength=1
	Region *string `json:"region"`

	// A custom SageMaker endpoint to use when communicating with SageMaker.
	// +kubebuilder:validation:Pattern=^(https|http)://.*$
	SageMakerEndpoint *string `json:"sageMakerEndpoint,omitempty"`
}

// HyperparameterTuningJobStatus defines the observed state of HyperparameterTuningJob
type HyperparameterTuningJobStatus struct {
	// Field to store additional information, for example if
	// we are unable to check the status we update this.
	Additional string `json:"additional,omitempty"`

	// The status of HyperParameterTrainingJob
	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeHyperParameterTuningJob.html#SageMaker-DescribeHyperParameterTuningJob-response-HyperParameterTuningJobStatus
	HyperParameterTuningJobStatus string `json:"hyperParameterTuningJobStatus,omitempty"`

	// A HyperParameterTrainingJobSummary object that describes the training job that completed with the best current HyperParameterTuningJobObjective.
	// See https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeHyperParameterTuningJob.html#SageMaker-DescribeHyperParameterTuningJob-response-BestTrainingJob
	BestTrainingJob *commonv1.HyperParameterTrainingJobSummary `json:"bestTrainingJob,omitempty"`

	// The last time that we checked the status of the SageMaker job.
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	//SageMaker hyperparametertuning job name
	SageMakerHyperParameterTuningJobName string `json:"sageMakerHyperParameterTuningJobName,omitempty"`

	// The TrainingJobStatusCounters object that specifies the number of training
	// jobs, categorized by status, that this tuning job launched.
	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_TrainingJobStatusCounters.html
	TrainingJobStatusCounters *commonv1.TrainingJobStatusCounters `json:"trainingJobStatusCounters,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.hyperParameterTuningJobStatus"
// +kubebuilder:printcolumn:name="Creation-Time",type="string", format="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Completed",type="number", format="int64", JSONPath=".status.trainingJobStatusCounters.completed"
// +kubebuilder:printcolumn:name="InProgress",type="number", format="int64", JSONPath=".status.trainingJobStatusCounters.inProgress"
// +kubebuilder:printcolumn:name="Errors",type="number", format="int64", JSONPath=".status.trainingJobStatusCounters.totalError"
// +kubebuilder:printcolumn:name="Stopped",type="number", format="int64", JSONPath=".status.trainingJobStatusCounters.stopped"
// +kubebuilder:printcolumn:name="Best-Training-Job",type="string", JSONPath=".status.bestTrainingJob.trainingJobName"
// +kubebuilder:printcolumn:name="Sagemaker-Job-Name",type="string", JSONPath=".status.sageMakerHyperParameterTuningJobName"

// HyperparameterTuningJob is the Schema for the hyperparametertuningjobs API
type HyperparameterTuningJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HyperparameterTuningJobSpec   `json:"spec,omitempty"`
	Status HyperparameterTuningJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HyperparameterTuningJobList contains a list of HyperparameterTuningJob
type HyperparameterTuningJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HyperparameterTuningJob `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&HyperparameterTuningJob{}, &HyperparameterTuningJobList{})
}
