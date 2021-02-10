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
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AlgorithmSpecification struct {

	// +kubebuilder:validation:MinLength=1
	AlgorithmName *string `json:"algorithmName,omitempty"`

	MetricDefinitions []MetricDefinition `json:"metricDefinitions,omitempty"`

	// +kubebuilder:validation:MinLength=1
	TrainingImage *string `json:"trainingImage,omitempty"`

	TrainingInputMode TrainingInputMode `json:"trainingInputMode"`
}

type AppSpecification struct {
	ContainerArguments []string `json:"containerArguments,omitempty"`

	ContainerEntrypoint []string `json:"containerEntrypoint,omitempty"`

	ImageURI string `json:"imageUri,omitempty"`
}

type MetricDefinition struct {
	// +kubebuilder:validation:MinLength=1
	Name *string `json:"name"`

	// +kubebuilder:validation:MinLength=1
	Regex *string `json:"regex"`
}

type Channel struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=[A-Za-z0-9\.\-_]+
	ChannelName *string `json:"channelName"`

	// +kubebuilder:validation:Enum=None;Gzip
	CompressionType string `json:"compressionType,omitempty"`

	ContentType *string `json:"contentType,omitempty"`

	DataSource *DataSource `json:"dataSource"`

	// +kubebuilder:validation:Enum=Pipe;File
	InputMode string `json:"inputMode,omitempty"`

	RecordWrapperType string `json:"recordWrapperType,omitempty"`

	ShuffleConfig *ShuffleConfig `json:"shuffleConfig,omitempty"`
}

type ProcessingInput struct {
	InputName string            `json:"inputName"`
	S3Input   ProcessingS3Input `json:"s3Input"`
}

type ProcessingS3Input struct {
	LocalPath LocalPath `json:"localPath"`

	CompressionType CompressionType `json:"s3CompressionType,omitempty"`

	// +kubebuilder:validation:Enum=FullyReplicated;ShardedByS3Key
	S3DataDistributionType S3DataDistributionType `json:"s3DataDistributionType,omitempty"`

	// +kubebuilder:validation:Enum=S3Prefix;ManifestFile
	S3DataType string `json:"s3DataType"`

	// +kubebuilder:validation:Enum=Pipe;File
	S3InputMode TrainingInputMode `json:"s3InputMode"`

	S3Uri S3Uri `json:"s3Uri"`
}

type ProcessingOutputConfig struct {
	// +kubebuilder:validation:MaxLength=1024
	KmsKeyId string `json:"kmsKeyId,omitempty"`

	// +kubebuilder:validation:MaxItems=10
	Outputs []ProcessingOutputStruct `json:"outputs"`
}

type ProcessingOutputStruct struct {
	OutputName string `json:"outputName"`

	S3Output ProcessingS3Output `json:"s3Output"`
}

type ProcessingS3Output struct {
	LocalPath LocalPath `json:"localPath"`

	// +kubebuilder:validation:Enum=Continuous;EndOfJob
	S3UploadMode string `json:"s3UploadMode"`

	S3Uri S3Uri `json:"s3Uri"`
}

type ProcessingNetworkConfig struct {
	EnableInterContainerTrafficEncryption bool `json:"enableInterContainerTrafficEncryption,omitempty"`

	EnableNetworkIsolation bool `json:"enableNetworkIsolation,omitempty"`

	VpcConfig *VpcConfig `json:"vpcConfig,omitempty"`
}

// +kubebuilder:validation:Enum=FullyReplicated;ShardedByS3Key
type S3DataDistributionType string

// +kubebuilder:validation:MaxLength=256
type LocalPath string

// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
type S3Uri string

type DataSource struct {
	FileSystemDataSource *FileSystemDataSource `json:"fileSystemDataSource,omitempty"`

	S3DataSource *S3DataSource `json:"s3DataSource,omitempty"`
}

type S3DataSource struct {
	AttributeNames []string `json:"attributeNames,omitempty"`

	S3DataDistributionType S3DataDistributionType `json:"s3DataDistributionType,omitempty"`

	// +kubebuilder:validation:Enum=S3Prefix;ManifestFile;AugmentedManifestFile
	S3DataType string `json:"s3DataType"`

	// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
	S3Uri *string `json:"s3Uri"`
}

