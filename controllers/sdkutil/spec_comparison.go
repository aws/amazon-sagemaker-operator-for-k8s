/*
Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sdkutil

import (
	batchtransformjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/batchtransformjob"
	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	endpointconfigv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/endpointconfig"
	hpojobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hyperparametertuningjob"
	modelv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/model"
	trainingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/trainingjob"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Simple struct representing whether two objects match and their differences.
type Comparison struct {
	// A human-readable list of differences.
	Differences string

	// Whether or not the two objects are equal.
	Equal bool
}

// TODO: Ensure there are no loose ends when deleting the following method.
// Determine if the given TrainingJobSpec matches the DescribeTrainingJobOutput. This converts the description to a TrainingJobSpec,
// then selectively compares fields.
// func TrainingJobSpecMatchesDescription(description sagemaker.DescribeTrainingJobOutput, spec trainingjobv1.TrainingJobSpec) Comparison {
// 	remoteSpec, _ := CreateTrainingJobSpecFromDescription(description)
// 	differences := cmp.Diff(remoteSpec, spec, trainingJobSpecComparisonOptions...)
// 	return createComparison(differences)
// }

// These options configure the equality check for TrainingJobSpecs.
var trainingJobSpecComparisonOptions = []cmp.Option{
	createIgnoreMetricDefinitionDifferencesOption(commonv1.AlgorithmSpecification{}),
	createIgnoreRegionOption(trainingjobv1.TrainingJobSpec{}),
	createIgnoreSageMakerEndpointOption(trainingjobv1.TrainingJobSpec{}),
	createIgnoreTagsOption(trainingjobv1.TrainingJobSpec{}),
	equateEmptyChannelFieldsToNone,
	equateEmptySlicesAndMapsToNil,
	equateNilBoolToFalse,
	equateNilStringToEmptyString,
	ignoreKeyValuePairSliceOrder,
}

// Simple helper function that creates a Comparison object from a difference result.
func createComparison(differences string) Comparison {
	return Comparison{
		Differences: differences,
		Equal:       differences == "",
	}
}

// Determine if the given HyperparameterTuningJobSpec matches the DescribeHyperParameterTuningJobOutput. This converts the description to a HyperparameterTuningJobSpec,
// then selectively compares fields.
func HyperparameterTuningJobSpecMatchesDescription(description sagemaker.DescribeHyperParameterTuningJobOutput, spec hpojobv1.HyperparameterTuningJobSpec) Comparison {
	remoteSpec := CreateHyperParameterTuningJobSpecFromDescription(description)
	differences := cmp.Diff(remoteSpec, spec, hyperparameterTuningJobSpecComparisonOptions...)
	return createComparison(differences)
}

// These options configure the equality check for HyperparameterTuningJobSpecs.
var hyperparameterTuningJobSpecComparisonOptions = []cmp.Option{
	createIgnoreMetricDefinitionDifferencesOption(commonv1.HyperParameterAlgorithmSpecification{}),
	createIgnoreRegionOption(hpojobv1.HyperparameterTuningJobSpec{}),
	createIgnoreSageMakerEndpointOption(hpojobv1.HyperparameterTuningJobSpec{}),
	createIgnoreTagsOption(hpojobv1.HyperparameterTuningJobSpec{}),
	equateEmptyChannelFieldsToNone,
	equateEmptySlicesAndMapsToNil,
	equateNilBoolToFalse,
	equateNilStringToEmptyString,
	ignoreKeyValuePairSliceOrder,
	ignoreKeyValuePairsWithUnderscoreNamePrefix,
}

// Determine if the given BatchTransformJobSpec matches the DescribeTransformJobOutput. This converts the description to a BatchTransformJobSpec,
// then selectively compares fields.
func TransformJobSpecMatchesDescription(description sagemaker.DescribeTransformJobOutput, spec batchtransformjobv1.BatchTransformJobSpec) Comparison {
	remoteSpec := CreateTransformJobSpecFromDescription(description)
	differences := cmp.Diff(remoteSpec, spec, transformJobSpecComparisonOptions...)
	return createComparison(differences)
}

// These options configure the equality check for BatchTransformJobSpec.
var transformJobSpecComparisonOptions = []cmp.Option{
	createIgnoreRegionOption(batchtransformjobv1.BatchTransformJobSpec{}),
	createIgnoreSageMakerEndpointOption(batchtransformjobv1.BatchTransformJobSpec{}),
	createIgnoreTagsOption(batchtransformjobv1.BatchTransformJobSpec{}),
	equateEmptyChannelFieldsToNone,
	equateEmptySlicesAndMapsToNil,
	equateEmptyTransformInputFieldsToNone,
	equateEmptyTransformOutputFieldsToNone,
	equateNilStringToEmptyString,
	ignoreDataProcessingOption,
	ignoreKeyValuePairSliceOrder,
}

// Determine if the given ModelSpec matches the DescribeModelOutput. This converts the description to a ModelSpec,
// then selectively compares fields.
func ModelSpecMatchesDescription(description sagemaker.DescribeModelOutput, spec modelv1.ModelSpec) (Comparison, error) {
	remoteSpec, err := CreateModelSpecFromDescription(&description)
	if err != nil {
		return Comparison{}, err
	}
	differences := cmp.Diff(*remoteSpec, spec, modelSpecComparisonOptions...)
	return createComparison(differences), nil
}

// These options configure the equality check for ModelSpecs.
var modelSpecComparisonOptions = []cmp.Option{
	createIgnoreRegionOption(modelv1.ModelSpec{}),
	createIgnoreSageMakerEndpointOption(modelv1.ModelSpec{}),
	createIgnoreTagsOption(modelv1.ModelSpec{}),
	equateEmptySlicesAndMapsToNil,
	equateNilBoolToFalse,
	equateNilStringToEmptyString,
	ignoreKeyValuePairSliceOrder,
}

// Determine if the given EndpointConfigSpec matches the DescribeEndpointConfigOutput. This converts the description to a EndpointConfigSpec,
// then selectively compares fields.
func EndpointConfigSpecMatchesDescription(description sagemaker.DescribeEndpointConfigOutput, spec endpointconfigv1.EndpointConfigSpec) (Comparison, error) {
	remoteSpec, err := CreateEndpointConfigSpecFromDescription(&description)
	if err != nil {
		return Comparison{}, err
	}
	differences := cmp.Diff(*remoteSpec, spec, endpointConfigSpecComparisonOptions...)
	return createComparison(differences), nil
}

// These options configure the equality check for EndpointConfigSpecs.
var endpointConfigSpecComparisonOptions = []cmp.Option{
	createIgnoreRegionOption(endpointconfigv1.EndpointConfigSpec{}),
	createIgnoreSageMakerEndpointOption(endpointconfigv1.EndpointConfigSpec{}),
	createIgnoreTagsOption(endpointconfigv1.EndpointConfigSpec{}),
	equateEmptySlicesAndMapsToNil,
	equateNilStringToEmptyString,
	ignoreKeyValuePairSliceOrder,
}

// Equate bool pointers that are nil to false.
var equateNilBoolToFalse = cmp.Transformer("", func(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
})

// Equate string pointers that are nil to the empty string.
var equateNilStringToEmptyString = cmp.Transformer("", func(value *string) string {
	if value == nil {
		return ""
	}
	return *value
})

// Equate string pointers that are nil in the TransformInput to the "None" string.
var equateEmptyTransformInputFieldsToNone = cmp.Transformer("", func(value commonv1.TransformInput) commonv1.TransformInput {
	if value.CompressionType == "" {
		value.CompressionType = "None"
	}
	if value.SplitType == "" {
		value.SplitType = "None"
	}
	return value
})

// Equate string pointers that are nil in the TransformOutput to the "None" string.
var equateEmptyTransformOutputFieldsToNone = cmp.Transformer("", func(value commonv1.TransformOutput) commonv1.TransformOutput {
	if value.AssembleWith == "" {
		value.AssembleWith = "None"
	}
	return value
})

// Equate Channel.RecordWrapperType and .CompressionType to "None" when empty. This is the same logic done by SageMaker.
var equateEmptyChannelFieldsToNone = cmp.Transformer("", func(value commonv1.Channel) commonv1.Channel {
	if value.RecordWrapperType == "" {
		value.RecordWrapperType = "None"
	}
	if value.CompressionType == "" {
		value.CompressionType = "None"
	}
	return value
})

var equateEmptySlicesAndMapsToNil = cmpopts.EquateEmpty()

// Define a custom sort order for *KeyValuePair slices.
var ignoreKeyValuePairSliceOrder = cmpopts.SortSlices(func(left, right *commonv1.KeyValuePair) bool {

	// Order is arbitrary, but must be deterministic, irreflexive, and transitive.
	// https://godoc.org/github.com/google/go-cmp/cmp/cmpopts#SortSlices
	if left == nil && right == nil {
		return true
	} else if left != nil && right == nil {
		return true
	} else if left == nil && right != nil {
		return false
	}

	if left.Name != right.Name {
		return left.Name < right.Name
	} else {
		return left.Value < right.Value
	}
})

// SageMaker adds StaticHyperParameters prefixed with an underscore. We must ignore these when comparing.
var ignoreKeyValuePairsWithUnderscoreNamePrefix = cmpopts.IgnoreSliceElements(func(kvp *commonv1.KeyValuePair) bool {
	if kvp == nil {
		return false
	}
	if len(kvp.Name) > 0 && kvp.Name[0:1] == "_" {
		return true
	}
	return false
})

// The operator value for DataProcessing is overwritten by the default value assigned by the service.
// Therefore we should ignore it if we are passing it as nil in the spec.
var ignoreDataProcessingOption = cmp.FilterValues(isEitherDataProcessingNodeNil, cmp.Comparer(equateAlways))

// SageMaker adds MetricDefinitions to the AlgorithmSpecification for 1p algorithms.
func createIgnoreMetricDefinitionDifferencesOption(parent interface{}) cmp.Option {
	return cmpopts.IgnoreFields(parent, "MetricDefinitions")
}

// The operator types have a Region field, but the SageMaker SDK types do not.
// Thus, we ignore when determining equality.
func createIgnoreRegionOption(parent interface{}) cmp.Option {
	return cmpopts.IgnoreFields(parent, "Region")
}

// The operator types have a SageMakerEndpoint field, but the SageMaker SDK types do not.
// Thus, we ignore when determining equality.
func createIgnoreSageMakerEndpointOption(parent interface{}) cmp.Option {
	return cmpopts.IgnoreFields(parent, "SageMakerEndpoint")
}

// SageMaker has tags in the various Create requests but does not return them in the Describe.
// We have an outstanding task to fix all controllers to use separate API when determining tags.
// This will ignore comparing tags until then.
// TODO implement support for tag updates.
func createIgnoreTagsOption(parent interface{}) cmp.Option {
	return cmpopts.IgnoreFields(parent, "Tags")
}

func equateAlways(_, _ interface{}) bool { return true }

func isEitherDataProcessingNodeNil(x, y *commonv1.DataProcessing) bool {
	return x == nil || y == nil
}
