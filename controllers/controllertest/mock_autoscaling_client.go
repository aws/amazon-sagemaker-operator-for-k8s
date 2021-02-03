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

package controllertest

import (
	. "container/list"
	"fmt"
	. "github.com/onsi/ginkgo"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling/applicationautoscalingiface"
)

// NewMockAutoscalingClientBuilder returns a MockAuoscalingClientBuilder
func NewMockAutoscalingClientBuilder(testReporter GinkgoTInterface) *MockAutoscalingClientBuilder {
	builder := MockAutoscalingClientBuilder{
		testReporter: testReporter,
	}
	return &builder
}

// MockAutoscalingClientBuilder is a Builder for mock Autoscaling API clients
type MockAutoscalingClientBuilder struct {
	// Used to fail tests when not enough responses are provided for requests.
	testReporter GinkgoTInterface
	// Used to store responses that the Server will respond with. The mock client responds
	// with responses in the order that they were added, i.e. with AddDescribeTrainingJobResponse.
	responses List
	// Used to store requests received by Autoscaling client.
	requests *List
}

// Helper data structure that represents a single describeScalableTargetsResponse.
type describeScalableTargetsResponse struct {
	err        awserr.RequestFailure
	targetData *applicationautoscaling.DescribeScalableTargetsOutput
}

// Helper data structure that represents a single registerScalableTargetResponse.
type registerScalableTargetResponse struct {
	err        awserr.RequestFailure
	targetData *applicationautoscaling.RegisterScalableTargetOutput
}

// Helper data structure that represents a single describeScalingPolicyResponse from the innerClient
type describeScalingPolicyResponse struct {
	err        awserr.RequestFailure
	policyData *applicationautoscaling.DescribeScalingPoliciesOutput
}

// Helper data structure that represents a single putScalingPolicyResponse.
type putScalingPolicyResponse struct {
	err        awserr.RequestFailure
	policyData *applicationautoscaling.PutScalingPolicyOutput
}

// Helper data structure that represents a single putScalingPolicyResponse.
type deleteScalingPolicyResponse struct {
	err        awserr.RequestFailure
	policyData *applicationautoscaling.DeleteScalingPolicyOutput
}

// Helper data structure that represents a single putScalingPolicyResponse.
type deregisterScalableTargetResponse struct {
	err        awserr.RequestFailure
	targetData *applicationautoscaling.DeregisterScalableTargetOutput
}

// AddDescribeScalingPoliciesErrorResponse Add a DescribeScalingPolicy error response to the client.
func (m *MockAutoscalingClientBuilder) AddDescribeScalingPoliciesErrorResponse(code string, message string, statusCode int, reqID string) *MockAutoscalingClientBuilder {
	m.responses.PushBack(describeScalingPolicyResponse{
		err:        awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqID),
		policyData: nil,
	})
	return m
}

// AddDescribeScalingPoliciesResponse Add a DescribeScalingPolicy response to the client.
func (m *MockAutoscalingClientBuilder) AddDescribeScalingPoliciesResponse(data applicationautoscaling.DescribeScalingPoliciesOutput) *MockAutoscalingClientBuilder {
	m.responses.PushBack(describeScalingPolicyResponse{
		err:        nil,
		policyData: &data,
	})

	return m
}

// AddDescribeScalingPoliciesEmptyResponse Add a DescribeScalingPolicy response to the client.
func (m *MockAutoscalingClientBuilder) AddDescribeScalingPoliciesEmptyResponse() *MockAutoscalingClientBuilder {
	m.responses.PushBack(nil)

	return m
}

// AddPutScalingPolicyErrorResponse Add a PutScalingPolicy error response to the client.
func (m *MockAutoscalingClientBuilder) AddPutScalingPolicyErrorResponse(code string, message string, statusCode int, reqID string) *MockAutoscalingClientBuilder {
	m.responses.PushBack(putScalingPolicyResponse{
		err:        awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqID),
		policyData: nil,
	})
	return m
}

// AddPutScalingPolicyResponse Add a PutScalingPolicy response to the client.
func (m *MockAutoscalingClientBuilder) AddPutScalingPolicyResponse(data applicationautoscaling.PutScalingPolicyOutput) *MockAutoscalingClientBuilder {
	m.responses.PushBack(putScalingPolicyResponse{
		err:        nil,
		policyData: &data,
	})

	return m
}

// AddDeleteScalingPolicyErrorResponse Add a PutScalingPolicy error response to the client.
func (m *MockAutoscalingClientBuilder) AddDeleteScalingPolicyErrorResponse(code string, message string, statusCode int, reqID string) *MockAutoscalingClientBuilder {
	m.responses.PushBack(putScalingPolicyResponse{
		err:        awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqID),
		policyData: nil,
	})
	return m
}

// AddDeleteScalingPolicyResponse Add a PutScalingPolicy response to the client.
func (m *MockAutoscalingClientBuilder) AddDeleteScalingPolicyResponse(data applicationautoscaling.DeleteScalingPolicyOutput) *MockAutoscalingClientBuilder {
	m.responses.PushBack(deleteScalingPolicyResponse{
		err:        nil,
		policyData: &data,
	})
	return m
}

