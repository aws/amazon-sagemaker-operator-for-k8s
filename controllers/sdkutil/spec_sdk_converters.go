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
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	gabs "github.com/Jeffail/gabs/v2"
	batchtransformjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/batchtransformjob"
	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	endpointconfigv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/endpointconfig"
	hostingautoscalingpolicyv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingautoscalingpolicy"
	hpojobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hyperparametertuningjob"
	modelv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/model"
	processingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/processingjob"
	trainingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/trainingjob"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/pkg/errors"
)

// HostingAutoscalingPolicyServiceNamespace is a constant value for using the Autoscaling API in the SageMaker Service
const HostingAutoscalingPolicyServiceNamespace = "sagemaker"

// CreateHyperParameterTuningJobSpecFromDescription creates a HyperParameterTuningJobSpec from a DescribeHyperParameterTuningJobOutput.
// This panics if json libraries are unable to serialize the description and deserialize the serialization.
func CreateHyperParameterTuningJobSpecFromDescription(description sagemaker.DescribeHyperParameterTuningJobOutput) hpojobv1.HyperparameterTuningJobSpec {
	if spec, err := createHyperParameterTuningJobSpecFromDescription(description); err == nil {
		return spec
	} else {
		panic("Unable to create HyperparameterTuningJobSpec from description: " + err.Error())
	}
}

// createHyperParameterTuningJobSpecFromDescription creates a HyperParameterTuningJobSpec from a DescribeHyperParameterTuningJobOutput.
func createHyperParameterTuningJobSpecFromDescription(description sagemaker.DescribeHyperParameterTuningJobOutput) (hpojobv1.HyperparameterTuningJobSpec, error) {

	transformedHyperParameters := []*commonv1.KeyValuePair{}

	if description.TrainingJobDefinition != nil {
		transformedHyperParameters = ConvertMapToKeyValuePairSlice(description.TrainingJobDefinition.StaticHyperParameters)
	}

	marshalled, err := json.Marshal(description)
	if err != nil {
		return hpojobv1.HyperparameterTuningJobSpec{}, err
	}

	// Replace map of hyperparameters with list of hyperparameters.
	// gabs makes this easier.
	obj, err := gabs.ParseJSON(marshalled)
	if err != nil {
		return hpojobv1.HyperparameterTuningJobSpec{}, err
	}

	if description.TrainingJobDefinition != nil {
		if _, err := obj.SetP(transformedHyperParameters, "TrainingJobDefinition.StaticHyperParameters"); err != nil {
			return hpojobv1.HyperparameterTuningJobSpec{}, err
		}
	}

	var unmarshalled hpojobv1.HyperparameterTuningJobSpec
	if err := json.Unmarshal(obj.Bytes(), &unmarshalled); err != nil {
		return hpojobv1.HyperparameterTuningJobSpec{}, err
	}

	return unmarshalled, nil
}

// CreateTrainingJobSpecFromDescription creates a TrainingJobSpec from a SageMaker description. This uses JSON to do the assignment. It also transforms the hyperparameter
// list from map to list of key,value pairs.
func CreateTrainingJobSpecFromDescription(description sagemaker.DescribeTrainingJobOutput) (trainingjobv1.TrainingJobSpec, error) {

	transformedHyperParameters := ConvertMapToKeyValuePairSlice(description.HyperParameters)

	marshalled, err := json.Marshal(description)
	if err != nil {
		return trainingjobv1.TrainingJobSpec{}, err
	}

	// Replace map of hyperparameters with list of hyperparameters.
	obj, err := gabs.ParseJSON(marshalled)
	if err != nil {
		return trainingjobv1.TrainingJobSpec{}, err
	}

	if _, err := obj.Set(transformedHyperParameters, "HyperParameters"); err != nil {
		return trainingjobv1.TrainingJobSpec{}, err
	}

	var unmarshalled trainingjobv1.TrainingJobSpec
	if err := json.Unmarshal(obj.Bytes(), &unmarshalled); err != nil {
		return trainingjobv1.TrainingJobSpec{}, err
	}

	return unmarshalled, nil
}

// CreateCreateProcessingJobInputFromSpec creates a CreateProcessingJobInput from a ProcessingJobSpec.
// This panics if json libraries are unable to serialize the spec or deserialize the serialization.
func CreateCreateProcessingJobInputFromSpec(spec processingjobv1.ProcessingJobSpec, processingJobName *string) sagemaker.CreateProcessingJobInput {
	if input, err := createCreateProcessingJobInputFromSpec(spec, processingJobName); err == nil {
		return input
	} else {
		panic("Unable to create CreateProcessingJobInput from spec : " + err.Error())
	}
}

