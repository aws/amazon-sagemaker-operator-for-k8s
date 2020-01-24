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
	"time"

	. "container/list"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	trainingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/trainingjob"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	// "github.com/adammck/venv"
	// "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	// "github.com/google/uuid"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	// "sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("Reconciling a TrainingJob while failing to get the Kubernetes job", func() {

	var (
		sageMakerClient sagemakeriface.ClientAPI
	)

	BeforeEach(func() {
		sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).Build()
	})

	It("should not requeue if the TrainingJob does not exist", func() {
		controller := createReconcilerWithMockedDependencies(k8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should requeue if there was an error", func() {
		mockK8sClient := FailToGetK8sClient{}
		controller := createReconcilerWithMockedDependencies(mockK8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Reconciling a HostingDeployment that exists", func() {

	var (
		// The requests received by the mock SageMaker client.
		receivedRequests List

		// SageMaker client builder used to create mock responses.
		mockSageMakerClientBuilder *MockSageMakerClientBuilder

		// The total number of requests added to the mock SageMaker client builder.
		expectedRequestCount int

		// The mock training job.
		trainingJob *trainingjobv1.TrainingJob

		// The kubernetes client to use in the test. This is different than the default
		// test client as some tests use a special test client.
		kubernetesClient k8sclient.Client

		// The poll duration that the controller is configured with.
		pollDuration string

		// A generated name to be used in the TrainingJob.Status SageMaker name.
		// trainingJobSageMakerName string

		// Whether or not the test deployment should have deletion timestamp set.
		shouldHaveDeletionTimestamp bool

		// Whether or not the test deployment should have a finalizer.
		shouldHaveFinalizer bool

		// The controller result.
		reconcileResult ctrl.Result

		// The controller error result.
		reconcileError error
	)

	BeforeEach(func() {
		pollDuration = "1s"

		// trainingJobSageMakerName = "training-job-" + uuid.New().String()

		shouldHaveDeletionTimestamp = false
		shouldHaveFinalizer = false

		kubernetesClient = k8sClient

		receivedRequests = List{}
		mockSageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		trainingJob = createTrainingJobWithGeneratedNames()
	})

	JustBeforeEach(func() {
		sageMakerClient := mockSageMakerClientBuilder.Build()
		expectedRequestCount = mockSageMakerClientBuilder.GetAddedResponsesLen()

		controller := createReconciler(kubernetesClient, sageMakerClient, pollDuration)

		err := k8sClient.Create(context.Background(), trainingJob)
		Expect(err).ToNot(HaveOccurred())

		if shouldHaveFinalizer {
			AddFinalizer(trainingJob)
		}

		if shouldHaveDeletionTimestamp {
			SetDeletionTimestamp(trainingJob)
		}

		request := CreateReconciliationRequest(trainingJob.ObjectMeta.GetName(), trainingJob.ObjectMeta.GetNamespace())
		reconcileResult, reconcileError = controller.Reconcile(request)
	})

	AfterEach(func() {
		Expect(receivedRequests.Len()).To(Equal(expectedRequestCount), "Expect that all SageMaker responses were consumed")
	})

	Context("DescribeTrainingJob fails", func() {

		var failureMessage string

		BeforeEach(func() {
			failureMessage = "error message " + uuid.New().String()
			mockSageMakerClientBuilder.AddDescribeTrainingJobErrorResponse("Exception", failureMessage, 500, "request id")
		})

		It("Requeues immediately", func() {
			ExpectRequeueImmediately(reconcileResult, reconcileError)
		})

		It("Updates status", func() {
			ExpectAdditionalToContain(trainingJob, failureMessage)
			ExpectStatusToBe(trainingJob, ReconcilingTrainingJobStatus, "")
		})
	})

	Context("TrainingJob does not exist", func() {

		BeforeEach(func() {
			mockSageMakerClientBuilder.
				AddDescribeTrainingJobErrorResponse(clientwrapper.DescribeTrainingJob404Code, clientwrapper.DescribeTrainingJob404MessagePrefix, 400, "request id")
		})

		Context("HasDeletionTimestamp", func() {

			BeforeEach(func() {
				shouldHaveDeletionTimestamp = true
				shouldHaveFinalizer = true
			})

			It("Removes finalizer", func() {
				ExpectTrainingJobToBeDeleted(trainingJob)
			})

			It("Requeues after interval", func() {
				ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
			})
		})

		Context("!HasDeletionTimestamp", func() {
			BeforeEach(func() {
				mockSageMakerClientBuilder.
					AddCreateTrainingJobResponse(sagemaker.CreateTrainingJobOutput{}).
					AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(sagemaker.TrainingJobStatusInProgress, sagemaker.SecondaryStatusStarting))

				shouldHaveDeletionTimestamp = false
				shouldHaveFinalizer = true
			})

			It("Creates a TrainingJob", func() {

				req := receivedRequests.Front().Next().Value
				Expect(req).To(BeAssignableToTypeOf((*sagemaker.CreateTrainingJobInput)(nil)))

				createdRequest := req.(*sagemaker.CreateTrainingJobInput)
				Expect(*createdRequest.TrainingJobName).To(Equal(controllers.GetGeneratedJobName(trainingJob.ObjectMeta.GetUID(), trainingJob.ObjectMeta.GetName(), 63)))
			})

			It("Requeues after interval", func() {
				ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
			})

			It("Updates status", func() {
				ExpectStatusToBe(trainingJob, string(sagemaker.TrainingJobStatusInProgress), string(sagemaker.SecondaryStatusStarting))
			})
		})
	})

})

// var _ = Describe("Reconciling a non-existent job", func() {

// 	It("should not requeue", func() {

// 		sageMakerClient := NewMockSageMakerClientBuilder(GinkgoT()).Build()
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)