type FileSystemDataSource struct {
	DirectoryPath *string `json:"directoryPath"`

	FileSystemAccessMode *string `json:"fileSystemAccessMode"`

	FileSystemId *string `json:"fileSystemId"`

	FileSystemType *string `json:"fileSystemType"`
}

type ShuffleConfig struct {
	Seed *int64 `json:"seed"`
}

type OutputDataConfig struct {
	KmsKeyId *string `json:"kmsKeyId,omitempty"`

	// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
	S3OutputPath *string `json:"s3OutputPath"`
}

type CheckpointConfig struct {
	LocalPath *string `json:"localPath,omitempty"`

	// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
	S3Uri *string `json:"s3Uri"`
}

type ResourceConfig struct {

	// +kubebuilder:validation:Minimum=1
	InstanceCount *int64 `json:"instanceCount"`

	// +kubebuilder:validation:MinLength=1
	InstanceType string `json:"instanceType"`

	VolumeKmsKeyId *string `json:"volumeKmsKeyId,omitempty"`

	// +kubebuilder:validation:Minimum=1
	VolumeSizeInGB *int64 `json:"volumeSizeInGB"`
}

type ProcessingResources struct {
	ClusterConfig *ResourceConfig `json:"clusterConfig"`
}

type StoppingCondition struct {
	// +kubebuilder:validation:Minimum=1
	MaxRuntimeInSeconds *int64 `json:"maxRuntimeInSeconds,omitempty"`

	// +kubebuilder:validation:Minimum=1
	MaxWaitTimeInSeconds *int64 `json:"maxWaitTimeInSeconds,omitempty"`
}

// StoppingConditionNoSpot is used for APIs which do not support WaitTime param i.e. managed spot training not supported
type StoppingConditionNoSpot struct {
	// +kubebuilder:validation:Minimum=1
	MaxRuntimeInSeconds *int64 `json:"maxRuntimeInSeconds,omitempty"`
}

type Tag struct {

	// +kubebuilder:validation:MinLength=1
	Key *string `json:"key"`

	Value *string `json:"value"`
}

// Used in describing maps in Kubernetes.
type KeyValuePair struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type VpcConfig struct {
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=5
	SecurityGroupIds []string `json:"securityGroupIds"`

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	Subnets []string `json:"subnets"`
}

type HyperParameterTuningJobConfig struct {
	HyperParameterTuningJobObjective *HyperParameterTuningJobObjective   `json:"hyperParameterTuningJobObjective,omitempty"`
	ParameterRanges                  *ParameterRanges                    `json:"parameterRanges,omitempty"`
	ResourceLimits                   *ResourceLimits                     `json:"resourceLimits"`
	Strategy                         HyperParameterTuningJobStrategyType `json:"strategy"`
	TrainingJobEarlyStoppingType     TrainingJobEarlyStoppingType        `json:"trainingJobEarlyStoppingType,omitempty"`
}

type HyperParameterTuningJobObjective struct {
	// +kubebuilder:validation:MinLength=1
	MetricName *string `json:"metricName"`

	Type HyperParameterTuningJobObjectiveType `json:"type"`
}

type HyperParameterTuningJobObjectiveType string

type ParameterRanges struct {
	CategoricalParameterRanges []CategoricalParameterRange `json:"categoricalParameterRanges,omitempty"`
	ContinuousParameterRanges  []ContinuousParameterRange  `json:"continuousParameterRanges,omitempty"`
	IntegerParameterRanges     []IntegerParameterRange     `json:"integerParameterRanges,omitempty"`
}

type CategoricalParameterRange struct {
	Name *string `json:"name"`

	// +kubebuilder:validation:MinItems=1
	Values []string `json:"values"`
}

type ContinuousParameterRange struct {
	MaxValue    *string                   `json:"maxValue"`
	MinValue    *string                   `json:"minValue"`
	Name        *string                   `json:"name"`
	ScalingType HyperParameterScalingType `json:"scalingType"`
}

type HyperParameterScalingType string