func createCreateProcessingJobInputFromSpec(spec processingjobv1.ProcessingJobSpec, processingJobName *string) (sagemaker.CreateProcessingJobInput, error) {
	var output sagemaker.CreateProcessingJobInput

	// clear out the KVPs from spec.
	environmentVars := spec.Environment
	spec.Environment = []*commonv1.KeyValuePair{}

	marshalledCreateProcessingJobInput, err := json.Marshal(spec)

	if err != nil {
		return sagemaker.CreateProcessingJobInput{}, err
	}

	if err = json.Unmarshal(marshalledCreateProcessingJobInput, &output); err != nil {
		return sagemaker.CreateProcessingJobInput{}, err
	}

	output.Environment = ConvertKeyValuePairSliceToMap(environmentVars)
	output.ProcessingJobName = processingJobName

	return output, nil
}

// CreateCreateTrainingJobInputFromSpec creates a CreateTrainingJobInput from a TrainingJobSpec.
// This panics if json libraries are unable to serialize the spec or deserialize the serialization.
func CreateCreateTrainingJobInputFromSpec(spec trainingjobv1.TrainingJobSpec) sagemaker.CreateTrainingJobInput {
	if input, err := createCreateTrainingJobInputFromSpec(spec); err == nil {
		return input
	} else {
		panic("Unable to create CreateTrainingJobInput from spec : " + err.Error())
	}
}

// createCreateTrainingJobInputFromSpec creates a CreateTrainingJob request input from a Kubernetes spec.
// TODO Implement tests or find an alternative method.
// This approach was done as part of a proof of concept. It escapes the Go type system via json to convert
// between trainingjobv1.and sdk struct types. There are a few other ways to do it (see alternatives).
// This way can be acceptable if we have test coverage that assures breakage when sdk / trainingjobv1.structs diverage.
// Alternatives: https://quip-amazon.com/3PVUAsbL9I69/how-do-we-convert-between-structs-coming-from-etcd-to-structs-going-to-sagemaker
func createCreateTrainingJobInputFromSpec(spec trainingjobv1.TrainingJobSpec) (sagemaker.CreateTrainingJobInput, error) {
	var output sagemaker.CreateTrainingJobInput

	// clear out the old KVPs from spec.
	hyperParameters := spec.HyperParameters
	spec.HyperParameters = []*commonv1.KeyValuePair{}

	// Debugger related structs
	debugRuleConfigurationsRuleParameters := [][]*commonv1.KeyValuePair{}
	debugHookConfigHookParameters := []*commonv1.KeyValuePair{}
	debugHookConfigCollectionsConfigurationsCollectionParameters := [][]*commonv1.KeyValuePair{}

	if spec.DebugHookConfig != nil {
		debugHookConfigHookParameters = spec.DebugHookConfig.HookParameters
		spec.DebugHookConfig.HookParameters = []*commonv1.KeyValuePair{}

		for _, debugHookConfigCollectionConfiguration := range spec.DebugHookConfig.CollectionConfigurations {
			debugHookConfigCollectionsConfigurationsCollectionParameters = append(debugHookConfigCollectionsConfigurationsCollectionParameters, debugHookConfigCollectionConfiguration.CollectionParameters)
			debugHookConfigCollectionConfiguration.CollectionParameters = []*commonv1.KeyValuePair{}
		}
	}

	if spec.DebugRuleConfigurations != nil {
		for _, debugRuleConfiguration := range spec.DebugRuleConfigurations {
			debugRuleConfigurationRuleParameters := debugRuleConfiguration.RuleParameters
			debugRuleConfigurationsRuleParameters = append(debugRuleConfigurationsRuleParameters, debugRuleConfigurationRuleParameters)
			debugRuleConfiguration.RuleParameters = []*commonv1.KeyValuePair{}
		}
	}

	marshalledCreateTrainingJobInput, err := json.Marshal(spec)
	if err != nil {
		return sagemaker.CreateTrainingJobInput{}, err
	}

	if err = json.Unmarshal(marshalledCreateTrainingJobInput, &output); err != nil {
		return sagemaker.CreateTrainingJobInput{}, err
	}

	output.HyperParameters = ConvertKeyValuePairSliceToMap(hyperParameters)

	if output.DebugHookConfig != nil {
		output.DebugHookConfig.HookParameters = ConvertKeyValuePairSliceToMap(debugHookConfigHookParameters)
		for i := range output.DebugHookConfig.CollectionConfigurations {
			output.DebugHookConfig.CollectionConfigurations[i].CollectionParameters = ConvertKeyValuePairSliceToMap(debugHookConfigCollectionsConfigurationsCollectionParameters[i])
		}
	}

	if output.DebugRuleConfigurations != nil {
		for i := range output.DebugRuleConfigurations {
			output.DebugRuleConfigurations[i].RuleParameters = ConvertKeyValuePairSliceToMap(debugRuleConfigurationsRuleParameters[i])
		}
	}

	return output, nil
}

