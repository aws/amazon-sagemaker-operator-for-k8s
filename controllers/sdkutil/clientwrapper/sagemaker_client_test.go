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

package clientwrapper_test

import (
	"context"

	. "container/list"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("SageMakerClientWrapper.DescribeEndpoint", func() {

	var (
		sageMakerClientBuilder *MockSageMakerClientBuilder
		receivedRequests       List
	)

	BeforeEach(func() {
		receivedRequests = List{}

		sageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
	})

	It("should return the response output if no error occurs", func() {

		mockResponse := sagemaker.DescribeEndpointOutput{
			EndpointConfigName: ToStringPtr("endpoint-config-name"),
		}

		sageMakerClient := sageMakerClientBuilder.
			AddDescribeEndpointResponse(mockResponse).
			Build()
		client := NewSageMakerClientWrapper(sageMakerClient)

		endpointName := "test-endpoint-name"
		response, err := client.DescribeEndpoint(context.Background(), endpointName)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		Expect(*response).To(Equal(mockResponse))
	})

	It("should return nil response, nil error if a 404 error occurs", func() {
		sageMakerClient := sageMakerClientBuilder.
			AddDescribeEndpointErrorResponse("ValidationException", "Could not find endpoint xyz", 400, "request id").
			Build()
		client := NewSageMakerClientWrapper(sageMakerClient)

		endpointName := "test-endpoint-name"
		response, err := client.DescribeEndpoint(context.Background(), endpointName)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(BeNil())
	})

	It("should return an error if a non-404 error occurs", func() {
		sageMakerClient := sageMakerClientBuilder.
			AddDescribeEndpointErrorResponse("SomeCode", "SomeMessage", 400, "request id").
			Build()
		client := NewSageMakerClientWrapper(sageMakerClient)

		endpointName := "test-endpoint-name"
		response, err := client.DescribeEndpoint(context.Background(), endpointName)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).To(HaveOccurred())
		Expect(response).To(BeNil())
	})

})

var _ = Describe("SageMakerClientWrapper.DescribeModel", func() {

	var (
		sageMakerClientBuilder *MockSageMakerClientBuilder
		receivedRequests       List
	)

	BeforeEach(func() {
		receivedRequests = List{}

		sageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
	})

	It("should return the response output if no error occurs", func() {

		mockResponse := sagemaker.DescribeModelOutput{
			ModelName: ToStringPtr("model-name"),
		}

		sageMakerClient := sageMakerClientBuilder.
			AddDescribeModelResponse(mockResponse).
			Build()
		client := NewSageMakerClientWrapper(sageMakerClient)

		modelName := "test-model-name"
		response, err := client.DescribeModel(context.Background(), modelName)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		Expect(*response).To(Equal(mockResponse))
	})

	It("should return nil response, nil error if a 404 error occurs", func() {
		sageMakerClient := sageMakerClientBuilder.
			AddDescribeModelErrorResponse("ValidationException", "Could not find model xyz", 400, "request id").
			Build()
		client := NewSageMakerClientWrapper(sageMakerClient)

		modelName := "test-model-name"
		response, err := client.DescribeModel(context.Background(), modelName)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).To(BeNil())
	})

	It("should return an error if a non-404 error occurs", func() {
		sageMakerClient := sageMakerClientBuilder.
			AddDescribeModelErrorResponse("SomeCode", "SomeMessage", 400, "request id").
			Build()
		client := NewSageMakerClientWrapper(sageMakerClient)

		modelName := "test-model-name"
		response, err := client.DescribeModel(context.Background(), modelName)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).To(HaveOccurred())
		Expect(response).To(BeNil())
	})

})

var _ = Describe("SageMakerClientWrapper.CreateModel", func() {

	var (
		sageMakerClientBuilder *MockSageMakerClientBuilder
		receivedRequests       List
	)

	BeforeEach(func() {
		receivedRequests = List{}

		sageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
	})

	It("should return the response output if no error occurs", func() {

		mockResponse := sagemaker.CreateModelOutput{
			ModelArn: ToStringPtr("xxxyyy"),
		}

		sageMakerClient := sageMakerClientBuilder.
			AddCreateModelResponse(mockResponse).
			Build()
		client := NewSageMakerClientWrapper(sageMakerClient)

		modelToCreate := &sagemaker.CreateModelInput{
			ModelName: ToStringPtr("test-model-name"),
		}
		response, err := client.CreateModel(context.Background(), modelToCreate)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		Expect(*response).To(Equal(mockResponse))
	})

	It("should return an error if an error occurs", func() {
		sageMakerClient := sageMakerClientBuilder.
			AddCreateModelErrorResponse("SomeCode", "SomeMessage", 400, "request id").
			Build()
		client := NewSageMakerClientWrapper(sageMakerClient)

		modelToCreate := &sagemaker.CreateModelInput{
			ModelName: ToStringPtr("test-model-name"),
		}
		response, err := client.CreateModel(context.Background(), modelToCreate)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).To(HaveOccurred())
		Expect(response).To(BeNil())
	})

})
