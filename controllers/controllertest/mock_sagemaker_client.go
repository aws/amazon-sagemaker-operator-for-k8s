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
	"fmt"
	"net/http"

	. "container/list"
	. "github.com/onsi/ginkgo"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
)

func NewMockSageMakerClientBuilder(testReporter GinkgoTInterface) *MockSageMakerClientBuilder {
	builder := MockSageMakerClientBuilder{
		testReporter: testReporter,
	}
	return &builder
}

// Builder for mock SageMaker API clients.
type MockSageMakerClientBuilder struct {
	// Used to fail tests when not enough responses are provided for requests.
	testReporter GinkgoTInterface
	// Used to store responses that SageMaker will respond with. The mock client responds
	// with responses in the order that they were added, i.e. with AddDescribeTrainingJobResponse.
	responses List
	// Used to store requests received by SageMaker client.
	requests *List
}

// Helper data structure that represents a single DescribeTrainingJob response.
type describeTrainingJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DescribeTrainingJobOutput
}

// Helper data structure that represents a single CreateTrainingJob response.
type createTrainingJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.CreateTrainingJobOutput
}

// Helper data structure that represents a single ListTrainingJobsForHyperParameterTuningJob response.
type listTrainingJobsForHyperParameterTuningJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput
}

// Helper data structure that represents a single StopTrainingJob response.
type stopTrainingJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.StopTrainingJobOutput
}

// Helper data structure that represents a single DescribeHyperParameterTuningJob response.
type describeHyperParameterTuningJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DescribeHyperParameterTuningJobOutput
}

// Helper data structure that represents a single CreateHyperParameterTuningJob response.
type createHyperParameterTuningJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.CreateHyperParameterTuningJobOutput
}

// Helper data structure that represents a single StopHyperParameterTuning response.
type stopHyperParameterTuningJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.StopHyperParameterTuningJobOutput
}

// Helper data structure that represents a single DescribeEndpoint response.
type describeEndpointResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DescribeEndpointOutput
}

// Helper data structure that represents a single DescribeModel response.
type describeModelResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DescribeModelOutput
}

// Helper data structure that represents a single CreateModel response.
type createModelResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.CreateModelOutput
}

// Helper data structure that represents a single DeleteModel response.
type deleteModelResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DeleteModelOutput
}

// Helper data structure that represents a single DescribeTransformJob response.
type describeTransformJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DescribeTransformJobOutput
}

// Helper data structure that represents a single StopTransform response.
type stopTransformJobResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.StopTransformJobOutput
}

// Helper data structure that represents a single DescribeEndpointConfig response.
type describeEndpointConfigResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DescribeEndpointConfigOutput
}

// Helper data structure that represents a single CreateEndpointConfig response.
type createEndpointConfigResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.CreateEndpointConfigOutput
}

// Helper data structure that represents a single UpdateEndpoint response.
type updateEndpointResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.UpdateEndpointOutput
}

// Helper data structure that represents a single DeleteEndpoint response.
type deleteEndpointResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DeleteEndpointOutput
}

// Helper data structure that represents a single DeleteEndpointConfigresponse.
type deleteEndpointConfigResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.DeleteEndpointConfigOutput
}

// Helper data structure that represents a single CreateEndpoint response.
type createEndpointResponse struct {
	err  awserr.RequestFailure
	data *sagemaker.CreateEndpointOutput
}

// Add a DescribeTrainingJob error response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeTrainingJobErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeTrainingJobResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DescribeTrainingJob response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeTrainingJobResponse(data sagemaker.DescribeTrainingJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeTrainingJobResponse{
		err:  nil,
		data: &data,
	})
	return m
}

// Add a CreateTrainingJob response to the client.
func (m *MockSageMakerClientBuilder) AddCreateTrainingJobResponse(data sagemaker.CreateTrainingJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(createTrainingJobResponse{
		err:  nil,
		data: &data,
	})
	return m
}

