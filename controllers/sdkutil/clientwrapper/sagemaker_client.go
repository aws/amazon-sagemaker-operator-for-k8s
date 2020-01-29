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

package clientwrapper

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"

	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
)

// Provides the prefixes and error codes relating to each endpoint
const (
	DescribeTrainingJob404Code          = "ValidationException"
	DescribeTrainingJob404MessagePrefix = "Requested resource not found"
	StopTrainingJob404Code              = "ValidationException"
	StopTrainingJob404MessagePrefix     = "Requested resource not found"

	DescribeHyperParameterTuningJob404Code          = "ValidationException"
	DescribeHyperParameterTuningJob404MessagePrefix = "Requested resource not found"
	StopHyperParameterTuningJob404Code              = "ValidationException"
	StopHyperParameterTuningJob404MessagePrefix     = "Requested resource not found"

	DeleteEndpoint404MessagePrefix                        = "Could not find endpoint"
	DeleteEndpoint404Code                                 = "ValidationException"
	DeleteEndpointInProgressMessagePrefix                 = "Cannot update in-progress endpoint"
	DeleteEndpointInProgressCode                          = "ValidationException"
	DescribeEndpoint404MessagePrefix                      = "Could not find endpoint"
	DescribeEndpoint404Code                               = "ValidationException"
	UpdateEndpoint404MessagePrefix                        = "Could not find endpoint"
	UpdateEndpoint404Code                                 = "ValidationException"
	UpdateEndpointUnableToFindEndpointConfigMessagePrefix = "Could not find endpoint configuration"
	UpdateEndpointUnableToFindEndpointConfigCode          = "ValidationException"

	DescribeEndpointConfig404MessagePrefix = "Could not find endpoint configuration"
	DescribeEndpointConfig404Code          = "ValidationException"
	DeleteEndpointConfig404MessagePrefix   = "Could not find endpoint configuration"
	DeleteEndpointConfig404Code            = "ValidationException"

	DescribeModel404MessagePrefix = "Could not find model"
	DescribeModel404Code          = "ValidationException"
	DeleteModel404MessagePrefix   = "Could not find model"
	DeleteModel404Code            = "ValidationException"
)

// SageMakerClientWrapper wraps the SageMaker client. "Not Found" errors are handled differently than in the Go SDK;
// here a method will return a nil pointer and a nil error if there is a 404. This simplifies code that interacts with
// SageMaker.
// Other errors are returned normally.
type SageMakerClientWrapper interface {
	DescribeTrainingJob(ctx context.Context, trainingJobName string) (*sagemaker.DescribeTrainingJobOutput, error)
	CreateTrainingJob(ctx context.Context, trainingJob *sagemaker.CreateTrainingJobInput) (*sagemaker.CreateTrainingJobOutput, error)
	StopTrainingJob(ctx context.Context, trainingJobName string) (*sagemaker.StopTrainingJobOutput, error)

	DescribeHyperParameterTuningJob(ctx context.Context, tuningJobName string) (*sagemaker.DescribeHyperParameterTuningJobOutput, error)
	CreateHyperParameterTuningJob(ctx context.Context, tuningJob *sagemaker.CreateHyperParameterTuningJobInput) (*sagemaker.CreateHyperParameterTuningJobOutput, error)
	StopHyperParameterTuningJob(ctx context.Context, tuningJobName string) (*sagemaker.StopHyperParameterTuningJobOutput, error)

	ListTrainingJobsForHyperParameterTuningJob(ctx context.Context, tuningJobName string) (sagemaker.ListTrainingJobsForHyperParameterTuningJobPaginator, error)

	DescribeEndpoint(ctx context.Context, endpointName string) (*sagemaker.DescribeEndpointOutput, error)
	CreateEndpoint(ctx context.Context, endpoint *sagemaker.CreateEndpointInput) (*sagemaker.CreateEndpointOutput, error)
	DeleteEndpoint(ctx context.Context, endpointName *string) (*sagemaker.DeleteEndpointOutput, error)
	UpdateEndpoint(ctx context.Context, endpointName, endpointConfigName string) (*sagemaker.UpdateEndpointOutput, error)

	DescribeModel(ctx context.Context, modelName string) (*sagemaker.DescribeModelOutput, error)
	CreateModel(ctx context.Context, model *sagemaker.CreateModelInput) (*sagemaker.CreateModelOutput, error)
	DeleteModel(ctx context.Context, model *sagemaker.DeleteModelInput) (*sagemaker.DeleteModelOutput, error)

	DescribeEndpointConfig(ctx context.Context, endpointConfigName string) (*sagemaker.DescribeEndpointConfigOutput, error)
	CreateEndpointConfig(ctx context.Context, endpointConfig *sagemaker.CreateEndpointConfigInput) (*sagemaker.CreateEndpointConfigOutput, error)
	DeleteEndpointConfig(ctx context.Context, endpointConfig *sagemaker.DeleteEndpointConfigInput) (*sagemaker.DeleteEndpointConfigOutput, error)
}

