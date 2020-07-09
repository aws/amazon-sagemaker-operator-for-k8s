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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/applicationautoscalingiface"
)

// Provides the prefixes and error codes relating to each endpoint
const (
	DescribeAutoscalingJob404Code          = "ValidationException"
	DescribeAutoscalingJob404MessagePrefix = "Requested resource not found"
	StopAutoscalingJob404Code              = "ValidationException"
	StopAutoscalingJob404MessagePrefix     = "Requested resource not found"
)

// ApplicationAutoscalingClientWrapper wraps the Autoscaling client. "Not Found" errors are handled differently than in the Go SDK;
// here a method will return a nil pointer and a nil error if there is a 404. TODO Meghna: check if error handling can be normal.
// Other errors are returned normally.
type ApplicationAutoscalingClientWrapper interface {
	RegisterScalableTarget(ctx context.Context, autoscalingTarget *applicationautoscaling.RegisterScalableTargetInput) (*applicationautoscaling.RegisterScalableTargetOutput, error)
	PutScalingPolicy(ctx context.Context, autoscalingJob *applicationautoscaling.PutScalingPolicyInput) (*applicationautoscaling.PutScalingPolicyOutput, error)
	DeleteScalingPolicy(ctx context.Context, autoscalingJob *applicationautoscaling.DeleteScalingPolicyInput) (*applicationautoscaling.DeleteScalingPolicyOutput, error)
	DeregisterScalableTarget(ctx context.Context, autoscalingJob *applicationautoscaling.DeregisterScalableTargetInput) (*applicationautoscaling.DeregisterScalableTargetOutput, error)
	//DescribeScalableTargets(ctx context.Context, trainingJobName string) (*applicationautoscaling.StopTrainingJobOutput, error)
	DescribeScalingPolicies(ctx context.Context, policyName string, resourceId string) (*applicationautoscaling.DescribeScalingPoliciesOutput, error)
}

// NewApplicationAutoscalingClientWrapper creates a ApplicationAutoscaling wrapper around an existing client.
func NewApplicationAutoscalingClientWrapper(innerClient applicationautoscalingiface.ClientAPI) ApplicationAutoscalingClientWrapper {
	return &applicationAutoscalingClientWrapper{
		innerClient: innerClient,
	}
}

// ApplicationAutoscalingClientWrapperProvider defines a function that returns a ApplicationAutoscaling client. Used for mocking.
type ApplicationAutoscalingClientWrapperProvider func(aws.Config) ApplicationAutoscalingClientWrapper

// Implementation of ApplicationAutoscaling client wrapper.
type applicationAutoscalingClientWrapper struct {
	ApplicationAutoscalingClientWrapper

	// TODO: what does this become
	innerClient applicationautoscalingiface.ClientAPI
}

// RegisterScalableTarget registers a scalable target. Returns the response output or nil if error.
func (c *applicationAutoscalingClientWrapper) RegisterScalableTarget(ctx context.Context, autoscalingTarget *applicationautoscaling.RegisterScalableTargetInput) (*applicationautoscaling.RegisterScalableTargetOutput, error) {

	createRequest := c.innerClient.RegisterScalableTargetRequest(autoscalingTarget)

	// TODO Meghna: Check if this is needed, probably not.
	// Add `sagemaker-on-kubernetes` string literal to identify the k8s job in sagemaker

	response, err := createRequest.Send(ctx)

	if response != nil {
		return response.RegisterScalableTargetOutput, nil
	}

	return nil, err
}

// RegisterScalableTarget registers a scalable target. Returns the response output or nil if error.
func (c *applicationAutoscalingClientWrapper) PutScalingPolicy(ctx context.Context, autoscalingJob *applicationautoscaling.PutScalingPolicyInput) (*applicationautoscaling.PutScalingPolicyOutput, error) {

	createRequest := c.innerClient.PutScalingPolicyRequest(autoscalingJob)

	response, err := createRequest.Send(ctx)

	if response != nil {
		return response.PutScalingPolicyOutput, nil
	}

	return nil, err
}

// Stops a training job. Returns the response output or nil if error.
func (c *applicationAutoscalingClientWrapper) DeleteScalingPolicy(ctx context.Context, autoscalingJob *applicationautoscaling.DeleteScalingPolicyInput) (*applicationautoscaling.DeleteScalingPolicyOutput, error) {
	deleteRequest := c.innerClient.DeleteScalingPolicyRequest(autoscalingJob)

	deleteResponse, deleteError := deleteRequest.Send(ctx)

	if deleteError != nil {
		return nil, deleteError
	}

	return deleteResponse.DeleteScalingPolicyOutput, deleteError
}

// Stops a training job. Returns the response output or nil if error.
func (c *applicationAutoscalingClientWrapper) DeregisterScalableTarget(ctx context.Context, autoscalingJob *applicationautoscaling.DeregisterScalableTargetInput) (*applicationautoscaling.DeregisterScalableTargetOutput, error) {
	deleteRequest := c.innerClient.DeregisterScalableTargetRequest(autoscalingJob)

	deleteResponse, deleteError := deleteRequest.Send(ctx)

	if deleteError != nil {
		return nil, deleteError
	}

	return deleteResponse.DeregisterScalableTargetOutput, deleteError
}

// Stops a training job. Returns the response output or nil if error.
func (c *applicationAutoscalingClientWrapper) DescribeScalingPolicies(ctx context.Context, policyName string, resourceId string) (*applicationautoscaling.DescribeScalingPoliciesOutput, error) {

	var policyNameList []string
	policyNameList = append(policyNameList, policyName)
	var maxResults int64 = 1

	describeRequest := c.innerClient.DescribeScalingPoliciesRequest(&applicationautoscaling.DescribeScalingPoliciesInput{
		PolicyNames:       policyNameList,
		MaxResults:        &maxResults,
		ResourceId:        &resourceId,
		ScalableDimension: "sagemaker:variant:DesiredInstanceCount",
		ServiceNamespace:  "sagemaker",
	})

	describeResponse, describeError := describeRequest.Send(ctx)

	if describeError != nil {
		return nil, describeError
	}

	return describeResponse.DescribeScalingPoliciesOutput, describeError
}

/*
// The SageMaker API does not conform to the HTTP standard. This detects if a SageMaker error response is equivalent
// to an HTTP 404 not found.
func (c *autoscalingClientWrapper) isDescribeAutoscalingJob404Error(err error) bool {
	if requestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == DescribeAutoscalingJob404Code && strings.HasPrefix(requestFailure.Message(), DescribeTrainingJob404MessagePrefix)
	}

	return false
}
*/
