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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling/applicationautoscalingiface"
	"github.com/pkg/errors"
)

// Provides error codes and messages
const (
	HAPObjectNotFoundExceptionCode   = "ObjectNotFoundException"
	HAPInternalServiceExceptionCode  = "InternalServiceException"
	HAPConcurrentUpdateExceptionCode = "ConcurrentUpdateException"
	HAPValidationExceptionCode       = "ValidationException"
	HDPendingMessage                 = "The status should be in 'InService' to register it as a scalable target"
)

// ApplicationAutoscalingClientWrapper interface for ApplicationAutoscalingClient wrapper
type ApplicationAutoscalingClientWrapper interface {
	RegisterScalableTarget(ctx context.Context, autoscalingTarget *applicationautoscaling.RegisterScalableTargetInput) (*applicationautoscaling.RegisterScalableTargetOutput, error)
	PutScalingPolicy(ctx context.Context, autoscalingJob *applicationautoscaling.PutScalingPolicyInput) (*applicationautoscaling.PutScalingPolicyOutput, error)
	DeleteScalingPolicy(ctx context.Context, autoscalingJob *applicationautoscaling.DeleteScalingPolicyInput) (*applicationautoscaling.DeleteScalingPolicyOutput, error)
	DeregisterScalableTarget(ctx context.Context, autoscalingJob *applicationautoscaling.DeregisterScalableTargetInput) (*applicationautoscaling.DeregisterScalableTargetOutput, error)
	DescribeScalableTargets(ctx context.Context, resourceID string) (*applicationautoscaling.DescribeScalableTargetsOutput, error)
	DescribeScalingPolicies(ctx context.Context, policyName string, resourceID string) (*applicationautoscaling.ScalingPolicy, error)
}

// NewApplicationAutoscalingClientWrapper creates a ApplicationAutoscaling wrapper around an existing client.
func NewApplicationAutoscalingClientWrapper(innerClient applicationautoscalingiface.ApplicationAutoScalingAPI) ApplicationAutoscalingClientWrapper {
	return &applicationAutoscalingClientWrapper{
		innerClient: innerClient,
	}
}

// ApplicationAutoscalingClientWrapperProvider defines a function that returns a ApplicationAutoscaling client. Used for mocking.
type ApplicationAutoscalingClientWrapperProvider func(aws.Config) ApplicationAutoscalingClientWrapper

// Implementation of ApplicationAutoscaling client wrapper.
type applicationAutoscalingClientWrapper struct {
	ApplicationAutoscalingClientWrapper
	innerClient applicationautoscalingiface.ApplicationAutoScalingAPI
}

// RegisterScalableTarget registers a scalable target. Returns the response output or nil if error.
func (c *applicationAutoscalingClientWrapper) RegisterScalableTarget(ctx context.Context, autoscalingTarget *applicationautoscaling.RegisterScalableTargetInput) (*applicationautoscaling.RegisterScalableTargetOutput, error) {

	createRequest, response := c.innerClient.RegisterScalableTargetRequest(autoscalingTarget)
	err := createRequest.Send()

	if response != nil {
		return response, nil
	}
	return nil, err
}

// RegisterScalableTarget registers a scalable target. Returns the response output or nil if error.
func (c *applicationAutoscalingClientWrapper) PutScalingPolicy(ctx context.Context, autoscalingJob *applicationautoscaling.PutScalingPolicyInput) (*applicationautoscaling.PutScalingPolicyOutput, error) {

	createRequest, response := c.innerClient.PutScalingPolicyRequest(autoscalingJob)
	err := createRequest.Send()

	if response != nil {
		return response, nil
	}

	return nil, err
}

// DeleteScalingPolicy Deletes the scaling policy
func (c *applicationAutoscalingClientWrapper) DeleteScalingPolicy(ctx context.Context, autoscalingJob *applicationautoscaling.DeleteScalingPolicyInput) (*applicationautoscaling.DeleteScalingPolicyOutput, error) {
	deleteRequest, deleteResponse := c.innerClient.DeleteScalingPolicyRequest(autoscalingJob)
	deleteError := deleteRequest.Send()

	if deleteError != nil {
		return nil, deleteError
	}

	return deleteResponse, deleteError
}

// DeregisterScalableTarget deregisters a scalable target
func (c *applicationAutoscalingClientWrapper) DeregisterScalableTarget(ctx context.Context, autoscalingJob *applicationautoscaling.DeregisterScalableTargetInput) (*applicationautoscaling.DeregisterScalableTargetOutput, error) {
	deleteRequest, deleteResponse := c.innerClient.DeregisterScalableTargetRequest(autoscalingJob)
	deleteError := deleteRequest.Send()

	if deleteError != nil {
		return nil, deleteError
	}

	return deleteResponse, deleteError
}