// CreateCreateHyperParameterTuningJobInputFromSpec creates a CreateHPO request input from a Kubernetes HPO spec.
func CreateCreateHyperParameterTuningJobInputFromSpec(spec hpojobv1.HyperparameterTuningJobSpec) (sagemaker.CreateHyperParameterTuningJobInput, error) {
	var target sagemaker.CreateHyperParameterTuningJobInput

	// Kubebuilder does not support arbitrary maps, so we encode these as KeyValuePairs.
	// After the JSON conversion, we will re-set the KeyValuePairs as map elements.
	var staticHyperParameters []*commonv1.KeyValuePair = []*commonv1.KeyValuePair{}
	if spec.TrainingJobDefinition != nil {
		staticHyperParameters = spec.TrainingJobDefinition.StaticHyperParameters
		spec.TrainingJobDefinition.StaticHyperParameters = []*commonv1.KeyValuePair{}
	}

	// TODO we should consider an alternative approach, see CreateCreateTrainingJobInputFromSpec
	str, err := json.Marshal(spec)
	if err != nil {
		return sagemaker.CreateHyperParameterTuningJobInput{}, err
	}

	if err = json.Unmarshal(str, &target); err != nil {
		return sagemaker.CreateHyperParameterTuningJobInput{}, err
	}

	if len(staticHyperParameters) > 0 {
		if target.TrainingJobDefinition == nil {
			target.TrainingJobDefinition = &sagemaker.HyperParameterTrainingJobDefinition{}
		}

		target.TrainingJobDefinition.StaticHyperParameters = ConvertKeyValuePairSliceToMap(staticHyperParameters)
	}

	return target, nil
}

// ConvertHyperParameterTrainingJobSummaryFromSageMaker converts a HyperParameterTrainingJobSummary to a Kubernetes SageMaker type, returning errors if there are any.
func ConvertHyperParameterTrainingJobSummaryFromSageMaker(source *sagemaker.HyperParameterTrainingJobSummary) (*commonv1.HyperParameterTrainingJobSummary, error) {
	var target commonv1.HyperParameterTrainingJobSummary

	// Kubebuilder does not support arbitrary maps, so we encode these as KeyValuePairs.
	// After the JSON conversion, we will re-set the KeyValuePairs as map elements.
	var tunedHyperParameters []*commonv1.KeyValuePair = []*commonv1.KeyValuePair{}

	for name, value := range source.TunedHyperParameters {
		tunedHyperParameters = append(tunedHyperParameters, &commonv1.KeyValuePair{
			Name:  name,
			Value: value,
		})
	}

	// TODO we should consider an alternative approach, see comments in TrainingController.
	str, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(str, &target)

	target.TunedHyperParameters = tunedHyperParameters
	return &target, nil
}

// ConvertDebugRuleEvaluationStatusesFromSageMaker converts an array of SageMaker DebugRuleEvaluationStatus to a Kubernetes SageMaker type.
func ConvertDebugRuleEvaluationStatusesFromSageMaker(source []sagemaker.DebugRuleEvaluationStatus) ([]commonv1.DebugRuleEvaluationStatus, error) {
	var convertedStatuses []commonv1.DebugRuleEvaluationStatus

	for _, status := range source {
		var target commonv1.DebugRuleEvaluationStatus

		str, err := json.Marshal(status)
		if err != nil {
			return nil, err
		}

		if err = json.Unmarshal(str, &target); err != nil {
			return nil, err
		}

		convertedStatuses = append(convertedStatuses, target)
	}

	return convertedStatuses, nil
}

// CreateTrainingJobStatusCountersFromDescription creates a set of TrainingJobStatusCounters from a DescribeHyperParameterTuningJobOutput
func CreateTrainingJobStatusCountersFromDescription(sageMakerDescription *sagemaker.DescribeHyperParameterTuningJobOutput) *commonv1.TrainingJobStatusCounters {
	if sageMakerDescription != nil && sageMakerDescription.TrainingJobStatusCounters != nil {
		var totalError *int64 = nil

		if sageMakerDescription.TrainingJobStatusCounters.NonRetryableError != nil && sageMakerDescription.TrainingJobStatusCounters.RetryableError != nil {
			totalErrorVal := *sageMakerDescription.TrainingJobStatusCounters.NonRetryableError + *sageMakerDescription.TrainingJobStatusCounters.RetryableError
			totalError = &totalErrorVal
		}

		return &commonv1.TrainingJobStatusCounters{
			Completed:         sageMakerDescription.TrainingJobStatusCounters.Completed,
			InProgress:        sageMakerDescription.TrainingJobStatusCounters.InProgress,
			NonRetryableError: sageMakerDescription.TrainingJobStatusCounters.NonRetryableError,
			RetryableError:    sageMakerDescription.TrainingJobStatusCounters.RetryableError,
			TotalError:        totalError,
			Stopped:           sageMakerDescription.TrainingJobStatusCounters.Stopped,
		}
	}

	return &commonv1.TrainingJobStatusCounters{}
}