type IntegerParameterRange struct {
	MaxValue    *string                   `json:"maxValue"`
	MinValue    *string                   `json:"minValue"`
	Name        *string                   `json:"name"`
	ScalingType HyperParameterScalingType `json:"scalingType"`
}

type ResourceLimits struct {
	// +kubebuilder:validation:Minimum=1
	MaxNumberOfTrainingJobs *int64 `json:"maxNumberOfTrainingJobs"`

	// +kubebuilder:validation:Minimum=1
	MaxParallelTrainingJobs *int64 `json:"maxParallelTrainingJobs"`
}

type HyperParameterTuningJobStrategyType string

type TrainingJobEarlyStoppingType string

type HyperParameterTrainingJobDefinition struct {
	AlgorithmSpecification                *HyperParameterAlgorithmSpecification `json:"algorithmSpecification"`
	EnableInterContainerTrafficEncryption *bool                                 `json:"enableInterContainerTrafficEncryption,omitempty"`
	EnableNetworkIsolation                *bool                                 `json:"enableNetworkIsolation,omitempty"`
	EnableManagedSpotTraining             *bool                                 `json:"enableManagedSpotTraining,omitempty"`

	// +kubebuilder:validation:MinItems=1
	InputDataConfig  []Channel         `json:"inputDataConfig,omitempty"`
	OutputDataConfig *OutputDataConfig `json:"outputDataConfig"`
	CheckpointConfig *CheckpointConfig `json:"checkpointConfig,omitempty"`
	ResourceConfig   *ResourceConfig   `json:"resourceConfig"`

	// +kubebuilder:validation:MinLength=20
	RoleArn               *string            `json:"roleArn"`
	StaticHyperParameters []*KeyValuePair    `json:"staticHyperParameters,omitempty"`
	StoppingCondition     *StoppingCondition `json:"stoppingCondition"`
	VpcConfig             *VpcConfig         `json:"vpcConfig,omitempty"`
}

type HyperParameterAlgorithmSpecification struct {
	// +kubebuilder:validation:MinLength=1
	AlgorithmName     *string            `json:"algorithmName,omitempty"`
	MetricDefinitions []MetricDefinition `json:"metricDefinitions,omitempty"`
	TrainingImage     *string            `json:"trainingImage,omitempty"`
	TrainingInputMode TrainingInputMode  `json:"trainingInputMode"`
}

// +kubebuilder:validation:Enum=File;Pipe
type TrainingInputMode string

type HyperParameterTuningJobWarmStartConfig struct {
	// +kubebuilder:validation:MinItems=1
	ParentHyperParameterTuningJobs []ParentHyperParameterTuningJob `json:"parentHyperParameterTuningJobs"`

	WarmStartType HyperParameterTuningJobWarmStartType `json:"warmStartType"`
}

type ParentHyperParameterTuningJob struct {
	// +kubebuilder:validation:MinLength=1
	HyperParameterTuningJobName *string `json:"hyperParameterTuningJobName,omitempty"`
}

type HyperParameterTuningJobWarmStartType string

// This is only used in the Status, so no validation is required.
type HyperParameterTrainingJobSummary struct {
	CreationTime *metav1.Time `json:"creationTime,omitempty"`

	FailureReason *string `json:"failureReason,omitempty"`

	FinalHyperParameterTuningJobObjectiveMetric *FinalHyperParameterTuningJobObjectiveMetric `json:"finalHyperParameterTuningJobObjectiveMetric,omitempty"`

	ObjectiveStatus ObjectiveStatus `json:"objectiveStatus,omitempty"`

	TrainingEndTime *metav1.Time `json:"trainingEndTime,omitempty"`

	TrainingJobArn *string `json:"trainingJobArn,omitempty"`

	TrainingJobName *string `json:"trainingJobName,omitempty"`

	TrainingJobStatus string `json:"trainingJobStatus,omitempty"`

	TrainingStartTime *metav1.Time `json:"trainingStartTime,omitempty"`

	TunedHyperParameters []*KeyValuePair `json:"tunedHyperParameters,omitempty"`

	TuningJobName *string `json:"tuningJobName,omitempty"`
}