// Add a ListTrainingJobsForHyperParameterTuningJob error response to the client.
func (m *MockSageMakerClientBuilder) AddListTrainingJobsForHyperParameterTuningJobErrorResponse(code string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(listTrainingJobsForHyperParameterTuningJobResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, "mock error message", fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a ListTrainingJobsForHyperParameterTuningJob response to the client.
func (m *MockSageMakerClientBuilder) AddListTrainingJobsForHyperParameterTuningJobResponse(data sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(listTrainingJobsForHyperParameterTuningJobResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a StopTrainingJob error response to the client.
func (m *MockSageMakerClientBuilder) AddStopTrainingJobErrorResponse(code string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(stopTrainingJobResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, "mock error message", fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a StopTrainingJob response to the client.
func (m *MockSageMakerClientBuilder) AddStopTrainingJobResponse(data sagemaker.StopTrainingJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(stopTrainingJobResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a DescribeHyperParameterTuningJob error response to the client which has messsage too.
func (m *MockSageMakerClientBuilder) AddDescribeHyperParameterTuningJobErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeHyperParameterTuningJobResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DescribeHyperParameterTuningJob response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeHyperParameterTuningJobResponse(data sagemaker.DescribeHyperParameterTuningJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeHyperParameterTuningJobResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a CreateHyperParameterTuningJob error response to the client.
func (m *MockSageMakerClientBuilder) AddCreateHyperParameterTuningJobErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(createHyperParameterTuningJobResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a CreateHyperParameterTuningJob response to the client.
func (m *MockSageMakerClientBuilder) AddCreateHyperParameterTuningJobResponse(data sagemaker.CreateHyperParameterTuningJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(createHyperParameterTuningJobResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a StopHyperParameterTuningJob response to the client.
func (m *MockSageMakerClientBuilder) AddStopHyperParameterTuningJobResponse(data sagemaker.StopHyperParameterTuningJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(stopHyperParameterTuningJobResponse{
		err:  nil,
		data: &data,
	})
	return m
}

// Add a DescribeEndpoint error response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeEndpointErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeEndpointResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DescribeTrainingJob error response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeTransformJobErrorResponse(code string, statusCode int, reqId, message string) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeTransformJobResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DescribeEndpoint response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeEndpointResponse(data sagemaker.DescribeEndpointOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeEndpointResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a DescribeModel error response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeModelErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeModelResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DescribeModel response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeModelResponse(data sagemaker.DescribeModelOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeModelResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a CreateModel error response to the client.
func (m *MockSageMakerClientBuilder) AddCreateModelErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(createModelResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a CreateModel response to the client.
func (m *MockSageMakerClientBuilder) AddCreateModelResponse(data sagemaker.CreateModelOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(createModelResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a DeleteModel error response to the client.
func (m *MockSageMakerClientBuilder) AddDeleteModelErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(deleteModelResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DeleteModel response to the client.
func (m *MockSageMakerClientBuilder) AddDeleteModelResponse(data sagemaker.DeleteModelOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(deleteModelResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a DescribeTransformJob response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeTransformJobResponse(data sagemaker.DescribeTransformJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeTransformJobResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a DescribeEndpointConfig error response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeEndpointConfigErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeEndpointConfigResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DescribeEndpointConfig response to the client.
func (m *MockSageMakerClientBuilder) AddDescribeEndpointConfigResponse(data sagemaker.DescribeEndpointConfigOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(describeEndpointConfigResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a DeleteEndpointConfig error response to the client.
func (m *MockSageMakerClientBuilder) AddDeleteEndpointConfigErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(deleteEndpointConfigResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DeleteEndpointConfig response to the client.
func (m *MockSageMakerClientBuilder) AddDeleteEndpointConfigResponse(data sagemaker.DeleteEndpointConfigOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(deleteEndpointConfigResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a DeleteEndpoint error response to the client.
func (m *MockSageMakerClientBuilder) AddDeleteEndpointErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(deleteEndpointResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a DeleteEndpoint response to the client.
func (m *MockSageMakerClientBuilder) AddDeleteEndpointResponse(data sagemaker.DeleteEndpointOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(deleteEndpointResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a CreateEndpointConfig error response to the client.
func (m *MockSageMakerClientBuilder) AddCreateEndpointConfigErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(createEndpointConfigResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a CreateEndpointConfig response to the client.
func (m *MockSageMakerClientBuilder) AddCreateEndpointConfigResponse(data sagemaker.CreateEndpointConfigOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(createEndpointConfigResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a UpdateEndpoint error response to the client.
func (m *MockSageMakerClientBuilder) AddUpdateEndpointErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(updateEndpointResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a UpdateEndpoint response to the client.
func (m *MockSageMakerClientBuilder) AddUpdateEndpointResponse(data sagemaker.UpdateEndpointOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(updateEndpointResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Add a CreateEndpoint error response to the client.
func (m *MockSageMakerClientBuilder) AddCreateEndpointErrorResponse(code string, message string, statusCode int, reqId string) *MockSageMakerClientBuilder {
	m.responses.PushBack(createEndpointResponse{
		err:  awserr.NewRequestFailure(awserr.New(code, message, fmt.Errorf(code)), statusCode, reqId),
		data: nil,
	})
	return m
}

// Add a CreateEndpoint response to the client.
func (m *MockSageMakerClientBuilder) AddCreateEndpointResponse(data sagemaker.CreateEndpointOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(createEndpointResponse{
		err:  nil,
		data: &data,
	})

	return m
}

// Store requests received by the mock client in a user-provided list.
func (m *MockSageMakerClientBuilder) WithRequestList(requests *List) *MockSageMakerClientBuilder {
	m.requests = requests
	return m
}

// Get how many responses were added to the mock SageMaker client.
func (m *MockSageMakerClientBuilder) GetAddedResponsesLen() int {
	return m.responses.Len()
}

// Create a mock SageMaker API client given configuration.
func (m *MockSageMakerClientBuilder) Build() sagemakeriface.ClientAPI {

	if m.testReporter == nil {
		panic("MockSageMakerClientBuilder requires non-nil test reporter.")
	}

	if m.requests == nil {
		m.requests = &List{}
	}

	sageMakerClient := mockSageMakerClient{
		responses:    &m.responses,
		requests:     m.requests,
		testReporter: m.testReporter,
	}

	return sageMakerClient
}

// Mock SageMaker API client.
type mockSageMakerClient struct {
	sagemakeriface.ClientAPI

	// List of responses to use when responding to API calls. They are returned in same order
	// as they are stored in the list.
	responses *List

	// List of requests that are received. They are stored in the same order that they are received.
	requests *List

	// Test reporter used to fail tests if not enough responses were provided.
	testReporter GinkgoTInterface
}

func (m *mockSageMakerClient) mockRequestBuilder() *aws.Request {
	return &aws.Request{
		HTTPRequest: &http.Request{
			Header: map[string][]string{},
		},
		HTTPResponse: &http.Response{},
		// Required for pagination operation.
		Operation: &aws.Operation{
			Paginator: nil,
		},
	}
}

// Mock CreateTrainingJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type CreateTrainingJob, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) CreateTrainingJobRequest(input *sagemaker.CreateTrainingJobInput) sagemaker.CreateTrainingJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough CreateTrainingJob responses provided for test"
		nextResponse = createTrainingJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextCreateTrainingJobResponse, ok := nextResponse.(createTrainingJobResponse)
	if !ok {
		message := "CreateTrainingJob request created, next response is not of type CreateTrainingJobOutput"
		nextCreateTrainingJobResponse = createTrainingJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextCreateTrainingJobResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextCreateTrainingJobResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextCreateTrainingJobResponse.data
		})
	}

	return sagemaker.CreateTrainingJobRequest{
		Request: mockRequest,
	}
}

// Mock DescribeTrainingJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DescribeTrainingJob, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DescribeTrainingJobRequest(input *sagemaker.DescribeTrainingJobInput) sagemaker.DescribeTrainingJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DescribeTrainingJob responses provided for test"
		nextResponse = describeTrainingJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDescribeTrainingJobResponse, ok := nextResponse.(describeTrainingJobResponse)
	if !ok {
		message := "DescribeTrainingJob request created, next response is not of type DescribeTrainingJobOutput"
		nextDescribeTrainingJobResponse = describeTrainingJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDescribeTrainingJobResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDescribeTrainingJobResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDescribeTrainingJobResponse.data
		})
	}

	return sagemaker.DescribeTrainingJobRequest{
		Request: mockRequest,
	}
}

// Mock ListTrainingJobsForHyperParameterTuningJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type listTrainingJobsForHyperParameterTuningJobResponse, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) ListTrainingJobsForHyperParameterTuningJobRequest(input *sagemaker.ListTrainingJobsForHyperParameterTuningJobInput) sagemaker.ListTrainingJobsForHyperParameterTuningJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough listTrainingJobsForHyperParameterTuningJobResponse responses provided for test"
		nextResponse = listTrainingJobsForHyperParameterTuningJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextListTrainingJobsForHyperParameterTuningJobResponseResponse, ok := nextResponse.(listTrainingJobsForHyperParameterTuningJobResponse)
	if !ok {
		message := "listTrainingJobsForHyperParameterTuningJobResponse request created, next response is not of type ListTrainingJobsForHyperParameterTuningJobResponseOutput"
		nextListTrainingJobsForHyperParameterTuningJobResponseResponse = listTrainingJobsForHyperParameterTuningJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextListTrainingJobsForHyperParameterTuningJobResponseResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextListTrainingJobsForHyperParameterTuningJobResponseResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextListTrainingJobsForHyperParameterTuningJobResponseResponse.data
		})
	}

	// Required for pagination operation. I do not recommend that you test actual pagination
	// in unit tests, as I imagine the Copy field will have to be filled out for every time you call
	// paginator.Next.
	copyFn := func(input *sagemaker.ListTrainingJobsForHyperParameterTuningJobInput) sagemaker.ListTrainingJobsForHyperParameterTuningJobRequest {
		return sagemaker.ListTrainingJobsForHyperParameterTuningJobRequest{
			Request: mockRequest,
			Input:   input,
			Copy:    nil,
		}
	}

	return sagemaker.ListTrainingJobsForHyperParameterTuningJobRequest{
		Request: mockRequest,
		Copy:    copyFn,
	}
}

// Mock StopTrainingJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type StopTrainingJob, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) StopTrainingJobRequest(input *sagemaker.StopTrainingJobInput) sagemaker.StopTrainingJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough StopTrainingJob responses provided for test"
		nextResponse = stopTrainingJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextStopTrainingJobResponse, ok := nextResponse.(stopTrainingJobResponse)
	if !ok {
		message := "StopTrainingJob request created, next response is not of type StopTrainingJobOutput"
		nextStopTrainingJobResponse = stopTrainingJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextStopTrainingJobResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextStopTrainingJobResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextStopTrainingJobResponse.data
		})
	}

	return sagemaker.StopTrainingJobRequest{
		Request: mockRequest,
	}
}

// Mock DescribeHyperParameterTuningJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DescribeHyperParameterTuningJob, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DescribeHyperParameterTuningJobRequest(input *sagemaker.DescribeHyperParameterTuningJobInput) sagemaker.DescribeHyperParameterTuningJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DescribeHyperParameterTuningJob responses provided for test"
		nextResponse = describeHyperParameterTuningJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDescribeHyperParameterTuningJobResponse, ok := nextResponse.(describeHyperParameterTuningJobResponse)
	if !ok {
		message := "DescribeHyperParameterTuningJob request created, next response is not of type DescribeHyperParameterTuningJobOutput"
		nextDescribeHyperParameterTuningJobResponse = describeHyperParameterTuningJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDescribeHyperParameterTuningJobResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDescribeHyperParameterTuningJobResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDescribeHyperParameterTuningJobResponse.data
		})
	}

	return sagemaker.DescribeHyperParameterTuningJobRequest{
		Request: mockRequest,
	}
}

// Mock CreateHyperParameterTuningJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type CreateHyperParameterTuningJob, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) CreateHyperParameterTuningJobRequest(input *sagemaker.CreateHyperParameterTuningJobInput) sagemaker.CreateHyperParameterTuningJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough CreateHyperParameterTuningJob responses provided for test"
		nextResponse = createHyperParameterTuningJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextCreateHyperParameterTuningJobResponse, ok := nextResponse.(createHyperParameterTuningJobResponse)
	if !ok {
		message := "CreateHyperParameterTuningJob request created, next response is not of type CreateHyperParameterTuningJobOutput"
		nextCreateHyperParameterTuningJobResponse = createHyperParameterTuningJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextCreateHyperParameterTuningJobResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextCreateHyperParameterTuningJobResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextCreateHyperParameterTuningJobResponse.data
		})
	}

	return sagemaker.CreateHyperParameterTuningJobRequest{
		Request: mockRequest,
	}
}

// Mock StopHyperParameterTuningJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type StopHyperParameterTuningJob, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) StopHyperParameterTuningJobRequest(input *sagemaker.StopHyperParameterTuningJobInput) sagemaker.StopHyperParameterTuningJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough StopHyperParameterTuningJob responses provided for test"
		nextResponse = stopHyperParameterTuningJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextStopHyperParameterTuningJobResponse, ok := nextResponse.(stopHyperParameterTuningJobResponse)
	if !ok {
		message := "StopHyperParameterTuningJob request stopd, next response is not of type StopHyperParameterTuningJobOutput"
		nextStopHyperParameterTuningJobResponse = stopHyperParameterTuningJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextStopHyperParameterTuningJobResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextStopHyperParameterTuningJobResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextStopHyperParameterTuningJobResponse.data
		})
	}

	return sagemaker.StopHyperParameterTuningJobRequest{
		Request: mockRequest,
	}
}

// Mock DescribeEndpointRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DescribeEndpoint, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DescribeEndpointRequest(input *sagemaker.DescribeEndpointInput) sagemaker.DescribeEndpointRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DescribeEndpoint responses provided for test"
		nextResponse = describeEndpointResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDescribeEndpointResponse, ok := nextResponse.(describeEndpointResponse)
	if !ok {
		message := "DescribeEndpoint request created, next response is not of type DescribeEndpointOutput"
		nextDescribeEndpointResponse = describeEndpointResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDescribeEndpointResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDescribeEndpointResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDescribeEndpointResponse.data
		})
	}

	return sagemaker.DescribeEndpointRequest{
		Request: mockRequest,
	}
}

// Mock DeleteModelRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DeleteModel, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DeleteModelRequest(input *sagemaker.DeleteModelInput) sagemaker.DeleteModelRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DeleteModelRequest responses provided for test"
		nextResponse = deleteModelResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDeleteModelResponse, ok := nextResponse.(deleteModelResponse)
	if !ok {
		message := "DeleteModel request created, next response is not of type DeleteModelOutput"
		nextDeleteModelResponse = deleteModelResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDeleteModelResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDeleteModelResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDeleteModelResponse.data
		})
	}

	return sagemaker.DeleteModelRequest{
		Request: mockRequest,
	}
}

// Mock DescribeModelRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DescribeModel, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DescribeModelRequest(input *sagemaker.DescribeModelInput) sagemaker.DescribeModelRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DescribeModel responses provided for test"
		nextResponse = describeModelResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDescribeModelResponse, ok := nextResponse.(describeModelResponse)
	if !ok {
		message := "DescribeModel request created, next response is not of type DescribeModelOutput"
		nextDescribeModelResponse = describeModelResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDescribeModelResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDescribeModelResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDescribeModelResponse.data
		})
	}

	return sagemaker.DescribeModelRequest{
		Request: mockRequest,
	}
}

// Mock CreateModelRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type CreateModel, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) CreateModelRequest(input *sagemaker.CreateModelInput) sagemaker.CreateModelRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough CreateModel responses provided for test"
		nextResponse = createModelResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextCreateModelResponse, ok := nextResponse.(createModelResponse)
	if !ok {
		message := "CreateModel request created, next response is not of type CreateModelOutput"
		nextCreateModelResponse = createModelResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextCreateModelResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextCreateModelResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextCreateModelResponse.data
		})
	}

	return sagemaker.CreateModelRequest{
		Request: mockRequest,
	}
}