// NewSageMakerClientWrapper creates a SageMaker wrapper around an existing client.
func NewSageMakerClientWrapper(innerClient sagemakeriface.ClientAPI) SageMakerClientWrapper {
	return &sageMakerClientWrapper{
		innerClient: innerClient,
	}
}

// SageMakerClientWrapperProvider defines a function that returns a SageMaker client. Used for mocking.
type SageMakerClientWrapperProvider func(aws.Config) SageMakerClientWrapper

// Implementation of SageMaker client wrapper.
type sageMakerClientWrapper struct {
	SageMakerClientWrapper

	innerClient sagemakeriface.ClientAPI
}

// Return a training job description or nil if error or does not exist.
func (c *sageMakerClientWrapper) DescribeTrainingJob(ctx context.Context, trainingJobName string) (*sagemaker.DescribeTrainingJobOutput, error) {

	describeRequest := c.innerClient.DescribeTrainingJobRequest(&sagemaker.DescribeTrainingJobInput{
		TrainingJobName: &trainingJobName,
	})

	describeResponse, describeError := describeRequest.Send(ctx)

	if describeError != nil {
		if c.isDescribeTrainingJob404Error(describeError) {
			return nil, nil
		}
		return nil, describeError
	}

	return describeResponse.DescribeTrainingJobOutput, describeError
}

// Create a training job. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) CreateTrainingJob(ctx context.Context, trainingJob *sagemaker.CreateTrainingJobInput) (*sagemaker.CreateTrainingJobOutput, error) {

	createRequest := c.innerClient.CreateTrainingJobRequest(trainingJob)

	// Add `sagemaker-on-kubernetes` string literal to identify the k8s job in sagemaker
	aws.AddToUserAgent(createRequest.Request, controllers.SagemakerOnKubernetesUserAgentAddition)

	response, err := createRequest.Send(ctx)

	if response != nil {
		return response.CreateTrainingJobOutput, nil
	}

	return nil, err
}

// Stops a training job. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) StopTrainingJob(ctx context.Context, trainingJobName string) (*sagemaker.StopTrainingJobOutput, error) {
	stopRequest := c.innerClient.StopTrainingJobRequest(&sagemaker.StopTrainingJobInput{
		TrainingJobName: &trainingJobName,
	})

	stopResponse, stopError := stopRequest.Send(ctx)

	if stopError != nil {
		return nil, stopError
	}

	return stopResponse.StopTrainingJobOutput, nil
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isDescribeTrainingJob404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DescribeTrainingJob404Code && strings.HasPrefix(requestFailure.Message(), DescribeTrainingJob404MessagePrefix)
	}

	return false
}