// This is only used in status, so no validation is required
// The numbers of training jobs launched by a hyperparameter tuning job, categorized
// by status.
// Please also see https://docs.aws.amazon.com/goto/WebAPI/sagemaker-2017-07-24/TrainingJobStatusCounters
// TODO: separate the status and spec structure in two different files and keep the common code here.
type TrainingJobStatusCounters struct {

	// The number of completed training jobs launched by the hyperparameter tuning
	// job.
	Completed *int64 `json:"completed,omitempty"`

	// The number of in-progress training jobs launched by a hyperparameter tuning
	// job.
	InProgress *int64 `json:"inProgress,omitempty"`

	// The number of training jobs that failed and can't be retried. A failed training
	// job can't be retried if it failed because a client error occurred.
	NonRetryableError *int64 `json:"nonRetryableError,omitempty"`

	// The number of training jobs that failed, but can be retried. A failed training
	// job can be retried only if it failed because an internal service error occurred.
	RetryableError *int64 `json:"retryableError,omitempty"`

	// The sum of NonRetryableError and RetryableError.
	// This is unique to the Kubernetes operator and is used to simplify the `kubectl get` output.
	TotalError *int64 `json:"totalError,omitempty"`

	// The number of training jobs launched by a hyperparameter tuning job that
	// were manually stopped.
	Stopped *int64 `json:"stopped,omitempty"`
}

type FinalHyperParameterTuningJobObjectiveMetric struct {
	MetricName *string                              `json:"metricName,omitempty"`
	Type       HyperParameterTuningJobObjectiveType `json:"type,omitempty"`

	// Value is string instead of float64 to prevent bugs when deserializing onto different platforms.
	Value *string `json:"value,omitempty"`
}

type ObjectiveStatus string

// Hosting related types

type ProductionVariant struct {
	AcceleratorType ProductionVariantAcceleratorType `json:"acceleratorType,omitempty"`

	// +kubebuilder:validation:Minimum=1
	InitialInstanceCount *int64 `json:"initialInstanceCount"`

	// We use an int64 here instead of float because floats are not supported
	// by the Kubernetes API.
	// The actual traffic directed to this ProductionVariant is the ratio of this
	// variant weight to the sum of all variant weights.
	InitialVariantWeight *int64 `json:"initialVariantWeight,omitempty"`

	InstanceType ProductionVariantInstanceType `json:"instanceType"`

	// +kubebuilder:validation:MinLength=1
	ModelName *string `json:"modelName"`

	// +kubebuilder:validation:MinLength=1
	VariantName *string `json:"variantName"`
}

type ProductionVariantAcceleratorType string

type ProductionVariantInstanceType string

// Describes the container, as part of model definition.
type ContainerDefinition struct {
	ContainerHostname *string `json:"containerHostname,omitempty"`

	Environment []*KeyValuePair `json:"environment,omitempty"`

	Image *string `json:"image,omitempty"`

	ModelDataUrl *string `json:"modelDataUrl,omitempty"`

	// +kubebuilder:validation:Enum=SingleModel;MultiModel
	Mode *string `json:"mode,omitempty"`

	ModelPackageName *string `json:"modelPackageName,omitempty"`
}

// This is something we are defining not coming from aws-sdk-go-v2
type Model struct {
	Name *string `json:"name"`

	// +kubebuilder:validation:MinItems=1
	Containers []*ContainerDefinition `json:"containers,omitempty"`

	// Primary container will be ignored if more than one container in the `containers`
	// field is provided.
	PrimaryContainer *string `json:"primaryContainer,omitempty"`

	// +kubebuilder:validation:MinLength=20
	ExecutionRoleArn *string `json:"executionRoleArn"`

	EnableNetworkIsolation *bool `json:"enableNetworkIsolation,omitempty"`

	VpcConfig *VpcConfig `json:"vpcConfig,omitempty"`
}