// Mock DescribeTransformJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DescribeTransformJob, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DescribeTransformJobRequest(input *sagemaker.DescribeTransformJobInput) sagemaker.DescribeTransformJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DescribeTransformJob responses provided for test"
		nextResponse = describeTransformJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDescribeTransformJobResponse, ok := nextResponse.(describeTransformJobResponse)
	if !ok {
		message := "DescribeTransformJob request created, next response is not of type DescribeTransformJobOutput"
		nextDescribeTransformJobResponse = describeTransformJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDescribeTransformJobResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDescribeTransformJobResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDescribeTransformJobResponse.data
		})
	}

	return sagemaker.DescribeTransformJobRequest{
		Request: mockRequest,
	}
}

// Add a AddStopTransformJobResponse response to the client.
func (m *MockSageMakerClientBuilder) AddStopTransformJobResponse(data sagemaker.StopTransformJobOutput) *MockSageMakerClientBuilder {
	m.responses.PushBack(stopTransformJobResponse{
		err:  nil,
		data: &data,
	})
	return m
}

// Mock StopTransformJobRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type StopTransformJob, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) StopTransformJobRequest(input *sagemaker.StopTransformJobInput) sagemaker.StopTransformJobRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough StopTransformJob responses provided for test"
		nextResponse = stopTransformJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextStopTransformJobResponse, ok := nextResponse.(stopTransformJobResponse)
	if !ok {
		message := "StopTransformJob request stopd, next response is not of type StopTransformJobOutput"
		nextStopTransformJobResponse = stopTransformJobResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextStopTransformJobResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextStopTransformJobResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextStopTransformJobResponse.data
		})
	}

	return sagemaker.StopTransformJobRequest{
		Request: mockRequest,
	}
}

