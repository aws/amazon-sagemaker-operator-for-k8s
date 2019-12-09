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

package batchtransformjob

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	. "container/list"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	batchtransformjobv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/batchtransformjob"
	commonv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/common"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/controllertest"

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
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)

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

var _ = Describe("Reconciling a job that has no finalizer", func() {

	var (
		job     *batchtransformjobv1.BatchTransformJob
		builder *MockSageMakerClientBuilder
		err     error
	)

	BeforeEach(func() {

		job = createTransformJobWithGeneratedName(false)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTransformJobStatus(job, InitializingJobStatus)

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
		sageMakerClient := builder.Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
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
		job     *batchtransformjobv1.BatchTransformJob
		builder *MockSageMakerClientBuilder
		err     error
	)

	BeforeEach(func() {
		job = createTransformJobWithGeneratedName(false)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTransformJobStatus(job, "")
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
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		controller.Reconcile(request)

		// Verify status is updated.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(job.Status.TransformJobStatus).To(Equal(InitializingJobStatus))
	})

	It("should requeue immediately", func() {
		sageMakerClient := builder.Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		reconciliationResult, err := controller.Reconcile(request)

		// Verify expectations
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(true))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Reconciling a transform job with no TransformJobName", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *batchtransformjobv1.BatchTransformJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeTransformJob request.
		err error
	)

	BeforeEach(func() {
		job = createTransformJobWithNoSageMakerName(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTransformJobStatus(job, InitializingJobStatus)

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
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
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
		controller := createTransformJobReconcilerForSageMakerClient(mockK8sClient, sageMakerClient, 1)
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

var _ = Describe("Reconciling a job with finalizer that is being deleted", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *batchtransformjobv1.BatchTransformJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeTransformJob request.
		description sagemaker.DescribeTransformJobOutput
		err         error
	)

	BeforeEach(func() {

		job = createTransformJobWithGeneratedName(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTransformJobStatus(job, InitializingJobStatus)

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		// Create SageMaker mock description.
		description = createDescriptionFromSmTransformJob(job)
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

	It("should stop the SageMaker job and requeue if the job is in progress", func() {
		description.TransformJobStatus = sagemaker.TransformJobStatusInProgress
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeTransformJobResponse(description).
			AddStopTransformJobResponse(sagemaker.StopTransformJobOutput{}).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(true))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
		// SageMaker will have two request, describe and delete
		Expect(receivedRequests.Len()).To(Equal(2))
	})

	It("should update the status and requeue if the job is stopping", func() {
		description.TransformJobStatus = sagemaker.TransformJobStatusStopping
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeTransformJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(controller.PollInterval))

		// Verify status is updated.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)

		Expect(job.Status.TransformJobStatus).To(ContainSubstring(string(sagemaker.TransformJobStatusStopping)))
	})

	It("should update the status and retry if SageMaker throttles", func() {
		rateExceededMessage := "Rate exceeded"
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeTransformJobErrorResponse("ThrottlingException", 400, "request id", rateExceededMessage).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(controller.PollInterval))

		// Verify status is updated.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)

		Expect(job.Status.Additional).To(ContainSubstring(rateExceededMessage))
	})

	It("should remove the finalizer and not requeue if the job is stopped", func() {
		description.TransformJobStatus = sagemaker.TransformJobStatusStopped
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeTransformJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))

		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("should remove the finalizer and not requeue if the job is failed", func() {

		description.TransformJobStatus = sagemaker.TransformJobStatusFailed
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeTransformJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))

		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("should remove the finalizer and not requeue if the job is completed", func() {
		description.TransformJobStatus = sagemaker.TransformJobStatusCompleted
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeTransformJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))

		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

})

var _ = Describe("Reconciling a job without finalizer that is being deleted", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *batchtransformjobv1.BatchTransformJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeTransformJob request.
		description sagemaker.DescribeTransformJobOutput
		err         error
	)

	BeforeEach(func() {

		job = createTransformJobWithGeneratedName(false)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setTransformJobStatus(job, InitializingJobStatus)

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		// Create SageMaker mock description.
		description = createDescriptionFromSmTransformJob(job)
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

	It("should not requeue if the job has no finalizer", func() {
		description.TransformJobStatus = sagemaker.TransformJobStatusInProgress
		// Setup mock responses.
		sageMakerClient := builder.
			Build()

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		reconciliationResult, err := controller.Reconcile(request)

		// Run test and verify expectations.
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
		// SageMaker will have no request
		Expect(receivedRequests.Len()).To(Equal(0))
	})
})

var _ = Describe("Reconciling when a custom SageMaker endpoint is requested", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *batchtransformjobv1.BatchTransformJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeTransformJob request.
		description sagemaker.DescribeTransformJobOutput

		expectedEndpoint string

		mockEnv venv.Env

		err error
	)

	BeforeEach(func() {
		job = createTransformJobWithGeneratedName(true)

		expectedEndpoint = "https://" + uuid.New().String() + ".com"
		mockEnv = venv.Mock()
		mockEnv.Setenv(DefaultSageMakerEndpointEnvKey, expectedEndpoint)

		// Create SageMaker mock description.
		description = createDescriptionFromSmTransformJob(job)

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

		setTransformJobStatus(job, InitializingJobStatus)

		description.TransformJobStatus = sagemaker.TransformJobStatusInProgress

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTransformJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
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

		setTransformJobStatus(job, InitializingJobStatus)

		description.TransformJobStatus = sagemaker.TransformJobStatusInProgress

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTransformJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
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

		setTransformJobStatus(job, InitializingJobStatus)

		description.TransformJobStatus = sagemaker.TransformJobStatusInProgress

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeTransformJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createTransformJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
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

// Helper function to create a reconciler.
func createTransformJobReconcilerForSageMakerClient(k8sClient client.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalSeconds int64) BatchTransformJobReconciler {
	provider := func(_ aws.Config) sagemakeriface.ClientAPI {
		return sageMakerClient
	}

	return createTransformJobReconciler(k8sClient, provider, CreateMockAwsConfigLoader(), pollIntervalSeconds)
}

// Helper function to create a reconciler.
func createTransformJobReconciler(k8sClient client.Client, sageMakerClientProvider SageMakerClientProvider, awsConfigLoader AwsConfigLoader, pollIntervalSeconds int64) BatchTransformJobReconciler {
	return BatchTransformJobReconciler{
		Client:                k8sClient,
		Log:                   ctrl.Log,
		PollInterval:          time.Duration(pollIntervalSeconds * 1e9),
		createSageMakerClient: sageMakerClientProvider,
		awsConfigLoader:       awsConfigLoader,
	}
}

// Use a randomly generated job name so that tests can be executed in parallel.
func generateTransformJobK8sName() string {
	jobId := rand.Int()
	return "transform-job-" + strconv.Itoa(jobId)
}

func createTransformJobWithNoSageMakerName(hasFinalizer bool) *batchtransformjobv1.BatchTransformJob {
	return createTransformJob(generateTransformJobK8sName(), "", hasFinalizer)
}

func createTransformJobWithGeneratedName(hasFinalizer bool) *batchtransformjobv1.BatchTransformJob {
	k8sName := generateTransformJobK8sName()
	sageMakerName := "sagemaker-" + k8sName
	return createTransformJob(k8sName, sageMakerName, hasFinalizer)
}

// Helper function to create a TransformJob with mock data.
func createTransformJob(k8sName, sageMakerName string, hasFinalizer bool) *batchtransformjobv1.BatchTransformJob {

	finalizers := []string{}
	if hasFinalizer {
		finalizers = append(finalizers, SageMakerResourceFinalizerName)
	}

	return &batchtransformjobv1.BatchTransformJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:       k8sName,
			Namespace:  "default",
			Finalizers: finalizers,
		},
		Spec: batchtransformjobv1.BatchTransformJobSpec{
			TransformJobName: &sageMakerName,
			ModelName:        ToStringPtr("model-abc"),
			TransformInput: &commonv1.TransformInput{
				ContentType: ToStringPtr("text/csv"),
				DataSource: &commonv1.TransformDataSource{
					S3DataSource: &commonv1.TransformS3DataSource{
						S3DataType: "S3Prefix",
						S3Uri:      ToStringPtr("s3://inputpath"),
					},
				},
			},
			TransformOutput: &commonv1.TransformOutput{
				S3OutputPath: ToStringPtr("s3://outputpath"),
			},
			TransformResources: &commonv1.TransformResources{
				InstanceCount: ToInt64Ptr(1),
				InstanceType:  "ml.m4.xlarge",
			},
			Region: ToStringPtr("region-xyz"),
		},
		Status: batchtransformjobv1.BatchTransformJobStatus{
			TransformJobStatus: InitializingJobStatus,
		},
	}
}

// Helper function to create a sagemaker.DescribeTransformJobOutput from a TransformJob.
func createDescriptionFromSmTransformJob(job *batchtransformjobv1.BatchTransformJob) sagemaker.DescribeTransformJobOutput {
	return sagemaker.DescribeTransformJobOutput{
		TransformJobName: job.Spec.TransformJobName,
		ModelName:        job.Spec.ModelName,
		TransformInput: &sagemaker.TransformInput{
			ContentType: job.Spec.TransformInput.ContentType,
			DataSource: &sagemaker.TransformDataSource{
				S3DataSource: &sagemaker.TransformS3DataSource{
					S3DataType: sagemaker.S3DataType(job.Spec.TransformInput.DataSource.S3DataSource.S3DataType),
					S3Uri:      job.Spec.TransformInput.DataSource.S3DataSource.S3Uri,
				},
			},
		},
		TransformOutput: &sagemaker.TransformOutput{
			S3OutputPath: job.Spec.TransformOutput.S3OutputPath,
		},
		TransformResources: &sagemaker.TransformResources{
			InstanceCount: job.Spec.TransformResources.InstanceCount,
			InstanceType:  sagemaker.TransformInstanceType(job.Spec.TransformResources.InstanceType),
		},
	}
}

func setTransformJobStatus(job *batchtransformjobv1.BatchTransformJob, status string) {
	job.Status = batchtransformjobv1.BatchTransformJobStatus{
		TransformJobStatus: status,
	}
	err := k8sClient.Status().Update(context.Background(), job)
	Expect(err).ToNot(HaveOccurred())
}