// Return a training job description or nil if error or does not exist.
func (c *sageMakerClientWrapper) DescribeHyperParameterTuningJob(ctx context.Context, tuningJobName string) (*sagemaker.DescribeHyperParameterTuningJobOutput, error) {

	describeRequest := c.innerClient.DescribeHyperParameterTuningJobRequest(&sagemaker.DescribeHyperParameterTuningJobInput{
		HyperParameterTuningJobName: &tuningJobName,
	})

	describeResponse, describeError := describeRequest.Send(ctx)

	if describeError != nil {
		if c.isDescribeHyperParameterTuningJob404Error(describeError) {
			return nil, nil
		}
		return nil, describeError
	}

	return describeResponse.DescribeHyperParameterTuningJobOutput, describeError
}

// Create a training job. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) CreateHyperParameterTuningJob(ctx context.Context, tuningJob *sagemaker.CreateHyperParameterTuningJobInput) (*sagemaker.CreateHyperParameterTuningJobOutput, error) {

	createRequest := c.innerClient.CreateHyperParameterTuningJobRequest(tuningJob)

	// Add `sagemaker-on-kubernetes` string literal to identify the k8s job in sagemaker
	aws.AddToUserAgent(createRequest.Request, controllers.SagemakerOnKubernetesUserAgentAddition)

	response, err := createRequest.Send(ctx)

	if response != nil {
		return response.CreateHyperParameterTuningJobOutput, nil
	}

	return nil, err
}

// Stops a training job. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) StopHyperParameterTuningJob(ctx context.Context, tuningJobName string) (*sagemaker.StopHyperParameterTuningJobOutput, error) {
	stopRequest := c.innerClient.StopHyperParameterTuningJobRequest(&sagemaker.StopHyperParameterTuningJobInput{
		HyperParameterTuningJobName: &tuningJobName,
	})

	stopResponse, stopError := stopRequest.Send(ctx)

	if stopError != nil {
		return nil, stopError
	}

	return stopResponse.StopHyperParameterTuningJobOutput, nil
}

// Returns a paginator for iterating through the training jobs associated with a given hyperparameter tuning job. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) ListTrainingJobsForHyperParameterTuningJob(ctx context.Context, tuningJobName string) (sagemaker.ListTrainingJobsForHyperParameterTuningJobPaginator, error) {
	listRequest := c.innerClient.ListTrainingJobsForHyperParameterTuningJobRequest(&sagemaker.ListTrainingJobsForHyperParameterTuningJobInput{
		HyperParameterTuningJobName: &tuningJobName,
	})

	return sagemaker.NewListTrainingJobsForHyperParameterTuningJobPaginator(listRequest), nil
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isDescribeHyperParameterTuningJob404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DescribeHyperParameterTuningJob404Code && strings.HasPrefix(requestFailure.Message(), DescribeHyperParameterTuningJob404MessagePrefix)
	}

	return false
}

// Return a endpoint description or nil if error.
// If the object is not found, return a nil description and nil error.
func (c *sageMakerClientWrapper) DescribeEndpoint(ctx context.Context, endpointName string) (*sagemaker.DescribeEndpointOutput, error) {

	describeRequest := c.innerClient.DescribeEndpointRequest(&sagemaker.DescribeEndpointInput{
		EndpointName: &endpointName,
	})

	describeResponse, describeError := describeRequest.Send(ctx)

	if describeError != nil {
		if c.isDescribeEndpoint404Error(describeError) {
			return nil, nil
		}
		return nil, describeError
	}

	return describeResponse.DescribeEndpointOutput, describeError
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isDescribeEndpoint404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DescribeEndpoint404Code && strings.HasPrefix(requestFailure.Message(), DescribeEndpoint404MessagePrefix)
	}

	return false
}

// Create an Endpoint. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) CreateEndpoint(ctx context.Context, endpoint *sagemaker.CreateEndpointInput) (*sagemaker.CreateEndpointOutput, error) {

	createRequest := c.innerClient.CreateEndpointRequest(endpoint)

	// Add `sagemaker-on-kubernetes` string literal to identify the k8s job in sagemaker
	aws.AddToUserAgent(createRequest.Request, controllers.SagemakerOnKubernetesUserAgentAddition)

	response, err := createRequest.Send(ctx)

	if response != nil {
		return response.CreateEndpointOutput, nil
	}

	return nil, err
}