// DescribeScalableTargets returns the scalableTarget description filtered on PolicyName and a single ResourceID
// TODO: change this to return only the ScalableTargetObject for cleaner descriptions
func (c *applicationAutoscalingClientWrapper) DescribeScalableTargets(ctx context.Context, resourceID string) (*applicationautoscaling.DescribeScalableTargetsOutput, error) {

	var resourceIDList []*string
	resourceIDList = append(resourceIDList, &resourceID)
	// Review: This filtered response should be of size 1 by default
	var maxResults int64 = 1

	// TODO: Remove hardcoded values, might need to construct the input object
	describeRequest, describeResponse := c.innerClient.DescribeScalableTargetsRequest(&applicationautoscaling.DescribeScalableTargetsInput{
		ResourceIds:       resourceIDList,
		MaxResults:        &maxResults,
		ScalableDimension: aws.String("sagemaker:variant:DesiredInstanceCount"),
		ServiceNamespace:  aws.String("sagemaker"),
	})

	describeError := describeRequest.Send()

	if describeError != nil {
		return nil, describeError
	}

	return describeResponse, describeError
}

// DescribeScalingPolicies returns the scaling policy description filtered on PolicyName and a single ResourceID
// returns only the scalingPolicy object else the actionDetermination gets messy
func (c *applicationAutoscalingClientWrapper) DescribeScalingPolicies(ctx context.Context, policyName string, resourceID string) (*applicationautoscaling.ScalingPolicy, error) {

	var policyNameList []*string
	var scalingPolicyDescription *applicationautoscaling.ScalingPolicy
	policyNameList = append(policyNameList, &policyName)
	// Review: This filtered response should be of size 1 by default
	var maxResults int64 = 1

	// TODO: Remove hardcoded values, might need to construct the inputs
	describeRequest, describeResponse := c.innerClient.DescribeScalingPoliciesRequest(&applicationautoscaling.DescribeScalingPoliciesInput{
		PolicyNames:       policyNameList,
		MaxResults:        &maxResults,
		ResourceId:        &resourceID,
		ScalableDimension: aws.String("sagemaker:variant:DesiredInstanceCount"),
		ServiceNamespace:  aws.String("sagemaker"),
	})

	describeError := describeRequest.Send()

	if describeError != nil {
		return scalingPolicyDescription, describeError
	}

	// Review: Slightly Hacky, but valid
	if len(describeResponse.ScalingPolicies) == 1 {
		scalingPolicyDescription = describeResponse.ScalingPolicies[0]
	} else {
		scalingPolicyDescription = nil
	}

	return scalingPolicyDescription, describeError
}

// IsDeleteHAP404Error determines whether the given error HAPObjectNotFoundExceptionCode, used for deletion.
func IsDeleteHAP404Error(err error) bool {
	awserror := errors.Cause(err)
	if requestFailure, isRequestFailure := awserror.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == HAPObjectNotFoundExceptionCode
	}

	return false
}

// IsDescribeHAP404Error detects if the error is a HAPValidationExceptionCode
func IsDescribeHAP404Error(err error) bool {
	awserror := errors.Cause(err)
	if requestFailure, isRequestFailure := awserror.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == HAPValidationExceptionCode
	}
	return false
}

// IsHAPInternalServiceExceptionError detects if the error is an InternalServiceException
func IsHAPInternalServiceExceptionError(err error) bool {
	awserror := errors.Cause(err)
	if requestFailure, isRequestFailure := awserror.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == HAPInternalServiceExceptionCode
	}
	return false
}

// IsHAPConcurrentUpdateExceptionError detects if the error is a ConcurrentUpdateException
func IsHAPConcurrentUpdateExceptionError(err error) bool {
	awserror := errors.Cause(err)
	if requestFailure, isRequestFailure := awserror.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == HAPConcurrentUpdateExceptionCode
	}
	return false
}

// IsHDPendingError is for the special case when the HD is in creating/updating status and HAP throws a validationException. Status is set to reconciling.
func IsHDPendingError(err error) bool {
	awserror := errors.Cause(err)
	if requestFailure, isRequestFailure := awserror.(awserr.RequestFailure); isRequestFailure {
		return requestFailure.Code() == HAPValidationExceptionCode && strings.Contains(requestFailure.Message(), HDPendingMessage)

	}
	return false
}
