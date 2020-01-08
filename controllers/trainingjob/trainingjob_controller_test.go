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

package trainingjob

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	. "container/list"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	trainingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/trainingjob"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"

	"github.com/adammck/venv"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	"github.com/google/uuid"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("Reconciling a non-existent job", func() {

	It("should not requeue", func() {

		sageMakerClient := NewMockSageMakerClientBuilder(GinkgoT()).Build()
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)

		request := ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "namespace",
				Name:      "non-existent-name",
			},
		}

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Deleting a job with a finalizer", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *trainingjobv1.TrainingJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeTrainingJob request.
		description sagemaker.DescribeTrainingJobOutput
		err         error
	)

	BeforeEach(func() {

		job = createTrainingJobWithGeneratedName(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTrainingJobStatus(job, InitializingJobStatus)

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT())

		// Create SageMaker mock description.
		description = createDescriptionFromSmTrainingJob(job)
	})

	AfterEach(func() {
		// Get job so we can delete it.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		if err != nil && apierrs.IsNotFound(err) {
			return
		}
		Expect(err).ToNot(HaveOccurred())

		// Assert that deletionTimestamp is nonzero.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Get the job again since we made update
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)

		// Remove finalizer so that the job can be deleted.
		job.Finalizers = []string{}
		err = k8sClient.Update(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert deleted.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("should delete an InProgress job and requeue", func() {
		description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

		// Setup mock responses.
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTrainingJobResponse(description).
			AddStopTrainingJobResponse(sagemaker.StopTrainingJobOutput{}).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(2))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(true))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should requeue a Stopping job", func() {
		description.TrainingJobStatus = sagemaker.TrainingJobStatusStopping

		// Setup mock responses.
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTrainingJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(controller.PollInterval))

		// TODO: Add Verify job status
	})

	It("should not requeue a Stopped job", func() {
		description.TrainingJobStatus = sagemaker.TrainingJobStatusStopped

		// Setup mock responses.
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTrainingJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should not requeue a Completed job", func() {
		description.TrainingJobStatus = sagemaker.TrainingJobStatusCompleted

		// Setup mock responses.
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTrainingJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should not requeue a Failed job", func() {
		description.TrainingJobStatus = sagemaker.TrainingJobStatusFailed

		// Setup mock responses.
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTrainingJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Reconciling a job that has no finalizer", func() {

	var (
		job     *trainingjobv1.TrainingJob
		builder *MockSageMakerClientBuilder
		err     error
	)

	BeforeEach(func() {

		job = createTrainingJobWithGeneratedName(false)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTrainingJobStatus(job, InitializingJobStatus)

		// Create SageMaker mock API client.
		builder = NewMockSageMakerClientBuilder(GinkgoT())
	})

	AfterEach(func() {
		// Get job so we can delete it.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		if err != nil && apierrs.IsNotFound(err) {
			return
		}
		Expect(err).ToNot(HaveOccurred())

		// Assert that deletionTimestamp is nonzero.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)

		job.Finalizers = []string{}
		err = k8sClient.Update(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert deleted.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("should add a finalizer and requeue immediately", func() {
		Skip("This test will not work until TrainingController is refactored to add finalizer before SageMaker call")
		sageMakerClient := builder.Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		reconciliationResult, err := controller.Reconcile(request)

		// Verify requeue immediately
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(true))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))

		// Verify a finalizer has been added
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(job.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))
	})
})