// Mock DescribeEndpointConfigRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DescribeEndpointConfig, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DescribeEndpointConfigRequest(input *sagemaker.DescribeEndpointConfigInput) sagemaker.DescribeEndpointConfigRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DescribeEndpointConfig responses provided for test"
		nextResponse = describeEndpointConfigResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDescribeEndpointConfigResponse, ok := nextResponse.(describeEndpointConfigResponse)
	if !ok {
		message := "DescribeEndpointConfig request created, next response is not of type DescribeEndpointConfigOutput"
		nextDescribeEndpointConfigResponse = describeEndpointConfigResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDescribeEndpointConfigResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDescribeEndpointConfigResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDescribeEndpointConfigResponse.data
		})
	}

	return sagemaker.DescribeEndpointConfigRequest{
		Request: mockRequest,
	}
}

// Mock CreateEndpointConfigRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type CreateEndpointConfig, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) CreateEndpointConfigRequest(input *sagemaker.CreateEndpointConfigInput) sagemaker.CreateEndpointConfigRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough CreateEndpointConfig responses provided for test"
		nextResponse = createEndpointConfigResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextCreateEndpointConfigResponse, ok := nextResponse.(createEndpointConfigResponse)
	if !ok {
		message := "CreateEndpointConfig request created, next response is not of type CreateEndpointConfigOutput"
		nextCreateEndpointConfigResponse = createEndpointConfigResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextCreateEndpointConfigResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextCreateEndpointConfigResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextCreateEndpointConfigResponse.data
		})
	}

	return sagemaker.CreateEndpointConfigRequest{
		Request: mockRequest,
	}
}

