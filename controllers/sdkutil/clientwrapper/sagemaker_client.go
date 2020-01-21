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
	CreateTrainingJob(ctx context.Context, trainingJobName string) (*sagemaker.CreateTrainingJobOutput, error)
	StopTrainingJob(ctx context.Context, trainingJobName string) (*sagemaker.StopTrainingJobOutput, error)

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

// Implementation of SageMaker client wrapper.
type sageMakerClientWrapper struct {
	SageMakerClientWrapper

	innerClient sagemakeriface.ClientAPI
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
		return requestFailure.Code() == "ValidationException" && strings.HasPrefix(requestFailure.Message(), "Could not find endpoint")
	}

	return false
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isDeleteEndpoint404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DeleteEndpoint404Code && strings.HasPrefix(requestFailure.Message(), DeleteEndpoint404MessagePrefix)
	}

	return false
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isUpdateEndpoint404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == UpdateEndpoint404Code && strings.HasPrefix(requestFailure.Message(), UpdateEndpoint404MessagePrefix)
	}

	return false
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isUpdateEndpointUnableToFindEndpointConfigurationError(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == UpdateEndpointUnableToFindEndpointConfigCode && strings.HasPrefix(requestFailure.Message(), UpdateEndpointUnableToFindEndpointConfigMessagePrefix)
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
		if c.isDeleteEndpoint404Error(deleteError) {
			return nil, nil
		}
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

		// Unfortunately both of these errors have the same prefix. We must check that it is 404 for Endpoint and not 404 for EndpointConfig.
		// SageMaker will return 404 if the original (non-updating) EndpointConfig does not exist.
		if c.isUpdateEndpoint404Error(updateError) && !c.isUpdateEndpointUnableToFindEndpointConfigurationError(updateError) {
			return nil, nil
		}
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
		if c.isDeleteModel404Error(deleteError) {
			return nil, nil
		}
		return nil, deleteError
	}
	return deleteResponse.DeleteModelOutput, deleteError
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isDeleteModel404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DeleteModel404Code && strings.HasPrefix(requestFailure.Message(), DeleteModel404MessagePrefix)
	}

	return false
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
		if c.isDeleteEndpointConfig404Error(deleteError) {
			//TODO: success and 404 both returns nil, nil
			//      Add a mechanism to separate them.
			return nil, nil
		}
		return nil, deleteError
	}

	return deleteResponse.DeleteEndpointConfigOutput, deleteError
}

// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *sageMakerClientWrapper) isDeleteEndpointConfig404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DeleteEndpointConfig404Code && strings.HasPrefix(requestFailure.Message(), DeleteEndpointConfig404MessagePrefix)
	}

	return false
}