// Please also see https://docs.aws.amazon.com/goto/WebAPI/sagemaker-2017-07-24/ProductionVariantSummary
// This is only used in status so no validation is required
type ProductionVariantSummary struct {
	CurrentInstanceCount *int64 `json:"currentInstanceCount,omitempty"`

	// We use an int64 here instead of float because floats are not supported
	// by the Kubernetes API.
	// The actual traffic directed to this ProductionVariant is the ratio of this
	// variant weight to the sum of all variant weights.
	CurrentWeight *int64 `json:"currentWeight,omitempty"`

	DeployedImages []DeployedImage `json:"deployedImages,omitempty"`

	DesiredInstanceCount *int64 `json:"desiredInstanceCount,omitempty"`

	// We use an int64 here instead of float because floats are not supported
	// by the Kubernetes API.
	// The actual traffic directed to this ProductionVariant is the ratio of this
	// variant weight to the sum of all variant weights.
	DesiredWeight *int64 `json:"desiredWeight,omitempty"`

	VariantName *string `json:"variantName"`
}

type DeployedImage struct {
	ResolutionTime *metav1.Time `json:"resolutionTime,omitempty"`

	ResolvedImage *string `json:"resolvedImage,omitempty"`

	SpecifiedImage *string `json:"specifiedImage,omitempty"`
}

type VariantProperty struct {
	// +kubebuilder:validation:Enum=DesiredInstanceCount;DesiredWeight;DataCaptureConfig
	VariantPropertyType *string `json:"variantPropertyType"`
}

// Batch Transform related struct
type BatchStrategy string

type DataProcessing struct {
	InputFilter *string `json:"inputFilter,omitempty"`

	JoinSource JoinSource `json:"JoinSource,omitempty"`

	OutputFilter *string `json:"OutputFilter,omitempty"`
}

type JoinSource string

type TransformInput struct {
	CompressionType CompressionType `json:"compressionType,omitempty"`

	ContentType *string `json:"contentType,omitempty"`

	DataSource *TransformDataSource `json:"dataSource"`

	SplitType SplitType `json:"splitType,omitempty"`
}

// +kubebuilder:validation:Enum=None;Gzip
type CompressionType string

type TransformDataSource struct {
	S3DataSource *TransformS3DataSource `json:"s3DataSource"`
}

type TransformS3DataSource struct {
	// +kubebuilder:validation:Enum=S3Prefix;ManifestFile;AugmentedManifestFile
	S3DataType S3DataType `json:"s3DataType"`

	// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
	S3Uri *string `json:"s3Uri"`
}

// +kubebuilder:validation:Enum=S3Prefix;ManifestFile;AugmentedManifestFile
type S3DataType string

type SplitType string

type TransformOutput struct {
	Accept *string `json:"accept,omitempty"`

	AssembleWith AssemblyType `json:"assembleWith,omitempty"`

	KmsKeyId *string `json:"kmsKeyId,omitempty"`

	// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
	S3OutputPath *string `json:"s3OutputPath"`
}

type AssemblyType string

type TransformResources struct {
	// +kubebuilder:validation:Minimum=1
	InstanceCount *int64 `json:"instanceCount"`

	// Transform job has separate instance type called TransformInstanceType
	// Keeping it string
	// +kubebuilder:validation:MinLength=1
	InstanceType string `json:"instanceType"`

	VolumeKmsKeyId *string `json:"volumeKmsKeyId,omitempty"`
}

// DebugRuleConfiguration https://docs.aws.amazon.com/sagemaker/latest/dg/API_DebugRuleConfiguration.html
type DebugRuleConfiguration struct {
	RuleConfigurationName *string `json:"ruleConfigurationName"`
	LocalPath             *string `json:"localPath,omitempty"`
	// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
	S3OutputPath       *string `json:"s3OutputPath,omitempty"`
	RuleEvaluatorImage *string `json:"ruleEvaluatorImage"`
	// +kubebuilder:validation:Minimum=1
	VolumeSizeInGB *int64          `json:"volumeSizeInGB,omitempty"`
	InstanceType   string          `json:"instanceType,omitempty"`
	RuleParameters []*KeyValuePair `json:"ruleParameters,omitempty"`
}

// DebugHookConfig https://docs.aws.amazon.com/sagemaker/latest/dg/API_DebugHookConfig.html
type DebugHookConfig struct {
	LocalPath *string `json:"localPath,omitempty"`
	// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
	S3OutputPath             *string                    `json:"s3OutputPath"`
	HookParameters           []*KeyValuePair            `json:"ruleParameters,omitempty"`
	CollectionConfigurations []*CollectionConfiguration `json:"collectionConfigurations,omitempty"`
}