// 		request := ctrl.Request{
// 			NamespacedName: types.NamespacedName{
// 				Namespace: "namespace",
// 				Name:      "non-existent-name",
// 			},
// 		}

// 		result, err := controller.Reconcile(request)

// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(result.Requeue).To(Equal(false))
// 		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
// 	})
// })

// var _ = Describe("Deleting a job with a finalizer", func() {

// 	var (
// 		// The Kubernetes job that the controller will reconcile.
// 		job *trainingjobv1.TrainingJob
// 		// The list of requests that the mock SageMaker API has recieved.
// 		receivedRequests List
// 		// A builder for mock SageMaker API clients.
// 		builder *MockSageMakerClientBuilder
// 		// The SageMaker response for a DescribeTrainingJob request.
// 		description sagemaker.DescribeTrainingJobOutput
// 		err         error
// 	)

// 	BeforeEach(func() {

// 		job = createTrainingJobWithGeneratedName(true)

// 		// Create job in Kubernetes.
// 		err = k8sClient.Create(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		setTrainingJobStatus(job, InitializingJobStatus)

// 		// Mark job as deleting in Kubernetes.
// 		err = k8sClient.Delete(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Create SageMaker mock API client.
// 		receivedRequests = List{}
// 		builder = NewMockSageMakerClientBuilder(GinkgoT())

// 		// Create SageMaker mock description.
// 		description = createDescriptionFromSmTrainingJob(job)
// 	})

// 	AfterEach(func() {
// 		// Get job so we can delete it.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		if err != nil && apierrs.IsNotFound(err) {
// 			return
// 		}
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert that deletionTimestamp is nonzero.
// 		err = k8sClient.Delete(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Get the job again since we made update
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)

// 		// Remove finalizer so that the job can be deleted.
// 		job.Finalizers = []string{}
// 		err = k8sClient.Update(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert deleted.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		Expect(err).To(HaveOccurred())
// 		Expect(apierrs.IsNotFound(err)).To(Equal(true))
// 	})

// 	It("should delete an InProgress job and requeue", func() {
// 		description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

// 		// Setup mock responses.
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			AddDescribeTrainingJobResponse(description).
// 			AddStopTrainingJobResponse(sagemaker.StopTrainingJobOutput{}).
// 			Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test and verify expectations.
// 		reconciliationResult, err := controller.Reconcile(request)