var _ = Describe("Reconciling a job with an empty status", func() {
	var (
		// The Kubernetes job that the controller will reconcile.
		job     *trainingjobv1.TrainingJob
		builder *MockSageMakerClientBuilder
		err     error
	)

	BeforeEach(func() {
		job = createTrainingJobWithGeneratedName(false)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTrainingJobStatus(job, "")
		builder = NewMockSageMakerClientBuilder(GinkgoT())
	})

	AfterEach(func() {
		// Get job so we can delete it.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		if err != nil && apierrs.IsNotFound(err) {
			return
		}
		Expect(err).ToNot(HaveOccurred())

		// Assert that deletionTimestamp is nonzero.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert deleted.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("should update the status to an initialization status", func() {
		sageMakerClient := builder.Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		controller.Reconcile(request)

		// Verify status is updated.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(job.Status.TrainingJobStatus).To(Equal(InitializingJobStatus))
	})

	It("should requeue immediately", func() {
		sageMakerClient := builder.Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		reconciliationResult, err := controller.Reconcile(request)

		// Verify expectations
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(true))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Reconciling a training job with no TrainingJobName", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *trainingjobv1.TrainingJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeTrainingJob request.
		err error
	)

	BeforeEach(func() {
		job = createTrainingJobWithNoSageMakerName(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTrainingJobStatus(job, InitializingJobStatus)

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT())
	})

	AfterEach(func() {
		// Get job so we can delete it.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		if err != nil && apierrs.IsNotFound(err) {
			return
		}
		Expect(err).ToNot(HaveOccurred())

		// Remove finalizer so that the job can be deleted.
		job.Finalizers = []string{}
		err = k8sClient.Update(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert that deletionTimestamp is nonzero.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert deleted.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("should generate a job name, update the spec, and not requeue", func() {
		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		reconciliationResult, err := controller.Reconcile(request)

		// Verify expectations
		Expect(receivedRequests.Len()).To(Equal(0))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should requeue if spec update fails", func() {
		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			Build()

		mockK8sClient := FailToUpdateK8sClient{
			ActualClient: k8sClient,
		}

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconcilerForSageMakerClient(mockK8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		reconciliationResult, err := controller.Reconcile(request)

		// Verify expectations
		Expect(receivedRequests.Len()).To(Equal(0))
		Expect(err).To(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Reconciling when a custom SageMaker endpoint is requested", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *trainingjobv1.TrainingJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeTrainingJob request.
		description sagemaker.DescribeTrainingJobOutput

		expectedEndpoint string

		mockEnv venv.Env

		err error
	)

	BeforeEach(func() {
		job = createTrainingJobWithGeneratedName(true)

		expectedEndpoint = "https://" + uuid.New().String() + ".com"
		mockEnv = venv.Mock()
		mockEnv.Setenv(DefaultSageMakerEndpointEnvKey, expectedEndpoint)

		// Create SageMaker mock description.
		description = createDescriptionFromSmTrainingJob(job)

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT())
	})

	AfterEach(func() {
		// Get job so we can delete it.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		if err != nil && apierrs.IsNotFound(err) {
			return
		}
		Expect(err).ToNot(HaveOccurred())

		// Remove finalizer so that the job can be deleted.
		job.Finalizers = []string{}
		err = k8sClient.Update(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert that deletionTimestamp is nonzero.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert deleted.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())

		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("should configure the SageMaker client to use the custom endpoint", func() {

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTrainingJobStatus(job, InitializingJobStatus)

		description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTrainingJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		_, err := controller.Reconcile(request)

		// Verify expectations
		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())

		Expect(actualEndpointResolver).ToNot(BeNil())
		actualEndpoint, err := (*actualEndpointResolver).ResolveEndpoint(sagemaker.EndpointsID, uuid.New().String())
		Expect(err).ToNot(HaveOccurred())
		Expect(actualEndpoint.URL).To(Equal(expectedEndpoint))
	})

	It("should use the job-specific SageMakerEndpoint over the environment variable", func() {

		expectedEndpoint = "https://" + uuid.New().String() + ".expected.com"
		job.Spec.SageMakerEndpoint = &expectedEndpoint

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTrainingJobStatus(job, InitializingJobStatus)

		description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTrainingJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		_, err := controller.Reconcile(request)

		// Verify expectations
		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())

		Expect(actualEndpointResolver).ToNot(BeNil())
		actualEndpoint, err := (*actualEndpointResolver).ResolveEndpoint(sagemaker.EndpointsID, uuid.New().String())
		Expect(err).ToNot(HaveOccurred())
		Expect(actualEndpoint.URL).To(Equal(expectedEndpoint))
	})

	It("should configure the SageMaker client to use the job-specific endpoint if provided", func() {

		// Set env variable to empty
		mockEnv.Setenv(DefaultSageMakerEndpointEnvKey, "")

		expectedEndpoint = "https://" + uuid.New().String() + ".expected.com"
		job.Spec.SageMakerEndpoint = &expectedEndpoint

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTrainingJobStatus(job, InitializingJobStatus)

		description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTrainingJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createTrainingJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		_, err := controller.Reconcile(request)

		// Verify expectations
		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())

		Expect(actualEndpointResolver).ToNot(BeNil())
		actualEndpoint, err := (*actualEndpointResolver).ResolveEndpoint(sagemaker.EndpointsID, uuid.New().String())
		Expect(err).ToNot(HaveOccurred())
		Expect(actualEndpoint.URL).To(Equal(expectedEndpoint))
	})
})

var _ = Describe("Reconciling an existing job", func() {

	var (
		job              *trainingjobv1.TrainingJob
		receivedRequests List
		builder          *MockSageMakerClientBuilder
		description      sagemaker.DescribeTrainingJobOutput
		err              error
	)

	BeforeEach(func() {

		job = createTrainingJobWithGeneratedName(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTrainingJobStatus(job, InitializingJobStatus)

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT())

		// Create SageMaker mock description.
		description = createDescriptionFromSmTrainingJob(job)
	})

	AfterEach(func() {
		// Get job so we can delete it.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		if err != nil && apierrs.IsNotFound(err) {
			return
		}
		Expect(err).ToNot(HaveOccurred())

		// Remove finalizer so that the job can be deleted.
		job.Finalizers = []string{}
		err = k8sClient.Update(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert that deletionTimestamp is nonzero.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Assert deleted.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	Context("when the spec is updated", func() {

		BeforeEach(func() {

			// Get job so we can modify it.
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: job.ObjectMeta.Namespace,
				Name:      job.ObjectMeta.Name,
			}, job)
			Expect(err).ToNot(HaveOccurred())

			job.Spec.ResourceConfig.VolumeSizeInGB = ToInt64Ptr(*job.Spec.ResourceConfig.VolumeSizeInGB + 5)
			err = k8sClient.Update(context.Background(), job)
			Expect(err).ToNot(HaveOccurred())

		})

		It("should set status to failed", func() {
			// Not failed
			description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

			// Setup mock responses.
			sageMakerClient := builder.
				WithRequestList(&receivedRequests).
				AddDescribeTrainingJobResponse(description).
				Build()

			// Instantiate controller and reconciliation request.
			controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

			// Run test and verify expectations.
			controller.Reconcile(request)

			// Verify status is failed.
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: job.ObjectMeta.Namespace,
				Name:      job.ObjectMeta.Name,
			}, job)
			Expect(job.Status.TrainingJobStatus).To(Equal(string(sagemaker.TrainingJobStatusFailed)))
		})

		It("should set additional to contain 'the resource no longer matches'", func() {
			// Setup mock responses.
			sageMakerClient := builder.
				WithRequestList(&receivedRequests).
				AddDescribeTrainingJobResponse(description).
				Build()

			// Instantiate controller and reconciliation request.
			controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

			// Run test and verify expectations.
			controller.Reconcile(request)

			// Verify status is failed.
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: job.ObjectMeta.Namespace,
				Name:      job.ObjectMeta.Name,
			}, job)
			Expect(job.Status.Additional).To(ContainSubstring("the resource no longer matches"))
		})

		It("should not requeue", func() {
			// Setup mock responses.
			sageMakerClient := builder.
				WithRequestList(&receivedRequests).
				AddDescribeTrainingJobResponse(description).
				Build()

			// Instantiate controller and reconciliation request.
			controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

			// Run test and verify expectations.
			reconciliationResult, err := controller.Reconcile(request)

			Expect(receivedRequests.Len()).To(Equal(1))
			Expect(err).ToNot(HaveOccurred())
			Expect(reconciliationResult.Requeue).To(Equal(false))
			Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
		})
	})
})