// Mock UpdateEndpointRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type UpdateEndpoint, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) UpdateEndpointRequest(input *sagemaker.UpdateEndpointInput) sagemaker.UpdateEndpointRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough UpdateEndpoint responses provided for test"
		nextResponse = updateEndpointResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextUpdateEndpointResponse, ok := nextResponse.(updateEndpointResponse)
	if !ok {
		message := "UpdateEndpoint request created, next response is not of type UpdateEndpointOutput"
		nextUpdateEndpointResponse = updateEndpointResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextUpdateEndpointResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextUpdateEndpointResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextUpdateEndpointResponse.data
		})
	}

	return sagemaker.UpdateEndpointRequest{
		Request: mockRequest,
	}
}

// Mock DeleteEndpointConfigRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DeleteEndpointConfig, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DeleteEndpointConfigRequest(input *sagemaker.DeleteEndpointConfigInput) sagemaker.DeleteEndpointConfigRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DeleteEndpointConfigRequest responses provided for test"
		nextResponse = deleteEndpointConfigResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDeleteEndpointConfigResponse, ok := nextResponse.(deleteEndpointConfigResponse)
	if !ok {
		message := "DeleteEndpointConfig request created, next response is not of type DeleteEndpointConfigOutput"
		nextDeleteEndpointConfigResponse = deleteEndpointConfigResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDeleteEndpointConfigResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDeleteEndpointConfigResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDeleteEndpointConfigResponse.data
		})
	}

	return sagemaker.DeleteEndpointConfigRequest{
		Request: mockRequest,
	}
}