// 		Expect(receivedRequests.Len()).To(Equal(2))
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(true))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
// 	})

// 	It("should requeue a Stopping job", func() {
// 		description.TrainingJobStatus = sagemaker.TrainingJobStatusStopping

// 		// Setup mock responses.
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			AddDescribeTrainingJobResponse(description).
// 			Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test and verify expectations.
// 		reconciliationResult, err := controller.Reconcile(request)

// 		Expect(receivedRequests.Len()).To(Equal(1))
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(false))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(controller.PollInterval))

// 		// TODO: Add Verify job status
// 	})

// 	It("should not requeue a Stopped job", func() {
// 		description.TrainingJobStatus = sagemaker.TrainingJobStatusStopped

// 		// Setup mock responses.
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			AddDescribeTrainingJobResponse(description).
// 			Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test and verify expectations.
// 		reconciliationResult, err := controller.Reconcile(request)

// 		Expect(receivedRequests.Len()).To(Equal(1))
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(false))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
// 	})

// 	It("should not requeue a Completed job", func() {
// 		description.TrainingJobStatus = sagemaker.TrainingJobStatusCompleted

// 		// Setup mock responses.
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			AddDescribeTrainingJobResponse(description).
// 			Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test and verify expectations.
// 		reconciliationResult, err := controller.Reconcile(request)

// 		Expect(receivedRequests.Len()).To(Equal(1))
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(false))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
// 	})

// 	It("should not requeue a Failed job", func() {
// 		description.TrainingJobStatus = sagemaker.TrainingJobStatusFailed

// 		// Setup mock responses.
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			AddDescribeTrainingJobResponse(description).
// 			Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test and verify expectations.
// 		reconciliationResult, err := controller.Reconcile(request)

// 		Expect(receivedRequests.Len()).To(Equal(1))
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(false))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
// 	})
// })

// var _ = Describe("Reconciling a job that has no finalizer", func() {

// 	var (
// 		job     *trainingjobv1.TrainingJob
// 		builder *MockSageMakerClientBuilder
// 		err     error
// 	)

// 	BeforeEach(func() {

// 		job = createTrainingJobWithGeneratedName(false)

// 		// Create job in Kubernetes.
// 		err = k8sClient.Create(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		setTrainingJobStatus(job, InitializingJobStatus)

// 		// Create SageMaker mock API client.
// 		builder = NewMockSageMakerClientBuilder(GinkgoT())
// 	})

// 	AfterEach(func() {
// 		// Get job so we can delete it.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		if err != nil && apierrs.IsNotFound(err) {
// 			return
// 		}
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert that deletionTimestamp is nonzero.
// 		err = k8sClient.Delete(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)

// 		job.Finalizers = []string{}
// 		err = k8sClient.Update(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert deleted.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		Expect(err).To(HaveOccurred())
// 		Expect(apierrs.IsNotFound(err)).To(Equal(true))
// 	})

// 	It("should add a finalizer and requeue immediately", func() {
// 		Skip("This test will not work until TrainingController is refactored to add finalizer before SageMaker call")
// 		sageMakerClient := builder.Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		reconciliationResult, err := controller.Reconcile(request)

// 		// Verify requeue immediately
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(true))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))

// 		// Verify a finalizer has been added
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		Expect(job.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))
// 	})
// })

// var _ = Describe("Reconciling a job with an empty status", func() {
// 	var (
// 		// The Kubernetes job that the controller will reconcile.
// 		job     *trainingjobv1.TrainingJob
// 		builder *MockSageMakerClientBuilder
// 		err     error
// 	)

// 	BeforeEach(func() {
// 		job = createTrainingJobWithGeneratedName(false)

// 		// Create job in Kubernetes.
// 		err = k8sClient.Create(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		setTrainingJobStatus(job, "")
// 		builder = NewMockSageMakerClientBuilder(GinkgoT())
// 	})