// Delete an Endpoint. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) DeleteEndpoint(ctx context.Context, endpointName *string) (*sagemaker.DeleteEndpointOutput, error) {
	deleteRequest := c.innerClient.DeleteEndpointRequest(&sagemaker.DeleteEndpointInput{
		EndpointName: endpointName,
	})

	deleteResponse, deleteError := deleteRequest.Send(ctx)

	if deleteError != nil {
		return nil, deleteError
	}

	return deleteResponse.DeleteEndpointOutput, nil
}

// Delete an Endpoint. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) UpdateEndpoint(ctx context.Context, endpointName, endpointConfigName string) (*sagemaker.UpdateEndpointOutput, error) {
	updateRequest := c.innerClient.UpdateEndpointRequest(&sagemaker.UpdateEndpointInput{
		EndpointName:       &endpointName,
		EndpointConfigName: &endpointConfigName,
	})

	updateResponse, updateError := updateRequest.Send(ctx)

	if updateError != nil {
		return nil, updateError
	}

	return updateResponse.UpdateEndpointOutput, nil
}

// Return a model description or nil if error.
// If the object is not found, return a nil description and nil error.
func (c *sageMakerClientWrapper) DescribeModel(ctx context.Context, modelName string) (*sagemaker.DescribeModelOutput, error) {
	describeRequest := c.innerClient.DescribeModelRequest(&sagemaker.DescribeModelInput{
		ModelName: &modelName,
	})

	describeResponse, describeError := describeRequest.Send(ctx)
	if describeError != nil {
		if c.isDescribeModel404Error(describeError) {
			return nil, nil
		}
		return nil, describeError
	}

	return describeResponse.DescribeModelOutput, describeError
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isDescribeModel404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DescribeModel404Code && strings.HasPrefix(requestFailure.Message(), DescribeModel404MessagePrefix)
	}

	return false
}

// Create a model. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) CreateModel(ctx context.Context, model *sagemaker.CreateModelInput) (*sagemaker.CreateModelOutput, error) {

	createRequest := c.innerClient.CreateModelRequest(model)

	// Add `sagemaker-on-kubernetes` string literal to identify the k8s job in sagemaker
	aws.AddToUserAgent(createRequest.Request, controllers.SagemakerOnKubernetesUserAgentAddition)

	response, err := createRequest.Send(ctx)

	if response != nil {
		return response.CreateModelOutput, nil
	}

	return nil, err
}

// Return a model delete or nil if error.
// If the object is not found, return a nil description and nil error.
func (c *sageMakerClientWrapper) DeleteModel(ctx context.Context, model *sagemaker.DeleteModelInput) (*sagemaker.DeleteModelOutput, error) {

	deleteRequest := c.innerClient.DeleteModelRequest(model)

	deleteResponse, deleteError := deleteRequest.Send(ctx)

	if deleteError != nil {
		return nil, deleteError
	}
	return deleteResponse.DeleteModelOutput, deleteError
}

// Return a endpointconfig description or nil if error.
// If the object is not found, return a nil description and nil error.
func (c *sageMakerClientWrapper) DescribeEndpointConfig(ctx context.Context, endpointconfigName string) (*sagemaker.DescribeEndpointConfigOutput, error) {
	describeRequest := c.innerClient.DescribeEndpointConfigRequest(&sagemaker.DescribeEndpointConfigInput{
		EndpointConfigName: &endpointconfigName,
	})

	describeResponse, describeError := describeRequest.Send(ctx)
	if describeError != nil {
		if c.isDescribeEndpointConfig404Error(describeError) {
			return nil, nil
		}
		return nil, describeError
	}

	return describeResponse.DescribeEndpointConfigOutput, describeError
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isDescribeEndpointConfig404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DescribeEndpointConfig404Code && strings.HasPrefix(requestFailure.Message(), DescribeEndpointConfig404MessagePrefix)
	}

	return false
}