// CreateCreateModelInputFromSpec creates a CreateModel request input from a Kubernetes Model spec.
func CreateCreateModelInputFromSpec(model *modelv1.ModelSpec, modelName string) (*sagemaker.CreateModelInput, error) {

	var primaryContainerEnvironment []*commonv1.KeyValuePair
	var containersEnvironment [][]*commonv1.KeyValuePair

	if model.Containers != nil {
		for _, container := range model.Containers {
			containerEnvironment := container.Environment
			containersEnvironment = append(containersEnvironment, containerEnvironment)
			// reset in spec
			container.Environment = []*commonv1.KeyValuePair{}
		}
	}

	if model.PrimaryContainer != nil {
		primaryContainerEnvironment = model.PrimaryContainer.Environment
		// reset in spec
		model.PrimaryContainer.Environment = []*commonv1.KeyValuePair{}
	}

	jsonstr, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}

	var output sagemaker.CreateModelInput
	if err = json.Unmarshal(jsonstr, &output); err != nil {
		return nil, err
	}

	if output.Containers != nil {
		for i := range output.Containers {
			output.Containers[i].Environment = ConvertKeyValuePairSliceToMap(containersEnvironment[i])
		}
	}

	if output.PrimaryContainer != nil {
		output.PrimaryContainer.Environment = ConvertKeyValuePairSliceToMap(primaryContainerEnvironment)
	}
	output.ModelName = &modelName

	return &output, nil
}

// CreateDeleteModelInput creates a DeleteModel request input from a ModelName.
func CreateDeleteModelInput(modelName *string) (*sagemaker.DeleteModelInput, error) {
	var output sagemaker.DeleteModelInput
	output.ModelName = modelName

	return &output, nil
}

// CreateModelSpecFromDescription creates a Kubernetes Model spec from a SageMaker model description.
func CreateModelSpecFromDescription(description *sagemaker.DescribeModelOutput) (*modelv1.ModelSpec, error) {

	transformedContainersEnvironment := [][]*commonv1.KeyValuePair{}
	transformedContainerEnvironment := []*commonv1.KeyValuePair{}
	transformedPrimaryContainerEnvironment := []*commonv1.KeyValuePair{}

	if description.Containers != nil {
		// Go through each container
		for _, container := range description.Containers {
			transformedContainerEnvironment = ConvertMapToKeyValuePairSlice(container.Environment)
			transformedContainersEnvironment = append(transformedContainersEnvironment, transformedContainerEnvironment)
		}
	}

	if description.PrimaryContainer != nil {
		transformedPrimaryContainerEnvironment = ConvertMapToKeyValuePairSlice(description.PrimaryContainer.Environment)
	}

	marshalled, err := json.Marshal(description)
	if err != nil {
		return nil, err
	}

	// Replace map of environments with list of environment.
	// gabs makes this easier.
	obj, err := gabs.ParseJSON(marshalled)
	if err != nil {
		return nil, err
	}

	if description.Containers != nil {
		for i, _ := range description.Containers {
			if _, err := obj.SetP(transformedContainersEnvironment[i], "Containers/"+strconv.Itoa(i)+"/.Environment"); err != nil {
				return nil, err
			}
		}
	}

	if description.PrimaryContainer != nil {
		if _, err := obj.SetP(transformedPrimaryContainerEnvironment, "PrimaryContainer.Environment"); err != nil {
			return nil, err
		}
	}

	var unmarshalled modelv1.ModelSpec
	if err := json.Unmarshal(obj.Bytes(), &unmarshalled); err != nil {
		return nil, err
	}

	return &unmarshalled, nil
}

// CreateCreateBatchTransformJobInputFromSpec creates a CreateTrainingJobInput from a BatchTransformJobSpec
func CreateCreateBatchTransformJobInputFromSpec(spec batchtransformjobv1.BatchTransformJobSpec) sagemaker.CreateTransformJobInput {
	input, err := createCreateBatchTransformJobInputFromSpec(spec)
	if err == nil {
		return input
	}
	panic("Unable to create CreateHyperParameterTuningJobInput from spec : " + err.Error())
}