// 	AfterEach(func() {
// 		// Get job so we can delete it.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		if err != nil && apierrs.IsNotFound(err) {
// 			return
// 		}
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert that deletionTimestamp is nonzero.
// 		err = k8sClient.Delete(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert deleted.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		Expect(err).To(HaveOccurred())
// 		Expect(apierrs.IsNotFound(err)).To(Equal(true))
// 	})

// 	It("should update the status to an initialization status", func() {
// 		sageMakerClient := builder.Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test
// 		controller.Reconcile(request)

// 		// Verify status is updated.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		Expect(job.Status.TrainingJobStatus).To(Equal(InitializingJobStatus))
// 	})

// 	It("should requeue immediately", func() {
// 		sageMakerClient := builder.Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test
// 		reconciliationResult, err := controller.Reconcile(request)

// 		// Verify expectations
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(true))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
// 	})
// })

// var _ = Describe("Reconciling a training job with no TrainingJobName", func() {

// 	var (
// 		// The Kubernetes job that the controller will reconcile.
// 		job *trainingjobv1.TrainingJob
// 		// The list of requests that the mock SageMaker API has recieved.
// 		receivedRequests List
// 		// A builder for mock SageMaker API clients.
// 		builder *MockSageMakerClientBuilder
// 		// The SageMaker response for a DescribeTrainingJob request.
// 		err error
// 	)

// 	BeforeEach(func() {
// 		job = createTrainingJobWithNoSageMakerName(true)

// 		// Create job in Kubernetes.
// 		err = k8sClient.Create(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		setTrainingJobStatus(job, InitializingJobStatus)

// 		// Create SageMaker mock API client.
// 		receivedRequests = List{}
// 		builder = NewMockSageMakerClientBuilder(GinkgoT())
// 	})

// 	AfterEach(func() {
// 		// Get job so we can delete it.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		if err != nil && apierrs.IsNotFound(err) {
// 			return
// 		}
// 		Expect(err).ToNot(HaveOccurred())

// 		// Remove finalizer so that the job can be deleted.
// 		job.Finalizers = []string{}
// 		err = k8sClient.Update(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert that deletionTimestamp is nonzero.
// 		err = k8sClient.Delete(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert deleted.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		Expect(err).To(HaveOccurred())
// 		Expect(apierrs.IsNotFound(err)).To(Equal(true))
// 	})

// 	It("should generate a job name, update the spec, and not requeue", func() {
// 		// Instantiate dependencies
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			Build()

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test
// 		reconciliationResult, err := controller.Reconcile(request)

// 		// Verify expectations
// 		Expect(receivedRequests.Len()).To(Equal(0))
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(false))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
// 	})

// 	It("should requeue if spec update fails", func() {
// 		// Instantiate dependencies
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			Build()