// AddDescribeScalableTargetsErrorResponse Add a DescribeScalingPolicy error response to the client.
func (m *MockAutoscalingClientBuilder) AddDescribeScalableTargetsErrorResponse(code string, message string, statusCode int, reqID string) *MockAutoscalingClientBuilder {
	m.responses.PushBack(describeScalableTargetsResponse{
		err:        awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqID),
		targetData: nil,
	})
	return m
}

// AddDescribeScalableTargetsResponse Add a DescribeScalingPolicy response to the client.
func (m *MockAutoscalingClientBuilder) AddDescribeScalableTargetsResponse(data applicationautoscaling.DescribeScalableTargetsOutput) *MockAutoscalingClientBuilder {
	m.responses.PushBack(describeScalableTargetsResponse{
		err:        nil,
		targetData: &data,
	})

	return m
}

// AddRegisterScalableTargetsErrorResponse Add a PutScalingPolicy error response to the client.
func (m *MockAutoscalingClientBuilder) AddRegisterScalableTargetsErrorResponse(code string, message string, statusCode int, reqID string) *MockAutoscalingClientBuilder {
	m.responses.PushBack(registerScalableTargetResponse{
		err:        awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqID),
		targetData: nil,
	})
	return m
}

// AddRegisterScalableTargetsResponse Add a PutScalingPolicy response to the client.
func (m *MockAutoscalingClientBuilder) AddRegisterScalableTargetsResponse(data applicationautoscaling.RegisterScalableTargetOutput) *MockAutoscalingClientBuilder {
	m.responses.PushBack(registerScalableTargetResponse{
		err:        nil,
		targetData: &data,
	})

	return m
}

// AddDeregisterScalableTargetsErrorResponse Add a PutScalingPolicy error response to the client.
func (m *MockAutoscalingClientBuilder) AddDeregisterScalableTargetsErrorResponse(code string, message string, statusCode int, reqID string) *MockAutoscalingClientBuilder {
	m.responses.PushBack(deregisterScalableTargetResponse{
		err:        awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqID),
		targetData: nil,
	})
	return m
}

// AddDeregisterScalableTargetsResponse Add a PutScalingPolicy response to the client.
func (m *MockAutoscalingClientBuilder) AddDeregisterScalableTargetsResponse(data applicationautoscaling.DeregisterScalableTargetOutput) *MockAutoscalingClientBuilder {
	m.responses.PushBack(deregisterScalableTargetResponse{
		err:        nil,
		targetData: &data,
	})

	return m
}

// WithRequestList Store requests received by the mock client in a user-provided list.
func (m *MockAutoscalingClientBuilder) WithRequestList(requests *List) *MockAutoscalingClientBuilder {
	m.requests = requests
	return m
}

// Build Create a mock ApplicationAutoscaling API client given configuration.
func (m *MockAutoscalingClientBuilder) Build() applicationautoscalingiface.ClientAPI {

	if m.testReporter == nil {
		panic("MockAutoscalingClientBuilder requires non-nil test reporter.")
	}

	if m.requests == nil {
		m.requests = &List{}
	}

	applicationAutoscalingClient := mockApplicationAutoscalingClient{
		responses:    &m.responses,
		requests:     m.requests,
		testReporter: m.testReporter,
	}

	return applicationAutoscalingClient
}

// Mock ApplicationAutoscaling API client.
type mockApplicationAutoscalingClient struct {
	applicationautoscalingiface.ClientAPI

	// List of responses to use when responding to API calls. They are returned in same order
	// as they are stored in the list.
	responses *List

	// List of requests that are received. They are stored in the same order that they are received.
	requests *List

	// Test reporter used to fail tests if not enough responses were provided.
	testReporter GinkgoTInterface
}

func (m *mockApplicationAutoscalingClient) mockRequestBuilder() *aws.Request {
	return &aws.Request{
		HTTPRequest: &http.Request{
			Header: map[string][]string{},
		},
		HTTPResponse: &http.Response{},
		Retryer:      &aws.NoOpRetryer{},
		// Required for pagination operation.
		Operation: &aws.Operation{
			Paginator: nil,
		},
	}
}

// Mock DescribeScalableTargetsRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type ScalableTargetsResponse, or there are no more responses to give, fail the test.
func (m mockApplicationAutoscalingClient) DescribeScalableTargetsRequest(input *applicationautoscaling.DescribeScalableTargetsInput) applicationautoscaling.DescribeScalableTargetsRequest {
	m.requests.PushBack(input)
	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DescribeScalableTargets responses provided for test"
		nextResponse = describeScalableTargetsResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextdescribeScalableTargetsResponse, ok := nextResponse.(describeScalableTargetsResponse)
	if !ok {
		message := "describeScalableTarget request created, next response is not of type describeScalableTargetOutput"
		nextdescribeScalableTargetsResponse = describeScalableTargetsResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextdescribeScalableTargetsResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextdescribeScalableTargetsResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextdescribeScalableTargetsResponse.targetData
		})
	}

	return applicationautoscaling.DescribeScalableTargetsRequest{
		Request: mockRequest,
	}
}

// Mock DescribeScalingPoliciesRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type ScalableTargetsResponse, or there are no more responses to give, fail the test.
func (m mockApplicationAutoscalingClient) DescribeScalingPoliciesRequest(input *applicationautoscaling.DescribeScalingPoliciesInput) applicationautoscaling.DescribeScalingPoliciesRequest {
	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DescribeScalingPolicies responses provided for test"
		nextResponse = describeScalingPolicyResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextdescribeScalingPolicyResponse, ok := nextResponse.(describeScalingPolicyResponse)
	if !ok {
		message := "DescribeScalingPoliciesRequest created, next response is not of type DescribeScalingPoliciesOutput"
		nextdescribeScalingPolicyResponse = describeScalingPolicyResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextdescribeScalingPolicyResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextdescribeScalingPolicyResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextdescribeScalingPolicyResponse.policyData
		})
	}

	return applicationautoscaling.DescribeScalingPoliciesRequest{
		Request: mockRequest,
	}
}

// Mock RegisterScalableTargetRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type RegisterScalableTarget, or there are no more responses to give, fail the test.
func (m mockApplicationAutoscalingClient) RegisterScalableTargetRequest(input *applicationautoscaling.RegisterScalableTargetInput) applicationautoscaling.RegisterScalableTargetRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough RegisterScalableTarget responses provided for test"
		nextResponse = registerScalableTargetResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextregisterScalableTargetResponse, ok := nextResponse.(registerScalableTargetResponse)
	if !ok {
		message := "registerScalableTarget request created, next response is not of type registerScalableTargetOutput"
		nextregisterScalableTargetResponse = registerScalableTargetResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextregisterScalableTargetResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextregisterScalableTargetResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextregisterScalableTargetResponse.targetData
		})
	}

	return applicationautoscaling.RegisterScalableTargetRequest{
		Request: mockRequest,
	}
}

// Mock PutScalingPolicyRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type PutScalingPolicy, or there are no more responses to give, fail the test.
func (m mockApplicationAutoscalingClient) PutScalingPolicyRequest(input *applicationautoscaling.PutScalingPolicyInput) applicationautoscaling.PutScalingPolicyRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough putScalingPolicyResponse responses provided for test"
		nextResponse = putScalingPolicyResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextputScalingPolicyResponse, ok := nextResponse.(putScalingPolicyResponse)
	if !ok {
		message := "putScalingPolicy request created, next response is not of type putScalingPolicyOutput"
		nextputScalingPolicyResponse = putScalingPolicyResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextputScalingPolicyResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextputScalingPolicyResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextputScalingPolicyResponse.policyData
		})
	}

	return applicationautoscaling.PutScalingPolicyRequest{
		Request: mockRequest,
	}
}

// Mock DeregisterScalableTargetRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DeregisterScalableTarget, or there are no more responses to give, fail the test.
func (m mockApplicationAutoscalingClient) DeregisterScalableTargetRequest(input *applicationautoscaling.DeregisterScalableTargetInput) applicationautoscaling.DeregisterScalableTargetRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DeregisterScalableTarget responses provided for test"
		nextResponse = deregisterScalableTargetResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextderegisterScalableTargetResponse, ok := nextResponse.(deregisterScalableTargetResponse)
	if !ok {
		message := "DeregisterScalableTargetRequest request created, next response is not of type DeregisterScalableTargetOutput"
		nextderegisterScalableTargetResponse = deregisterScalableTargetResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextderegisterScalableTargetResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextderegisterScalableTargetResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextderegisterScalableTargetResponse.targetData
		})
	}

	return applicationautoscaling.DeregisterScalableTargetRequest{
		Request: mockRequest,
	}
}

// Mock DeleteScalingPolicyRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DeleteScalingPolicy, or there are no more responses to give, fail the test.
func (m mockApplicationAutoscalingClient) DeleteScalingPolicyRequest(input *applicationautoscaling.DeleteScalingPolicyInput) applicationautoscaling.DeleteScalingPolicyRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough deleteScalingPolicyResponse responses provided for test"
		nextResponse = deleteScalingPolicyResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextdeleteScalingPolicyResponse, ok := nextResponse.(deleteScalingPolicyResponse)
	if !ok {
		message := "deleteScalingPolicy request created, next response is not of type deleteScalingPolicy"
		nextdeleteScalingPolicyResponse = deleteScalingPolicyResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextdeleteScalingPolicyResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextdeleteScalingPolicyResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextdeleteScalingPolicyResponse.policyData
		})
	}

	return applicationautoscaling.DeleteScalingPolicyRequest{
		Request: mockRequest,
	}
}