// Helper function to create a reconciler.
func createTrainingJobReconcilerForSageMakerClient(k8sClient client.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalSeconds int64) TrainingJobReconciler {
	provider := func(_ aws.Config) sagemakeriface.ClientAPI {
		return sageMakerClient
	}

	return createTrainingJobReconciler(k8sClient, provider, CreateMockAwsConfigLoader(), pollIntervalSeconds)
}

// Helper function to create a reconciler.
func createTrainingJobReconciler(k8sClient client.Client, sageMakerClientProvider SageMakerClientProvider, awsConfigLoader AwsConfigLoader, pollIntervalSeconds int64) TrainingJobReconciler {
	return TrainingJobReconciler{
		Client:                k8sClient,
		Log:                   ctrl.Log,
		PollInterval:          time.Duration(pollIntervalSeconds * 1e9),
		createSageMakerClient: sageMakerClientProvider,
		awsConfigLoader:       awsConfigLoader,
	}
}

// Use a randomly generated job name so that tests can be executed in parallel.
func generateTrainingJobK8sName() string {
	jobId := rand.Int()
	return "training-job-" + strconv.Itoa(jobId)
}

func createTrainingJobWithNoSageMakerName(hasFinalizer bool) *trainingjobv1.TrainingJob {
	return createTrainingJob(generateTrainingJobK8sName(), "", hasFinalizer)
}

