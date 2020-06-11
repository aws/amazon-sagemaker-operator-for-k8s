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

// These annotations signal to the generator that there are types in this file
// that need DeepCopy methods.
// +kubebuilder:object:generate=true
// +groupName=sagemaker.aws.amazon.com
package v1

import (
	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BatchTransformJobSpec defines the desired state of BatchTransformJob
type BatchTransformJobSpec struct {
	// The SageMaker batchtransform job name. This is optional for the SageMaker K8s operator. If it is empty,
	// the operator will populate it with a generated name.
	// +kubebuilder:validation:MaxLength=63
	TransformJobName *string `json:"transformJobName,omitempty"`

	BatchStrategy commonv1.BatchStrategy `json:"batchStrategy,omitempty"`

	DataProcessing *commonv1.DataProcessing `json:"dataProcessing,omitempty"`

	Environment []*commonv1.KeyValuePair `json:"environment,omitempty"`

	MaxConcurrentTransforms *int64 `json:"maxConcurrentTransforms,omitempty"`

	MaxPayloadInMB *int64 `json:"maxPayloadInMB,omitempty"`

	ModelName *string `json:"modelName"`

	Tags []commonv1.Tag `json:"tags,omitempty"`

	TransformInput *commonv1.TransformInput `json:"transformInput"`

	TransformOutput *commonv1.TransformOutput `json:"transformOutput"`

	TransformResources *commonv1.TransformResources `json:"transformResources"`

	// +kubebuilder:validation:MinLength=1
	Region *string `json:"region"`

	// A custom SageMaker endpoint to use when communicating with SageMaker.
	// +kubebuilder:validation:Pattern=^(https|http)://.*$
	SageMakerEndpoint *string `json:"sageMakerEndpoint,omitempty"`
}

// BatchTransformJobStatus defines the observed state of BatchTransformJob
type BatchTransformJobStatus struct {
	// The status of the transform job.
	// https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeTransformJob.html
	TransformJobStatus string `json:"transformJobStatus,omitempty"`

	// Field to store additional information, for example if
	// we are unable to check the status we update this.
	Additional string `json:"additional,omitempty"`

	// The last time that we checked the status of the SageMaker job.
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	//SageMaker TransformJobName job name
	SageMakerTransformJobName string `json:"sageMakerTransformJobName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string", JSONPath=".status.transformJobStatus"
// +kubebuilder:printcolumn:name="Creation-Time",type="string", format="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Sagemaker-Job-Name",type="string", JSONPath=".status.sageMakerTransformJobName"
// TODO: Add CloudWatch stream URL to the status

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BatchTransformJob is the Schema for the batchtransformjobs API
type BatchTransformJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BatchTransformJobSpec   `json:"spec,omitempty"`
	Status BatchTransformJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BatchTransformJobList contains a list of BatchTransformJob
type BatchTransformJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BatchTransformJob `json:"items"`
}

func init() {
	commonv1.SchemeBuilder.Register(&BatchTransformJob{}, &BatchTransformJobList{})
}