//createCreateBatchTransformJobInputFromSpec creates a BatchTransformJobInput From Spec
func createCreateBatchTransformJobInputFromSpec(spec batchtransformjobv1.BatchTransformJobSpec) (sagemaker.CreateTransformJobInput, error) {
	var target sagemaker.CreateTransformJobInput

	marshalledCreateBatchTransformJobInput, err := json.Marshal(spec)
	if err != nil {
		return sagemaker.CreateTransformJobInput{}, err
	}

	if err = json.Unmarshal(marshalledCreateBatchTransformJobInput, &target); err != nil {
		return sagemaker.CreateTransformJobInput{}, err
	}

	return target, nil
}

// CreateTransformJobSpecFromDescription creates a BatchTransformJobSpec from a DescribeTrainingJobOutput.
// This panics if json libraries are unable to serialize the description and deserialize the serialization.
func CreateTransformJobSpecFromDescription(description sagemaker.DescribeTransformJobOutput) batchtransformjobv1.BatchTransformJobSpec {
	if spec, err := createTransformJobSpecFromDescription(description); err == nil {
		return spec
	} else {
		panic("Unable to create TrainingJobSpec from description: " + err.Error())
	}
}

// createTransformJobSpecFromDescription creates a BatchTransformJobSpec from a SageMaker description. This uses JSON to do the assignment.
func createTransformJobSpecFromDescription(description sagemaker.DescribeTransformJobOutput) (batchtransformjobv1.BatchTransformJobSpec, error) {

	marshalled, err := json.Marshal(description)
	if err != nil {
		return batchtransformjobv1.BatchTransformJobSpec{}, err
	}

	obj, err := gabs.ParseJSON(marshalled)
	if err != nil {
		return batchtransformjobv1.BatchTransformJobSpec{}, err
	}

	var unmarshalled batchtransformjobv1.BatchTransformJobSpec
	if err := json.Unmarshal(obj.Bytes(), &unmarshalled); err != nil {
		return batchtransformjobv1.BatchTransformJobSpec{}, err
	}

	return unmarshalled, nil
}

// CreateCreateEndpointConfigInputFromSpec creates a CreateEndpointConfig request input from a Kubernetes EndpointConfig spec.
func CreateCreateEndpointConfigInputFromSpec(endpointconfig *endpointconfigv1.EndpointConfigSpec, endpointConfigName string) (*sagemaker.CreateEndpointConfigInput, error) {

	jsonstr, err := json.Marshal(endpointconfig)
	if err != nil {
		return nil, err
	}

	var output sagemaker.CreateEndpointConfigInput
	if err = json.Unmarshal(jsonstr, &output); err != nil {
		return nil, err
	}

	output.EndpointConfigName = &endpointConfigName

	return &output, nil
}

// CreateDeleteEndpointConfigInput creates a DeleteEndpointConfigRequest input from a EndpointConfigName.
func CreateDeleteEndpointConfigInput(endpointConfigName *string) (*sagemaker.DeleteEndpointConfigInput, error) {
	var output sagemaker.DeleteEndpointConfigInput
	output.EndpointConfigName = endpointConfigName

	return &output, nil
}

// CreateEndpointConfigSpecFromDescription creates a Kubernetes EndpointConfig spec from a SageMaker endpointconfig description.
func CreateEndpointConfigSpecFromDescription(description *sagemaker.DescribeEndpointConfigOutput) (*endpointconfigv1.EndpointConfigSpec, error) {
	jsonstr, err := json.Marshal(description)
	if err != nil {
		return nil, err
	}

	var output endpointconfigv1.EndpointConfigSpec
	if err = json.Unmarshal(jsonstr, &output); err != nil {
		return nil, err
	}

	return &output, nil
}

// ConvertProductionVariantSummary creates a *commonv1.ProductionVariantSummary from the equivalent SageMaker type.
func ConvertProductionVariantSummary(pv *sagemaker.ProductionVariantSummary) (*commonv1.ProductionVariantSummary, error) {

	jsonstr, err := json.Marshal(pv)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to convert produciton variant to Kubernetes type")
	}

	// If there are non-nil float64s, we need to convert them to a type that
	// the Kubernetes API supports.
	if pv.DesiredWeight != nil || pv.CurrentWeight != nil {

		obj, err := gabs.ParseJSON(jsonstr)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to convert production variant weights to int64")
		}

		if pv.DesiredWeight != nil {
			if err = replaceFloat64WithInt64(obj, "DesiredWeight", *pv.DesiredWeight); err != nil {
				return nil, errors.Wrap(err, "Unable to convert production variant desired weights to int64")
			}
		}

		if pv.CurrentWeight != nil {
			if err = replaceFloat64WithInt64(obj, "CurrentWeight", *pv.CurrentWeight); err != nil {
				return nil, errors.Wrap(err, "Unable to convert production variant current weights to int64")
			}
		}

		jsonstr = obj.Bytes()
	}

	var output commonv1.ProductionVariantSummary
	if err = json.Unmarshal(jsonstr, &output); err != nil {
		return nil, errors.Wrap(err, "Unable to convert produciton variant to Kubernetes type")
	}

	return &output, nil
}