func createTrainingJobWithGeneratedName(hasFinalizer bool) *trainingjobv1.TrainingJob {
	k8sName := generateTrainingJobK8sName()
	sageMakerName := "sagemaker-" + k8sName
	return createTrainingJob(k8sName, sageMakerName, hasFinalizer)
}

// Helper function to create a TrainingJob with mock data.
func createTrainingJob(k8sName, sageMakerName string, hasFinalizer bool) *trainingjobv1.TrainingJob {

	finalizers := []string{}
	if hasFinalizer {
		finalizers = append(finalizers, SageMakerResourceFinalizerName)
	}

	return &trainingjobv1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:       k8sName,
			Namespace:  "default",
			Finalizers: finalizers,
		},
		Spec: trainingjobv1.TrainingJobSpec{
			TrainingJobName: ToStringPtr(sageMakerName),
			AlgorithmSpecification: &commonv1.AlgorithmSpecification{
				TrainingInputMode: "File",
			},
			OutputDataConfig: &commonv1.OutputDataConfig{
				S3OutputPath: ToStringPtr("s3://outputpath"),
			},
			ResourceConfig: &commonv1.ResourceConfig{
				InstanceCount:  ToInt64Ptr(1),
				InstanceType:   "xyz",
				VolumeSizeInGB: ToInt64Ptr(50),
			},
			RoleArn:           ToStringPtr("xxxxxxxxxxxxxxxxxxxx"),
			Region:            ToStringPtr("region-xyz"),
			StoppingCondition: &commonv1.StoppingCondition{},
		},
		Status: trainingjobv1.TrainingJobStatus{
			TrainingJobStatus: InitializingJobStatus,
		},
	}
}

// Helper function to create a sagemaker.DescribeTrainingJobOutput from an TrainingJob.
func createDescriptionFromSmTrainingJob(job *trainingjobv1.TrainingJob) sagemaker.DescribeTrainingJobOutput {
	return sagemaker.DescribeTrainingJobOutput{
		TrainingJobName: job.Spec.TrainingJobName,
		AlgorithmSpecification: &sagemaker.AlgorithmSpecification{
			TrainingInputMode: sagemaker.TrainingInputMode(job.Spec.AlgorithmSpecification.TrainingInputMode),
		},
		OutputDataConfig: &sagemaker.OutputDataConfig{
			S3OutputPath: job.Spec.OutputDataConfig.S3OutputPath,
		},
		ResourceConfig: &sagemaker.ResourceConfig{
			InstanceCount:  job.Spec.ResourceConfig.InstanceCount,
			InstanceType:   sagemaker.TrainingInstanceType(job.Spec.ResourceConfig.InstanceType),
			VolumeSizeInGB: job.Spec.ResourceConfig.VolumeSizeInGB,
		},
		RoleArn:           job.Spec.RoleArn,
		StoppingCondition: &sagemaker.StoppingCondition{},
	}
}

func setTrainingJobStatus(job *trainingjobv1.TrainingJob, status string) {
	job.Status = trainingjobv1.TrainingJobStatus{
		TrainingJobStatus: status,
	}
	err := k8sClient.Status().Update(context.Background(), job)
	Expect(err).ToNot(HaveOccurred())
}