// 		mockK8sClient := FailToUpdateK8sClient{
// 			ActualClient: k8sClient,
// 		}

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconcilerForSageMakerClient(mockK8sClient, sageMakerClient, 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test
// 		reconciliationResult, err := controller.Reconcile(request)

// 		// Verify expectations
// 		Expect(receivedRequests.Len()).To(Equal(0))
// 		Expect(err).To(HaveOccurred())
// 		Expect(reconciliationResult.Requeue).To(Equal(false))
// 		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
// 	})
// })

// var _ = Describe("Reconciling when a custom SageMaker endpoint is requested", func() {

// 	var (
// 		// The Kubernetes job that the controller will reconcile.
// 		job *trainingjobv1.TrainingJob
// 		// The list of requests that the mock SageMaker API has recieved.
// 		receivedRequests List
// 		// A builder for mock SageMaker API clients.
// 		builder *MockSageMakerClientBuilder
// 		// The SageMaker response for a DescribeTrainingJob request.
// 		description sagemaker.DescribeTrainingJobOutput

// 		expectedEndpoint string

// 		mockEnv venv.Env

// 		err error
// 	)

// 	BeforeEach(func() {
// 		job = createTrainingJobWithGeneratedName(true)

// 		expectedEndpoint = "https://" + uuid.New().String() + ".com"
// 		mockEnv = venv.Mock()
// 		mockEnv.Setenv(DefaultSageMakerEndpointEnvKey, expectedEndpoint)

// 		// Create SageMaker mock description.
// 		description = createDescriptionFromSmTrainingJob(job)

// 		// Create SageMaker mock API client.
// 		receivedRequests = List{}
// 		builder = NewMockSageMakerClientBuilder(GinkgoT())
// 	})

// 	AfterEach(func() {
// 		// Get job so we can delete it.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		if err != nil && apierrs.IsNotFound(err) {
// 			return
// 		}
// 		Expect(err).ToNot(HaveOccurred())

// 		// Remove finalizer so that the job can be deleted.
// 		job.Finalizers = []string{}
// 		err = k8sClient.Update(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert that deletionTimestamp is nonzero.
// 		err = k8sClient.Delete(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert deleted.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		Expect(err).To(HaveOccurred())

// 		Expect(apierrs.IsNotFound(err)).To(Equal(true))
// 	})

// 	It("should configure the SageMaker client to use the custom endpoint", func() {

// 		// Create job in Kubernetes.
// 		err = k8sClient.Create(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		setTrainingJobStatus(job, InitializingJobStatus)

// 		description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

// 		// Instantiate dependencies
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			AddDescribeTrainingJobResponse(description).
// 			Build()

// 		var actualEndpointResolver *aws.EndpointResolver = nil

// 		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
// 			actualEndpointResolver = &awsConfig.EndpointResolver
// 			return sageMakerClient
// 		}

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test
// 		_, err := controller.Reconcile(request)

// 		// Verify expectations
// 		Expect(receivedRequests.Len()).To(Equal(1))
// 		Expect(err).ToNot(HaveOccurred())

// 		Expect(actualEndpointResolver).ToNot(BeNil())
// 		actualEndpoint, err := (*actualEndpointResolver).ResolveEndpoint(sagemaker.EndpointsID, uuid.New().String())
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(actualEndpoint.URL).To(Equal(expectedEndpoint))
// 	})

// 	It("should use the job-specific SageMakerEndpoint over the environment variable", func() {

// 		expectedEndpoint = "https://" + uuid.New().String() + ".expected.com"
// 		job.Spec.SageMakerEndpoint = &expectedEndpoint

// 		// Create job in Kubernetes.
// 		err = k8sClient.Create(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		setTrainingJobStatus(job, InitializingJobStatus)

// 		description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

// 		// Instantiate dependencies
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			AddDescribeTrainingJobResponse(description).
// 			Build()

// 		var actualEndpointResolver *aws.EndpointResolver = nil

// 		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
// 			actualEndpointResolver = &awsConfig.EndpointResolver
// 			return sageMakerClient
// 		}

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test
// 		_, err := controller.Reconcile(request)

// 		// Verify expectations
// 		Expect(receivedRequests.Len()).To(Equal(1))
// 		Expect(err).ToNot(HaveOccurred())

// 		Expect(actualEndpointResolver).ToNot(BeNil())
// 		actualEndpoint, err := (*actualEndpointResolver).ResolveEndpoint(sagemaker.EndpointsID, uuid.New().String())
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(actualEndpoint.URL).To(Equal(expectedEndpoint))
// 	})

// 	It("should configure the SageMaker client to use the job-specific endpoint if provided", func() {

// 		// Set env variable to empty
// 		mockEnv.Setenv(DefaultSageMakerEndpointEnvKey, "")

// 		expectedEndpoint = "https://" + uuid.New().String() + ".expected.com"
// 		job.Spec.SageMakerEndpoint = &expectedEndpoint

// 		// Create job in Kubernetes.
// 		err = k8sClient.Create(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		setTrainingJobStatus(job, InitializingJobStatus)

// 		description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

// 		// Instantiate dependencies
// 		sageMakerClient := builder.
// 			WithRequestList(&receivedRequests).
// 			AddDescribeTrainingJobResponse(description).
// 			Build()

// 		var actualEndpointResolver *aws.EndpointResolver = nil

// 		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
// 			actualEndpointResolver = &awsConfig.EndpointResolver
// 			return sageMakerClient
// 		}

// 		// Instantiate controller and reconciliation request.
// 		controller := createTrainingJobReconciler(k8sClient, endpointInspector, NewAwsConfigLoaderForEnv(mockEnv), 1)
// 		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 		// Run test
// 		_, err := controller.Reconcile(request)

// 		// Verify expectations
// 		Expect(receivedRequests.Len()).To(Equal(1))
// 		Expect(err).ToNot(HaveOccurred())

// 		Expect(actualEndpointResolver).ToNot(BeNil())
// 		actualEndpoint, err := (*actualEndpointResolver).ResolveEndpoint(sagemaker.EndpointsID, uuid.New().String())
// 		Expect(err).ToNot(HaveOccurred())
// 		Expect(actualEndpoint.URL).To(Equal(expectedEndpoint))
// 	})
// })

// var _ = Describe("Reconciling an existing job", func() {

// 	var (
// 		job              *trainingjobv1.TrainingJob
// 		receivedRequests List
// 		builder          *MockSageMakerClientBuilder
// 		description      sagemaker.DescribeTrainingJobOutput
// 		err              error
// 	)

// 	BeforeEach(func() {

// 		job = createTrainingJobWithGeneratedName(true)

// 		// Create job in Kubernetes.
// 		err = k8sClient.Create(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		setTrainingJobStatus(job, InitializingJobStatus)

// 		// Create SageMaker mock API client.
// 		receivedRequests = List{}
// 		builder = NewMockSageMakerClientBuilder(GinkgoT())

// 		// Create SageMaker mock description.
// 		description = createDescriptionFromSmTrainingJob(job)
// 	})

// 	AfterEach(func() {
// 		// Get job so we can delete it.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		if err != nil && apierrs.IsNotFound(err) {
// 			return
// 		}
// 		Expect(err).ToNot(HaveOccurred())

// 		// Remove finalizer so that the job can be deleted.
// 		job.Finalizers = []string{}
// 		err = k8sClient.Update(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert that deletionTimestamp is nonzero.
// 		err = k8sClient.Delete(context.Background(), job)
// 		Expect(err).ToNot(HaveOccurred())

// 		// Assert deleted.
// 		err = k8sClient.Get(context.Background(), types.NamespacedName{
// 			Namespace: job.ObjectMeta.Namespace,
// 			Name:      job.ObjectMeta.Name,
// 		}, job)
// 		Expect(err).To(HaveOccurred())
// 		Expect(apierrs.IsNotFound(err)).To(Equal(true))
// 	})

// 	Context("when the spec is updated", func() {

// 		BeforeEach(func() {

// 			// Get job so we can modify it.
// 			err = k8sClient.Get(context.Background(), types.NamespacedName{
// 				Namespace: job.ObjectMeta.Namespace,
// 				Name:      job.ObjectMeta.Name,
// 			}, job)
// 			Expect(err).ToNot(HaveOccurred())

// 			job.Spec.ResourceConfig.VolumeSizeInGB = ToInt64Ptr(*job.Spec.ResourceConfig.VolumeSizeInGB + 5)
// 			err = k8sClient.Update(context.Background(), job)
// 			Expect(err).ToNot(HaveOccurred())

// 		})

// 		It("should set status to failed", func() {
// 			// Not failed
// 			description.TrainingJobStatus = sagemaker.TrainingJobStatusInProgress

// 			// Setup mock responses.
// 			sageMakerClient := builder.
// 				WithRequestList(&receivedRequests).
// 				AddDescribeTrainingJobResponse(description).
// 				Build()

// 			// Instantiate controller and reconciliation request.
// 			controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 			// Run test and verify expectations.
// 			controller.Reconcile(request)

// 			// Verify status is failed.
// 			err = k8sClient.Get(context.Background(), types.NamespacedName{
// 				Namespace: job.ObjectMeta.Namespace,
// 				Name:      job.ObjectMeta.Name,
// 			}, job)
// 			Expect(job.Status.TrainingJobStatus).To(Equal(string(sagemaker.TrainingJobStatusFailed)))
// 		})

// 		It("should set additional to contain 'the resource no longer matches'", func() {
// 			// Setup mock responses.
// 			sageMakerClient := builder.
// 				WithRequestList(&receivedRequests).
// 				AddDescribeTrainingJobResponse(description).
// 				Build()

// 			// Instantiate controller and reconciliation request.
// 			controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 			// Run test and verify expectations.
// 			controller.Reconcile(request)

// 			// Verify status is failed.
// 			err = k8sClient.Get(context.Background(), types.NamespacedName{
// 				Namespace: job.ObjectMeta.Namespace,
// 				Name:      job.ObjectMeta.Name,
// 			}, job)
// 			Expect(job.Status.Additional).To(ContainSubstring("the resource no longer matches"))
// 		})

// 		It("should not requeue", func() {
// 			// Setup mock responses.
// 			sageMakerClient := builder.
// 				WithRequestList(&receivedRequests).
// 				AddDescribeTrainingJobResponse(description).
// 				Build()

// 			// Instantiate controller and reconciliation request.
// 			controller := createTrainingJobReconcilerForSageMakerClient(k8sClient, sageMakerClient, 1)
// 			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

// 			// Run test and verify expectations.
// 			reconciliationResult, err := controller.Reconcile(request)

// 			Expect(receivedRequests.Len()).To(Equal(1))
// 			Expect(err).ToNot(HaveOccurred())
// 			Expect(reconciliationResult.Requeue).To(Equal(false))
// 			Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
// 		})
// 	})
// })

func createReconcilerWithMockedDependencies(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalStr string) *Reconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return &Reconciler{
		Client:                k8sClient,
		Log:                   ctrl.Log,
		PollInterval:          pollInterval,
		createSageMakerClient: CreateMockSageMakerClientWrapperProvider(sageMakerClient),
		awsConfigLoader:       CreateMockAwsConfigLoader(),
	}
}