func replaceFloat64WithInt64(obj *gabs.Container, path string, toConvert float64) error {
	if math.IsNaN(toConvert) || math.IsInf(toConvert, 0) {
		return fmt.Errorf("Unable to convert float64 '%f' to int64", toConvert)
	}

	integerWeight := int64(toConvert)

	if _, err := obj.Set(integerWeight, path); err != nil {
		return errors.Wrap(err, "Unable to replace float64 with int64")
	}

	return nil
}

// ConvertProductionVariantSummarySlice creates a []*commonv1.ProductionVariantSummary from the equivalent SageMaker type.
func ConvertProductionVariantSummarySlice(pvs []sagemaker.ProductionVariantSummary) ([]*commonv1.ProductionVariantSummary, error) {
	productionVariants := []*commonv1.ProductionVariantSummary{}
	for _, pv := range pvs {
		if converted, err := ConvertProductionVariantSummary(&pv); err != nil {
			return nil, err
		} else {
			productionVariants = append(productionVariants, converted)
		}
	}

	return productionVariants, nil
}

// ConvertTagSliceToSageMakerTagSlice converts Tags to Sagemaker Tags
func ConvertTagSliceToSageMakerTagSlice(tags []commonv1.Tag) []sagemaker.Tag {
	sageMakerTags := []sagemaker.Tag{}
	for _, tag := range tags {
		sageMakerTags = append(sageMakerTags, sagemaker.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}

	return sageMakerTags
}

// ConvertKeyValuePairSliceToMap converts key value pairs to a map
func ConvertKeyValuePairSliceToMap(kvps []*commonv1.KeyValuePair) map[string]string {
	target := map[string]string{}
	for _, kvp := range kvps {
		target[kvp.Name] = kvp.Value
	}
	return target
}

// ConvertMapToKeyValuePairSlice converts a map to a key value pair
func ConvertMapToKeyValuePairSlice(m map[string]string) []*commonv1.KeyValuePair {
	var kvps []*commonv1.KeyValuePair
	for name, value := range m {
		kvps = append(kvps, &commonv1.KeyValuePair{
			Name:  name,
			Value: value,
		})
	}
	return kvps
}

// ConvertAutoscalingResourceToString converts a map to a key value pair
func ConvertAutoscalingResourceToString(resourceIDfromSpec commonv1.AutoscalingResource) *string {
	var resourceString string = "endpoint/" + *resourceIDfromSpec.EndpointName + "/variant/" + *resourceIDfromSpec.VariantName
	return &resourceString
}

// CreateRegisterScalableTargetInputFromSpec from a JobSpec.
// This panics if json libraries are unable to serialize the spec or deserialize the serialization.
func CreateRegisterScalableTargetInputFromSpec(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec) []applicationautoscaling.RegisterScalableTargetInput {

	if input, err := createRegisterScalableTargetInputListFromSpec(spec); err == nil {
		return input
	} else {
		panic("Unable to CreateRegisterScalableTargetInputFromSpec : " + err.Error())
	}
}

// createRegisterScalableTargetInputListFromSpec request input from a Kubernetes spec.
func createRegisterScalableTargetInputListFromSpec(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec) ([]applicationautoscaling.RegisterScalableTargetInput, error) {
	var outputList []applicationautoscaling.RegisterScalableTargetInput
	var output applicationautoscaling.RegisterScalableTargetInput

	// clear out the old KVPs from spec and init to empty struct
	resourceIDListfromSpec := spec.ResourceID
	spec.ResourceID = []*commonv1.AutoscalingResource{}

	for _, resourceIDfromSpec := range resourceIDListfromSpec {
		ResourceID := ConvertAutoscalingResourceToString(*resourceIDfromSpec)
		output, _ = createRegisterScalableTargetInputFromSpec(spec, ResourceID)
		outputList = append(outputList, output)
	}

	return outputList, nil
}

// createRegisterScalableTargetInputFromSpec request input from a Kubernetes spec.
func createRegisterScalableTargetInputFromSpec(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec, resourceID *string) (applicationautoscaling.RegisterScalableTargetInput, error) {
	var output applicationautoscaling.RegisterScalableTargetInput

	marshalledRegisterScalableTargetInput, err := json.Marshal(spec)
	if err != nil {
		return applicationautoscaling.RegisterScalableTargetInput{}, err
	}

	if err = json.Unmarshal(marshalledRegisterScalableTargetInput, &output); err != nil {
		return applicationautoscaling.RegisterScalableTargetInput{}, err
	}

	output.ResourceId = resourceID

	return output, nil
}

// CreatePutScalingPolicyInputFromSpec from a JobSpec.
// This panics if json libraries are unable to serialize the spec or deserialize the serialization.
func CreatePutScalingPolicyInputFromSpec(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec) []applicationautoscaling.PutScalingPolicyInput {

	if input, err := createPutScalingPolicyInputListFromSpec(spec); err == nil {
		return input
	} else {
		panic("Unable to create CreateTrainingJobInput from spec : " + err.Error())
	}
}

// createPutScalingPolicyInputListFromSpec request input from a Kubernetes spec.
func createPutScalingPolicyInputListFromSpec(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec) ([]applicationautoscaling.PutScalingPolicyInput, error) {
	var outputList []applicationautoscaling.PutScalingPolicyInput
	var output applicationautoscaling.PutScalingPolicyInput

	// clear out the old KVPs from spec and init to empty struct
	resourceIDListfromSpec := spec.ResourceID
	spec.ResourceID = []*commonv1.AutoscalingResource{}

	for _, resourceIDfromSpec := range resourceIDListfromSpec {
		ResourceID := ConvertAutoscalingResourceToString(*resourceIDfromSpec)
		output, _ = createPutScalingPolicyInputFromSpec(spec, ResourceID)
		outputList = append(outputList, output)
	}

	return outputList, nil
}

// createPutScalingPolicyInputFromSpec request input from a Kubernetes spec.
func createPutScalingPolicyInputFromSpec(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec, resourceID *string) (applicationautoscaling.PutScalingPolicyInput, error) {
	var output applicationautoscaling.PutScalingPolicyInput

	// clear out the CustomizedMetricSpecification KVPs from spec and init to empty struct
	customizedMetricSpecificationDimensions := []*commonv1.KeyValuePair{}
	if spec.TargetTrackingScalingPolicyConfiguration.CustomizedMetricSpecification != nil {
		customizedMetricSpecificationDimensions = spec.TargetTrackingScalingPolicyConfiguration.CustomizedMetricSpecification.Dimensions
		spec.TargetTrackingScalingPolicyConfiguration.CustomizedMetricSpecification.Dimensions = []*commonv1.KeyValuePair{}
	}

	marshalledPutScalingPolicyInputInput, err := json.Marshal(spec)
	if err != nil {
		return applicationautoscaling.PutScalingPolicyInput{}, err
	}

	if err = json.Unmarshal(marshalledPutScalingPolicyInputInput, &output); err != nil {
		return applicationautoscaling.PutScalingPolicyInput{}, err
	}

	output.ResourceId = resourceID

	if output.TargetTrackingScalingPolicyConfiguration.CustomizedMetricSpecification != nil {
		marshalledDimensions, err := json.Marshal(customizedMetricSpecificationDimensions)
		if err = json.Unmarshal(marshalledDimensions, &output.TargetTrackingScalingPolicyConfiguration.CustomizedMetricSpecification.Dimensions); err != nil {
			return applicationautoscaling.PutScalingPolicyInput{}, err
		}
	}

	return output, nil
}

// CreateDeregisterScalableTargetInput creates DeregisterScalableTargetInput from spec
func CreateDeregisterScalableTargetInput(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec, resourceID string) applicationautoscaling.DeregisterScalableTargetInput {

	if input, err := createDeregisterScalableTargetInput(spec, resourceID); err == nil {
		return input
	} else {
		panic("Unable to create CreateDegisterScalableTargetInput " + err.Error())
	}
}

// createDeregisterScalableTargetInput creates DeregisterScalableTargetInput from spec.
func createDeregisterScalableTargetInput(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec, resourceID string) (applicationautoscaling.DeregisterScalableTargetInput, error) {
	var output applicationautoscaling.DeregisterScalableTargetInput

	marshalledScalableDimension, err := json.Marshal(spec.ScalableDimension)
	if err = json.Unmarshal(marshalledScalableDimension, &output.ScalableDimension); err != nil {
		return applicationautoscaling.DeregisterScalableTargetInput{}, err
	}

	output.ResourceId = &resourceID
	output.ServiceNamespace = HostingAutoscalingPolicyServiceNamespace

	return output, nil
}

// CreateDeleteScalingPolicyInput creates DeleteScalingPolicyInput from spec
func CreateDeleteScalingPolicyInput(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec, resourceID string) applicationautoscaling.DeleteScalingPolicyInput {

	if input, err := createDeleteScalingPolicyInput(spec, resourceID); err == nil {
		return input
	} else {
		panic("Unable to create CreateDegisterScalableTargetInput " + err.Error())
	}
}

// createDeleteScalingPolicyInput creates DeleteScalingPolicyInput from spec
func createDeleteScalingPolicyInput(spec hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec, resourceID string) (applicationautoscaling.DeleteScalingPolicyInput, error) {
	var output applicationautoscaling.DeleteScalingPolicyInput

	marshalledScalableDimension, err := json.Marshal(spec.ScalableDimension)
	if err = json.Unmarshal(marshalledScalableDimension, &output.ScalableDimension); err != nil {
		return applicationautoscaling.DeleteScalingPolicyInput{}, err
	}

	output.PolicyName = spec.PolicyName
	output.ResourceId = &resourceID

	output.ServiceNamespace = HostingAutoscalingPolicyServiceNamespace

	return output, nil
}

// getResourceIDListfromDescriptions converts a map to a key value pair
func getResourceIDListfromDescriptions(descriptions []*applicationautoscaling.ScalingPolicy) []*commonv1.AutoscalingResource {
	var resourceIDListforSpec []*commonv1.AutoscalingResource

	for _, description := range descriptions {
		resourceID := strings.Split(*description.ResourceId, "/")
		endpointName, variantName := resourceID[1], resourceID[3]
		resourceIDforSpec := commonv1.AutoscalingResource{EndpointName: &endpointName, VariantName: &variantName}
		resourceIDListforSpec = append(resourceIDListforSpec, &resourceIDforSpec)
	}

	return resourceIDListforSpec
}

// getResourceIDForSpecFromList converts a list of ResourceID strings to the Spec format.
func getResourceIDForSpecFromList(resourceIDList []string) []*commonv1.AutoscalingResource {
	var resourceIDListforSpec []*commonv1.AutoscalingResource

	for _, resourceID := range resourceIDList {
		resourceID := strings.Split(resourceID, "/")
		endpointName, variantName := resourceID[1], resourceID[3]
		resourceIDforSpec := commonv1.AutoscalingResource{EndpointName: &endpointName, VariantName: &variantName}
		resourceIDListforSpec = append(resourceIDListforSpec, &resourceIDforSpec)
	}

	return resourceIDListforSpec
}

// CreateHostingAutoscalingPolicySpecFromDescription creates a Kubernetes spec from a List of Descriptions
// Review: Needs a major review and also update if additional fields are added/removed from spec
func CreateHostingAutoscalingPolicySpecFromDescription(targetDescriptions []*applicationautoscaling.DescribeScalableTargetsOutput, descriptions []*applicationautoscaling.ScalingPolicy, oldResourceIDList []string) (hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec, error) {
	transformedResourceIDs := getResourceIDForSpecFromList(oldResourceIDList)

	// This might not be needed since updates to customMetric and suspended state work out of the box
	minCapacity := targetDescriptions[0].ScalableTargets[0].MinCapacity
	maxCapacity := targetDescriptions[0].ScalableTargets[0].MaxCapacity

	marshalled, err := json.Marshal(descriptions[0])
	if err != nil {
		return hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec{}, err
	}

	obj, err := gabs.ParseJSON(marshalled)
	if err != nil {
		return hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec{}, err
	}

	if _, err := obj.Set(transformedResourceIDs, "ResourceId"); err != nil {
		return hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec{}, err
	}

	if _, err := obj.Set(minCapacity, "MinCapacity"); err != nil {
		return hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec{}, err
	}

	if _, err := obj.Set(maxCapacity, "MaxCapacity"); err != nil {
		return hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec{}, err
	}

	var unmarshalled hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec
	if err := json.Unmarshal(obj.Bytes(), &unmarshalled); err != nil {
		return hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec{}, err
	}

	return unmarshalled, nil
}

// Converts VariantProperties to SageMaker VariantProperties
func ConvertVariantPropertiesToSageMakerVariantProperties(variantProperties []commonv1.VariantProperty) []sagemaker.VariantProperty {
	sageMakerVariantProperties := []sagemaker.VariantProperty{}

	for _, variantProperty := range variantProperties {
		variantPropertyType := sagemaker.VariantPropertyTypeDesiredInstanceCount

		switch *variantProperty.VariantPropertyType {
		case "DesiredInstanceCount":
			variantPropertyType = sagemaker.VariantPropertyTypeDesiredInstanceCount
		case "DesiredWeight":
			variantPropertyType = sagemaker.VariantPropertyTypeDesiredWeight
		case "DataCaptureConfig":
			variantPropertyType = sagemaker.VariantPropertyTypeDataCaptureConfig
		default:
			variantPropertyType = ""
			errors.New("Error: invalid VariantPropertyType string '" + *variantProperty.VariantPropertyType + "'")
		}

		sageMakerVariantProperties = append(sageMakerVariantProperties, sagemaker.VariantProperty{
			VariantPropertyType: variantPropertyType,
		})
	}

	return sageMakerVariantProperties
}