// Mock CreateEndpointRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type CreateEndpoint, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) CreateEndpointRequest(input *sagemaker.CreateEndpointInput) sagemaker.CreateEndpointRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough CreateEndpoint responses provided for test"
		nextResponse = createEndpointResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextCreateEndpointResponse, ok := nextResponse.(createEndpointResponse)
	if !ok {
		message := "CreateEndpoint request created, next response is not of type CreateEndpointOutput"
		nextCreateEndpointResponse = createEndpointResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextCreateEndpointResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextCreateEndpointResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextCreateEndpointResponse.data
		})
	}

	return sagemaker.CreateEndpointRequest{
		Request: mockRequest,
	}
}

// Mock DeleteEndpointRequest implementation. It overrides a request response with the mock data.
// If the next response is not of type DeleteEndpoint, or there are no more responses to give, fail the test.
func (m mockSageMakerClient) DeleteEndpointRequest(input *sagemaker.DeleteEndpointInput) sagemaker.DeleteEndpointRequest {

	m.requests.PushBack(input)

	front := m.responses.Front()

	var nextResponse interface{}
	if front == nil {
		message := "Not enough DeleteEndpointRequest responses provided for test"
		nextResponse = deleteEndpointResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
		m.testReporter.Error(message)
	} else {
		nextResponse = front.Value
		m.responses.Remove(front)
	}

	nextDeleteEndpointResponse, ok := nextResponse.(deleteEndpointResponse)
	if !ok {
		message := "DeleteEndpoint request created, next response is not of type DeleteEndpointOutput"
		nextDeleteEndpointResponse = deleteEndpointResponse{
			err: awserr.NewRequestFailure(awserr.New("test error", message, fmt.Errorf(message)), 500, "request id"),
		}
	}

	mockRequest := m.mockRequestBuilder()

	if nextDeleteEndpointResponse.err != nil {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Error = nextDeleteEndpointResponse.err
		})
	} else {
		mockRequest.Handlers.Send.PushBack(func(r *aws.Request) {
			r.Data = nextDeleteEndpointResponse.data
		})
	}

	return sagemaker.DeleteEndpointRequest{
		Request: mockRequest,
	}
}