func createReconciler(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalStr string) *Reconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return &Reconciler{
		Client:                k8sClient,
		Log:                   ctrl.Log,
		PollInterval:          pollInterval,
		createSageMakerClient: CreateMockSageMakerClientWrapperProvider(sageMakerClient),
		awsConfigLoader:       CreateMockAwsConfigLoader(),
	}
}

func createTrainingJobWithGeneratedNames() *trainingjobv1.TrainingJob {
	k8sName := "training-job-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()
	sageMakerName := "training-job-" + uuid.New().String()
	return createTrainingJob(k8sName, k8sNamespace, sageMakerName)
}

func createTrainingJob(k8sName, k8sNamespace, smName string) *trainingjobv1.TrainingJob {
	return &trainingjobv1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: k8sNamespace,
		},
		Spec: trainingjobv1.TrainingJobSpec{
			TrainingJobName: &smName,
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
			TrainingJobStatus: controllers.InitializingJobStatus,
		},
	}
}

// Add a finalizer to the deployment.
func AddFinalizer(trainingJob *trainingjobv1.TrainingJob) {
	var actual trainingjobv1.TrainingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	actual.ObjectMeta.Finalizers = []string{controllers.SageMakerResourceFinalizerName}

	Expect(k8sClient.Update(context.Background(), &actual)).To(Succeed())
}