// Create an EndpointConfig. Returns the response output or nil if error.
func (c *sageMakerClientWrapper) CreateEndpointConfig(ctx context.Context, endpointconfig *sagemaker.CreateEndpointConfigInput) (*sagemaker.CreateEndpointConfigOutput, error) {

	createRequest := c.innerClient.CreateEndpointConfigRequest(endpointconfig)

	// Add `sagemaker-on-kubernetes` string literal to identify the k8s job in sagemaker
	aws.AddToUserAgent(createRequest.Request, controllers.SagemakerOnKubernetesUserAgentAddition)

	response, err := createRequest.Send(ctx)

	if response != nil {
		return response.CreateEndpointConfigOutput, nil
	}

	return nil, err
}

//  Return a EndpointConfig delete response output or nil if error
//  If the EndpointConfig is not found, return a nil description and nil error
func (c *sageMakerClientWrapper) DeleteEndpointConfig(ctx context.Context, endpointConfig *sagemaker.DeleteEndpointConfigInput) (*sagemaker.DeleteEndpointConfigOutput, error) {
	deleteRequest := c.innerClient.DeleteEndpointConfigRequest(endpointConfig)

	deleteResponse, deleteError := deleteRequest.Send(ctx)
	if deleteError != nil {
		return nil, deleteError
	}

	return deleteResponse.DeleteEndpointConfigOutput, deleteError
}

// The SageMaker API does not conform to the HTTP standard. The following methods detect
// if a SageMaker error response is equivalent to an HTTP 404 not found.

// IsDeleteEndpointConfig404Error determines whether the given error is equivalent to an HTTP 404 status code.
func IsDeleteEndpointConfig404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DeleteEndpointConfig404Code && strings.HasPrefix(requestFailure.Message(), DeleteEndpointConfig404MessagePrefix)
	}

	return false
}

// IsDeleteModel404Error determines whether the given error is equivalent to an HTTP 404 status code.
func IsDeleteModel404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DeleteModel404Code && strings.HasPrefix(requestFailure.Message(), DeleteModel404MessagePrefix)
	}

	return false
}

// IsDeleteEndpoint404Error determines whether the given error is equivalent to an HTTP 404 status code.
func IsDeleteEndpoint404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DeleteEndpoint404Code && strings.HasPrefix(requestFailure.Message(), DeleteEndpoint404MessagePrefix)
	}

	return false
}

func isUpdateEndpointUnableToFindEndpointConfigurationError(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == UpdateEndpointUnableToFindEndpointConfigCode && strings.HasPrefix(requestFailure.Message(), UpdateEndpointUnableToFindEndpointConfigMessagePrefix)
	}

	return false
}

// IsUpdateEndpoint404Error determines whether the given error is equivalent to an HTTP 404 status code.
func IsUpdateEndpoint404Error(err error) bool {
	// Unfortunately both of these errors have the same prefix. We must check that it is 404 for Endpoint and not 404 for EndpointConfig.
	// SageMaker will return 404 if the original (non-updating) EndpointConfig does not exist.
	if isUpdateEndpointUnableToFindEndpointConfigurationError(err) {
		return false
	}

	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == UpdateEndpoint404Code && strings.HasPrefix(requestFailure.Message(), UpdateEndpoint404MessagePrefix)
	}

	return false
}

// IsStopTrainingJob404Error determines whether the given error is equivalent to an HTTP 404 status code.
func IsStopTrainingJob404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == StopTrainingJob404Code && strings.HasPrefix(requestFailure.Message(), StopTrainingJob404MessagePrefix)
	}

	return false
}

func IsStopHyperParameterTuningJob404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == StopHyperParameterTuningJob404Code && strings.HasPrefix(requestFailure.Message(), StopHyperParameterTuningJob404MessagePrefix)
	}

	return false
}