// CollectionConfiguration https://docs.aws.amazon.com/sagemaker/latest/dg/API_CollectionConfiguration.html
type CollectionConfiguration struct {
	CollectionName       *string         `json:"collectionName,omitempty"`
	CollectionParameters []*KeyValuePair `json:"collectionParameters,omitempty"`
}

// TensorBoardOutputConfig https://docs.aws.amazon.com/sagemaker/latest/dg/API_TensorBoardOutputConfig.html
type TensorBoardOutputConfig struct {
	LocalPath *string `json:"localPath,omitempty"`
	// +kubebuilder:validation:Pattern="^(https|s3)://([^/]+)/?(.*)$"
	S3OutputPath *string `json:"s3OutputPath"`
}

// DebugRuleEvaluationStatus https://docs.aws.amazon.com/sagemaker/latest/dg/API_DebugRuleEvaluationStatus.html
type DebugRuleEvaluationStatus struct {
	LastModifiedTime *metav1.Time `json:"lastModifiedTime,omitempty"`

	RuleConfigurationName *string `json:"ruleConfigurationName,omitempty"`

	RuleEvaluationJobArn *string `json:"ruleEvaluationJobArn,omitempty"`

	RuleEvaluationStatus *string `json:"ruleEvaluationStatus,omitempty"`

	StatusDetail *string `json:"statusDetail,omitempty"`
}

// AutoscalingResource is used to create the string representing the resourceID
// in the format endpoint/my-end-point/variant/my-variant
type AutoscalingResource struct {
	// +kubebuilder:validation:MinLength=1
	EndpointName *string `json:"endpointName,omitempty"`

	// +kubebuilder:validation:MinLength=1
	VariantName *string `json:"variantName,omitempty"`
}

// PredefinedMetricSpecification https://docs.aws.amazon.com/autoscaling/application/APIReference/API_PredefinedMetricSpecification.html
type PredefinedMetricSpecification struct {
	PredefinedMetricType *string `json:"predefinedMetricType,omitempty"`
}

// CustomizedMetricSpecification https://docs.aws.amazon.com/autoscaling/application/APIReference/API_CustomizedMetricSpecification.html
type CustomizedMetricSpecification struct {
	// +kubebuilder:validation:MinLength=1
	MetricName *string `json:"metricName,omitempty"`

	// +kubebuilder:validation:MinLength=1
	Namespace *string `json:"namespace,omitempty"`

	// +kubebuilder:validation:MinLength=1
	Statistic  *string         `json:"statistic,omitempty"`
	Unit       *string         `json:"unit,omitempty"`
	Dimensions []*KeyValuePair `json:"dimensions,omitempty"`
}

// TargetTrackingScalingPolicyConfig https://docs.aws.amazon.com/autoscaling/application/APIReference/API_TargetTrackingScalingPolicyConfiguration.html
// TODO: string requires the input to be in quotes in the spec which is not intuitive
// Needs a fix for floats, probably use resource.Quantity
type TargetTrackingScalingPolicyConfig struct {
	TargetValue      *int64 `json:"targetValue,omitempty"`
	ScaleInCooldown  *int64 `json:"scaleInCooldown,omitempty"`
	ScaleOutCooldown *int64 `json:"scaleOutCooldown,omitempty"`
	DisableScaleIn   *bool  `json:"disableScaleIn,omitempty"`

	// Ideally Predefined metric should not need a value but this is for consistency with API usage
	PredefinedMetricSpecification *PredefinedMetricSpecification `json:"predefinedMetricSpecification,omitempty"`
	CustomizedMetricSpecification *CustomizedMetricSpecification `json:"customizedMetricSpecification,omitempty"`
}

// HAPSuspendedState https://docs.aws.amazon.com/autoscaling/application/APIReference/API_SuspendedState.html
type HAPSuspendedState struct {
	DynamicScalingInSuspended  *bool `json:"dynamicScalingInSuspended,omitempty"`
	DynamicScalingOutSuspended *bool `json:"dynamicScalingOutSuspended,omitempty"`
	ScheduledScalingSuspended  *bool `json:"scheduledScalingSuspended,omitempty"`
}