// Set the deletion timestamp to be nonzero.
func SetDeletionTimestamp(trainingJob *trainingjobv1.TrainingJob) {
	var actual trainingjobv1.TrainingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(k8sClient.Delete(context.Background(), &actual)).To(Succeed())
}

// Expect the controller return value to be RequeueAfterInterval, with the poll duration specified.
func ExpectRequeueAfterInterval(result ctrl.Result, err error, pollDuration string) {
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(Equal(false))
	Expect(result.RequeueAfter).To(Equal(ParseDurationOrFail(pollDuration)))
}

// Expect the controller return value to be RequeueImmediately.
func ExpectRequeueImmediately(result ctrl.Result, err error) {
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(Equal(true))
	Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
}

// Expect trainingjob.Status and trainingJob.SecondaryStatus to have the given values.
func ExpectAdditionalToContain(trainingJob *trainingjobv1.TrainingJob, substring string) {
	var actual trainingjobv1.TrainingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.Status.Additional).To(ContainSubstring(substring))
}

// Expect trainingjob status to be as specified.
func ExpectStatusToBe(trainingJob *trainingjobv1.TrainingJob, primaryStatus, secondaryStatus string) {
	var actual trainingjobv1.TrainingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(string(actual.Status.TrainingJobStatus)).To(Equal(primaryStatus))
	Expect(string(actual.Status.SecondaryStatus)).To(Equal(secondaryStatus))
}

// Expect the training job to have the specified finalizer.
func ExpectToHaveFinalizer(trainingJob *trainingjobv1.TrainingJob, finalizer string) {
	var actual trainingjobv1.TrainingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.ObjectMeta.Finalizers).To(ContainElement(finalizer))
}

// Expect the training job to not exist.
func ExpectTrainingJobToBeDeleted(trainingJob *trainingjobv1.TrainingJob) {
	var actual trainingjobv1.TrainingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).To(HaveOccurred())
	Expect(apierrs.IsNotFound(err)).To(Equal(true))
}

// Helper function to create a DescribeTrainingJobOutput.
func CreateDescribeOutputWithOnlyStatus(status sagemaker.TrainingJobStatus, secondaryStatus sagemaker.SecondaryStatus) sagemaker.DescribeTrainingJobOutput {
	return sagemaker.DescribeTrainingJobOutput{
		TrainingJobStatus: status,
		SecondaryStatus:   secondaryStatus,
	}
}

/// ----------------------------
/// OLD CODE BEGINS BELOW
/// ----------------------------

// func createTrainingJobWithNoSageMakerName(hasFinalizer bool) *trainingjobv1.TrainingJob {
// 	return createTrainingJob(generateTrainingJobK8sName(), "", hasFinalizer)
// }

// func createTrainingJobWithGeneratedName(hasFinalizer bool) *trainingjobv1.TrainingJob {
// 	k8sName := generateTrainingJobK8sName()
// 	sageMakerName := "sagemaker-" + k8sName
// 	return createTrainingJob(k8sName, sageMakerName, hasFinalizer)
// }

// // Helper function to create a sagemaker.DescribeTrainingJobOutput from an TrainingJob.
// func createDescriptionFromSmTrainingJob(job *trainingjobv1.TrainingJob) sagemaker.DescribeTrainingJobOutput {
// 	return sagemaker.DescribeTrainingJobOutput{
// 		TrainingJobName: job.Spec.TrainingJobName,
// 		AlgorithmSpecification: &sagemaker.AlgorithmSpecification{
// 			TrainingInputMode: sagemaker.TrainingInputMode(job.Spec.AlgorithmSpecification.TrainingInputMode),
// 		},
// 		OutputDataConfig: &sagemaker.OutputDataConfig{
// 			S3OutputPath: job.Spec.OutputDataConfig.S3OutputPath,
// 		},
// 		ResourceConfig: &sagemaker.ResourceConfig{
// 			InstanceCount:  job.Spec.ResourceConfig.InstanceCount,
// 			InstanceType:   sagemaker.TrainingInstanceType(job.Spec.ResourceConfig.InstanceType),
// 			VolumeSizeInGB: job.Spec.ResourceConfig.VolumeSizeInGB,
// 		},
// 		RoleArn:           job.Spec.RoleArn,
// 		StoppingCondition: &sagemaker.StoppingCondition{},
// 	}
// }

// func setTrainingJobStatus(job *trainingjobv1.TrainingJob, status string) {
// 	job.Status = trainingjobv1.TrainingJobStatus{
// 		TrainingJobStatus: status,
// 	}
// 	err := k8sClient.Status().Update(context.Background(), job)
// 	Expect(err).ToNot(HaveOccurred())
// }
