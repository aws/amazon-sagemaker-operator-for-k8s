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

package hyperparametertuningjob

import (
	"context"
	"fmt"
	"os"
	"time"

	. "container/list"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/adammck/venv"
	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	hpojobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hyperparametertuningjob"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

// Mock HpoTrainingJobSpawner that does nothing when called.
type noopHpoTrainingJobSpawner struct {
	HpoTrainingJobSpawner
}

// Do nothing when called.
func (_ noopHpoTrainingJobSpawner) SpawnMissingTrainingJobs(_ context.Context, _ hpojobv1.HyperparameterTuningJob) {
}

// Do nothing when called.
func (_ noopHpoTrainingJobSpawner) DeleteSpawnedTrainingJobs(_ context.Context, _ hpojobv1.HyperparameterTuningJob) error {
	return nil
}

// Create a provider that creates a mock HPO TrainingJob Spawner.
func createMockHpoTrainingJobSpawnerProvider(spawner HpoTrainingJobSpawner) HpoTrainingJobSpawnerProvider {
	return func(_ client.Client, _ logr.Logger, _ sagemakeriface.ClientAPI) HpoTrainingJobSpawner {
		return spawner
	}
}

// Create an HPO reconciler for a given SageMaker client.
func createHpoReconcilerForSageMakerClient(k8sClient client.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalStr string) HyperparameterTuningJobReconciler {
	sageMakerClientProvider := CreateMockSageMakerClientProvider(sageMakerClient)
	return createHpoReconciler(k8sClient, sageMakerClientProvider, createMockHpoTrainingJobSpawnerProvider(noopHpoTrainingJobSpawner{}), CreateMockAwsConfigLoader(), pollIntervalStr)
}

// Create an HPO reconciler for a SageMaker client provider and hpoTrainingJobSpawner provider.
func createHpoReconciler(k8sClient client.Client, sageMakerClientProvider SageMakerClientProvider, hpoTrainingJobSpawnerProvider HpoTrainingJobSpawnerProvider, awsConfigLoader AwsConfigLoader, pollIntervalStr string) HyperparameterTuningJobReconciler {

	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return HyperparameterTuningJobReconciler{
		Client:                      k8sClient,
		Log:                         ctrl.Log,
		PollInterval:                pollInterval,
		createSageMakerClient:       sageMakerClientProvider,
		createHpoTrainingJobSpawner: hpoTrainingJobSpawnerProvider,
		awsConfigLoader:             awsConfigLoader,
	}
}

var _ = Describe("Reconciling a job while failing to get the Kubernetes job", func() {

	var (
		sageMakerClient sagemakeriface.ClientAPI
	)

	BeforeEach(func() {
		sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).Build()
	})

	It("should not requeue if the job does not exist", func() {
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should requeue if there was an error", func() {
		mockK8sClient := FailToGetK8sClient{}
		controller := createHpoReconcilerForSageMakerClient(mockK8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		_, err := controller.Reconcile(request)

		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Reconciling a job with finalizer that is being deleted", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *hpojobv1.HyperparameterTuningJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeHyperParameterTuningJob request.
		description sagemaker.DescribeHyperParameterTuningJobOutput
		err         error
	)

	BeforeEach(func() {

		job = createHpoJobWithGeneratedNames(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, InitializingJobStatus)

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		// Create SageMaker mock description.
		description = createDescriptionFromSageMakerHyperParameterTuningJob(job)
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
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusInProgress
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			AddStopHyperParameterTuningJobResponse(sagemaker.StopHyperParameterTuningJobOutput{}).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		// In HPO we requeue immediately unlike training
		Expect(reconciliationResult.Requeue).To(Equal(true))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
		// SageMaker will have two request, describe and delete
		Expect(receivedRequests.Len()).To(Equal(2))
	})

	It("should update the status and retry if SageMaker throttles", func() {
		rateExceededMessage := "Rate exceeded"
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobErrorResponseWithMessage("ThrottlingException", 400, "request id", rateExceededMessage).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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

	It("should update the status and requeue if the job is stopping", func() {
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusStopping
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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

		Expect(job.Status.HyperParameterTuningJobStatus).To(ContainSubstring(string(sagemaker.HyperParameterTuningJobStatusStopping)))
	})

	It("should remove the finalizer and not requeue if the job is stopped", func() {
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusStopped
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusFailed
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusCompleted
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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

	Context("DeleteSpawnedTrainingJobs", func() {
		It("should call DeleteSpawnedTrainingJobs if the job is stopped", func() {

			description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusStopped

			// Setup mock responses.
			sageMakerClient := builder.
				AddDescribeHyperParameterTuningJobResponse(description).
				Build()

			// Setup mock that reports the arguments it was called with
			deleteSpawnedTrainingJobHpoJobs := List{}

			mockHpoTrainingJobSpawner := callTrackingHpoTrainingJobSpawner{
				DeleteSpawnedTrainingJobHpoJobs: &deleteSpawnedTrainingJobHpoJobs,
			}

			// Create controller with mocks and its request
			controller := createHpoReconciler(k8sClient, CreateMockSageMakerClientProvider(sageMakerClient), createMockHpoTrainingJobSpawnerProvider(mockHpoTrainingJobSpawner), CreateMockAwsConfigLoader(), "1s")
			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

			// Run test and verify expectations.
			controller.Reconcile(request)

			// Verify that the hpoTrainingJobSpawner was called
			Expect(deleteSpawnedTrainingJobHpoJobs.Len()).To(Equal(1))

			// Verify that the hpoTrainingJobSpawner received the correct parameters.
			actualHpoJob := deleteSpawnedTrainingJobHpoJobs.Front().Value.(hpojobv1.HyperparameterTuningJob)
			Expect(*actualHpoJob.Spec.HyperParameterTuningJobName).To(Equal(*job.Spec.HyperParameterTuningJobName))
			Expect(actualHpoJob.ObjectMeta.GetNamespace()).To(Equal(job.ObjectMeta.GetNamespace()))
		})

		It("should requeue if DeleteSpawnedTrainingJobs failed", func() {

			description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusStopped
			// Setup mock responses.
			sageMakerClient := builder.
				AddDescribeHyperParameterTuningJobResponse(description).
				Build()

			// Setup mock that reports the arguments it was called with
			deleteSpawnedTrainingJobHpoJobs := List{}
			deleteSpawnedTrainingJobReturnValues := List{}

			deleteSpawnedTrainingJobReturnValues.PushBack(fmt.Errorf("some error"))

			mockHpoTrainingJobSpawner := callTrackingHpoTrainingJobSpawner{
				DeleteSpawnedTrainingJobHpoJobs:      &deleteSpawnedTrainingJobHpoJobs,
				DeleteSpawnedTrainingJobReturnValues: &deleteSpawnedTrainingJobReturnValues,
			}

			// Create controller with mocks and its request
			controller := createHpoReconciler(k8sClient, CreateMockSageMakerClientProvider(sageMakerClient), createMockHpoTrainingJobSpawnerProvider(mockHpoTrainingJobSpawner), CreateMockAwsConfigLoader(), "1s")
			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

			// Run test and verify expectations.
			reconciliationResult, err := controller.Reconcile(request)

			// Expect no error
			Expect(err).ToNot(HaveOccurred())

			// Expect requeue after interval
			Expect(reconciliationResult.Requeue).To(Equal(false))
			Expect(reconciliationResult.RequeueAfter).To(Equal(controller.PollInterval))

			// Verify that the hpoTrainingJobSpawner was called
			Expect(deleteSpawnedTrainingJobHpoJobs.Len()).To(Equal(1))

			// Verify that the hpoTrainingJobSpawner received the correct parameters.
			actualHpoJob := deleteSpawnedTrainingJobHpoJobs.Front().Value.(hpojobv1.HyperparameterTuningJob)
			Expect(*actualHpoJob.Spec.HyperParameterTuningJobName).To(Equal(*job.Spec.HyperParameterTuningJobName))
			Expect(actualHpoJob.ObjectMeta.GetNamespace()).To(Equal(job.ObjectMeta.GetNamespace()))
		})
	})
})

var _ = Describe("Reconciling a job without finalizer that is being deleted", func() {

	var (
		// The Kubernetes job that the controller will reconcile.
		job *hpojobv1.HyperparameterTuningJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder
		// The SageMaker response for a DescribeHyperParameterTuningJob request.
		description sagemaker.DescribeHyperParameterTuningJobOutput
		err         error
	)

	BeforeEach(func() {

		job = createHpoJobWithGeneratedNames(false)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, InitializingJobStatus)

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		// Create SageMaker mock description.
		description = createDescriptionFromSageMakerHyperParameterTuningJob(job)
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
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusInProgress
		// Setup mock responses.
		sageMakerClient := builder.
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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

var _ = Describe("Reconciling a job that has no finalizer", func() {

	var (
		job     *hpojobv1.HyperparameterTuningJob
		builder *MockSageMakerClientBuilder
		err     error
	)

	BeforeEach(func() {

		job = createHpoJobWithGeneratedNames(false)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, InitializingJobStatus)

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
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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
		job     *hpojobv1.HyperparameterTuningJob
		builder *MockSageMakerClientBuilder
		err     error
	)

	BeforeEach(func() {
		job = createHpoJobWithGeneratedNames(false)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, "")
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
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		controller.Reconcile(request)

		// Verify status is updated.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(job.Status.HyperParameterTuningJobStatus).To(Equal(InitializingJobStatus))
	})

	It("should requeue immediately", func() {
		sageMakerClient := builder.Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		reconciliationResult, err := controller.Reconcile(request)

		// Verify expectations
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(true))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Reconciling a job with a finalizer but no name", func() {
	var (
		// The Kubernetes job that the controller will reconcile.
		job *hpojobv1.HyperparameterTuningJob
		// The list of requests that the mock SageMaker API has recieved.
		receivedRequests List
		// A builder for mock SageMaker API clients.
		builder *MockSageMakerClientBuilder

		err error
	)

	BeforeEach(func() {
		job = createHpoJobWithGeneratedNames(true)
		job.Spec.HyperParameterTuningJobName = ToStringPtr("")

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, InitializingJobStatus)

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
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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
		controller := createHpoReconcilerForSageMakerClient(mockK8sClient, sageMakerClient, "1s")
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

var _ = Describe("Reconciling a job that does not exist in SageMaker", func() {

	var (
		job              *hpojobv1.HyperparameterTuningJob
		receivedRequests List
		builder          *MockSageMakerClientBuilder
		err              error
	)

	BeforeEach(func() {
		job = createHpoJobWithGeneratedNames(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, InitializingJobStatus)

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
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

		// Get job so we can remove finalizers.
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

		// Assert deleted.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("should try to create the SageMaker job, if success then requeue immediately", func() {
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobErrorResponse(hpoResourceNotFoundApiCode, 400, "request id").
			AddCreateHyperParameterTuningJobResponse(sagemaker.CreateHyperParameterTuningJobOutput{HyperParameterTuningJobArn: ToStringPtr("output arn")}).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(true))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
		Expect(receivedRequests.Len()).To(Equal(2))
	})

	It("should try to create the SageMaker job, if 4xx then return without requeue", func() {
		sageMakerErrorMessage := "mock validation error"

		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobErrorResponse(hpoResourceNotFoundApiCode, 400, "request id").
			AddCreateHyperParameterTuningJobErrorResponse(sageMakerErrorMessage, 400, "request id").
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
		Expect(receivedRequests.Len()).To(Equal(2))

		// Verify status is updated.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(job.Status.Additional).To(ContainSubstring(sageMakerErrorMessage))
	})

	It("should try to create the SageMaker job, if 5xx then requeue after interval", func() {
		sageMakerErrorMessage := "sagemaker service is down, please try again later."

		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobErrorResponse(hpoResourceNotFoundApiCode, 400, "request id").
			AddCreateHyperParameterTuningJobErrorResponse(sageMakerErrorMessage, 500, "request id").
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(1e9)))
		Expect(receivedRequests.Len()).To(Equal(2))

		// Verify status is updated.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(job.Status.Additional).To(ContainSubstring(sageMakerErrorMessage))

	})

	It("should try to describe the SageMaker job using the correct AWS region", func() {

		expectedRegion := *job.Spec.Region

		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobErrorResponse("some error", 400, "request id").
			Build()

		actualRegion := ""

		regionInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualRegion = awsConfig.Region
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createHpoReconciler(k8sClient, regionInspector, createMockHpoTrainingJobSpawnerProvider(noopHpoTrainingJobSpawner{}), CreateMockAwsConfigLoader(), "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		controller.Reconcile(request)
		Expect(actualRegion).To(Equal(expectedRegion))
	})
})

var _ = Describe("Reconciling a job when SageMaker Describe returns an error", func() {
	var (
		job              *hpojobv1.HyperparameterTuningJob
		receivedRequests List
		builder          *MockSageMakerClientBuilder
		err              error
	)

	BeforeEach(func() {
		job = createHpoJobWithGeneratedNames(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, InitializingJobStatus)

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
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

		// Get job so we can remove finalizers.
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

		// Assert deleted.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	// Sagemaker returns 400 for throttlingException
	It("requeue after interval if error is 429", func() {
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobErrorResponseWithMessage("ThrottlingException", 400, "request id", "Rate exceeded").
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		// Expect requeue after interval
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(controller.PollInterval))
		Expect(receivedRequests.Len()).To(Equal(1))
	})

	It("should update status and not requeue if error is 4xx", func() {

	})
})

var _ = Describe("Reconciling with a custom endpoint", func() {

	var (
		job              *hpojobv1.HyperparameterTuningJob
		receivedRequests List
		builder          *MockSageMakerClientBuilder
		description      sagemaker.DescribeHyperParameterTuningJobOutput
		err              error
		expectedEndpoint string
		mockEnv          venv.Env
	)

	BeforeEach(func() {

		job = createHpoJobWithGeneratedNames(true)

		// Set env variable.
		expectedEndpoint = "https://" + uuid.New().String() + ".com"
		mockEnv = venv.Mock()
		mockEnv.Setenv(DefaultSageMakerEndpointEnvKey, expectedEndpoint)

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		// Create SageMaker mock description.
		description = createDescriptionFromSageMakerHyperParameterTuningJob(job)
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

		setHpoStatus(job, InitializingJobStatus)

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createHpoReconciler(k8sClient, endpointInspector, createMockHpoTrainingJobSpawnerProvider(noopHpoTrainingJobSpawner{}), NewAwsConfigLoaderForEnv(mockEnv), "1s")
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

		setHpoStatus(job, InitializingJobStatus)

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createHpoReconciler(k8sClient, endpointInspector, createMockHpoTrainingJobSpawnerProvider(noopHpoTrainingJobSpawner{}), NewAwsConfigLoaderForEnv(mockEnv), "1s")
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
		err = os.Setenv(DefaultSageMakerEndpointEnvKey, "")
		Expect(err).ToNot(HaveOccurred())

		expectedEndpoint = "https://" + uuid.New().String() + ".expected.com"
		job.Spec.SageMakerEndpoint = &expectedEndpoint

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, InitializingJobStatus)

		// Instantiate dependencies
		sageMakerClient := builder.
			WithRequestList(&receivedRequests).
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		var actualEndpointResolver *aws.EndpointResolver = nil

		endpointInspector := func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			actualEndpointResolver = &awsConfig.EndpointResolver
			return sageMakerClient
		}

		// Instantiate controller and reconciliation request.
		controller := createHpoReconciler(k8sClient, endpointInspector, createMockHpoTrainingJobSpawnerProvider(noopHpoTrainingJobSpawner{}), NewAwsConfigLoaderForEnv(mockEnv), "1s")
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

var _ = Describe("Reconciling a job given a SageMaker description", func() {

	var (
		job              *hpojobv1.HyperparameterTuningJob
		receivedRequests List
		builder          *MockSageMakerClientBuilder
		description      sagemaker.DescribeHyperParameterTuningJobOutput
		err              error
	)

	BeforeEach(func() {

		job = createHpoJobWithGeneratedNames(true)

		// Create job in Kubernetes.
		err = k8sClient.Create(context.Background(), job)
		Expect(err).ToNot(HaveOccurred())

		setHpoStatus(job, InitializingJobStatus)

		// Create SageMaker mock API client.
		receivedRequests = List{}
		builder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		// Create SageMaker mock description.
		description = createDescriptionFromSageMakerHyperParameterTuningJob(job)
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

	It("should update status with an error message and not requeue if the SageMaker description does not match the spec", func() {

	})

	It("should create k8s jobs for every new TrainingJob", func() {

	})

	It("should not re-create k8s jobs for TrainingJobs that have already been mapped in k8s", func() {

	})

	It("should requeue after interval if the SageMaker status matches the k8s status", func() {

	})

	It("should update the status and requeue after interval if the SageMaker status does not match the k8s status", func() {

	})

	It("should update the status and not requeue a completed job", func() {
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusCompleted
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should update the status and not requeue a stopped job", func() {
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusStopped
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should update the status and not requeue a failed job", func() {
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusFailed
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should update the status and and requeue when job is in progress", func() {
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusInProgress
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(controller.PollInterval))
	})

	It("should update the status and and requeue when job is in stopping", func() {
		description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusStopping
		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Instantiate controller and reconciliation request.
		controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test and verify expectations.
		reconciliationResult, err := controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(1))
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciliationResult.Requeue).To(Equal(false))
		Expect(reconciliationResult.RequeueAfter).To(Equal(controller.PollInterval))
	})

	It("should update the BestTrainingJob if one is present", func() {

		// Set mock BestTrainingJob.TrainingJobName
		expectedTrainingJobName := "best-training-job-name-123"
		description.BestTrainingJob = &sagemaker.HyperParameterTrainingJobSummary{
			TrainingJobName: &expectedTrainingJobName,
		}

		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Create controller with mocks and its request
		controller := createHpoReconciler(k8sClient, CreateMockSageMakerClientProvider(sageMakerClient), createMockHpoTrainingJobSpawnerProvider(noopHpoTrainingJobSpawner{}), CreateMockAwsConfigLoader(), "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		_, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())

		// Get job so we can verify it has the correct status.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).ToNot(HaveOccurred())

		Expect(job.Status.BestTrainingJob.TrainingJobName).ToNot(BeNil())
		Expect(*job.Status.BestTrainingJob.TrainingJobName).To(Equal(expectedTrainingJobName))
	})

	It("should update the status TrainingJobStatusCounters", func() {
		// Set status
		var completed int64 = 1
		var inProgress int64 = 2
		var nonRetryableError int64 = 3
		var retryableError int64 = 4
		var stopped int64 = 5

		// Set mock error values.
		description.TrainingJobStatusCounters = &sagemaker.TrainingJobStatusCounters{
			Completed:         &completed,
			InProgress:        &inProgress,
			NonRetryableError: &nonRetryableError,
			RetryableError:    &retryableError,
			Stopped:           &stopped,
		}

		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Create controller with mocks and its request
		controller := createHpoReconciler(k8sClient, CreateMockSageMakerClientProvider(sageMakerClient), createMockHpoTrainingJobSpawnerProvider(noopHpoTrainingJobSpawner{}), CreateMockAwsConfigLoader(), "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		_, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())

		// Get job so we can verify it has the correct status.
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.ObjectMeta.Name,
		}, job)
		Expect(err).ToNot(HaveOccurred())

		Expect(job.Status.TrainingJobStatusCounters).ToNot(BeNil())

		Expect(job.Status.TrainingJobStatusCounters.Completed).ToNot(BeNil())
		Expect(*job.Status.TrainingJobStatusCounters.Completed).To(Equal(completed))

		Expect(job.Status.TrainingJobStatusCounters.InProgress).ToNot(BeNil())
		Expect(*job.Status.TrainingJobStatusCounters.InProgress).To(Equal(inProgress))

		Expect(job.Status.TrainingJobStatusCounters.NonRetryableError).ToNot(BeNil())
		Expect(*job.Status.TrainingJobStatusCounters.NonRetryableError).To(Equal(nonRetryableError))

		Expect(job.Status.TrainingJobStatusCounters.RetryableError).ToNot(BeNil())
		Expect(*job.Status.TrainingJobStatusCounters.RetryableError).To(Equal(retryableError))

		Expect(job.Status.TrainingJobStatusCounters.TotalError).ToNot(BeNil())
		Expect(*job.Status.TrainingJobStatusCounters.TotalError).To(Equal(nonRetryableError + retryableError))
		Expect(job.Status.TrainingJobStatusCounters.Stopped).ToNot(BeNil())
		Expect(*job.Status.TrainingJobStatusCounters.Stopped).To(Equal(stopped))
	})

	It("should call the HpoTrainingJobSpawner", func() {

		// Setup mock responses.
		sageMakerClient := builder.
			AddDescribeHyperParameterTuningJobResponse(description).
			Build()

		// Setup mock that reports the arguments it was called with
		spawnMissingTrainingJobHpoJobs := List{}

		mockHpoTrainingJobSpawner := callTrackingHpoTrainingJobSpawner{
			SpawnMissingTrainingJobHpoJobs: &spawnMissingTrainingJobHpoJobs,
		}

		// Create controller with mocks and its request
		controller := createHpoReconciler(k8sClient, CreateMockSageMakerClientProvider(sageMakerClient), createMockHpoTrainingJobSpawnerProvider(mockHpoTrainingJobSpawner), CreateMockAwsConfigLoader(), "1s")
		request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

		// Run test
		controller.Reconcile(request)

		// Verify that the hpoTrainingJobSpawner was called
		Expect(spawnMissingTrainingJobHpoJobs.Len()).To(Equal(1))

		// Verify that the hpoTrainingJobSpawner received the correct parameters.
		actualHpoJob := spawnMissingTrainingJobHpoJobs.Front().Value.(hpojobv1.HyperparameterTuningJob)
		Expect(*actualHpoJob.Spec.HyperParameterTuningJobName).To(Equal(*job.Spec.HyperParameterTuningJobName))
		Expect(*actualHpoJob.Spec.Region).To(Equal(*job.Spec.Region))
		Expect(actualHpoJob.ObjectMeta.GetNamespace()).To(Equal(job.ObjectMeta.GetNamespace()))

		// Verify that the SageMaker request was made
		Expect(receivedRequests.Len()).To(Equal(1))
	})

	Context("when the spec is updated", func() {

		BeforeEach(func() {

			// Get job so we can modify it.
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: job.ObjectMeta.Namespace,
				Name:      job.ObjectMeta.Name,
			}, job)
			Expect(err).ToNot(HaveOccurred())

			job.Spec.HyperParameterTuningJobConfig.ResourceLimits.MaxNumberOfTrainingJobs = ToInt64Ptr(*job.Spec.HyperParameterTuningJobConfig.ResourceLimits.MaxNumberOfTrainingJobs + 5)
			err = k8sClient.Update(context.Background(), job)
			Expect(err).ToNot(HaveOccurred())

		})

		It("should set status to failed", func() {
			// Not failed
			description.HyperParameterTuningJobStatus = sagemaker.HyperParameterTuningJobStatusInProgress

			// Setup mock responses.
			sageMakerClient := builder.
				WithRequestList(&receivedRequests).
				AddDescribeHyperParameterTuningJobResponse(description).
				Build()

			// Instantiate controller and reconciliation request.
			controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
			request := CreateReconciliationRequest(job.ObjectMeta.Name, job.ObjectMeta.Namespace)

			// Run test and verify expectations.
			controller.Reconcile(request)

			// Verify status is failed.
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: job.ObjectMeta.Namespace,
				Name:      job.ObjectMeta.Name,
			}, job)
			Expect(job.Status.HyperParameterTuningJobStatus).To(Equal(string(sagemaker.HyperParameterTuningJobStatusFailed)))
		})

		It("should set additional to contain 'the resource no longer matches'", func() {
			// Setup mock responses.
			sageMakerClient := builder.
				WithRequestList(&receivedRequests).
				AddDescribeHyperParameterTuningJobResponse(description).
				Build()

			// Instantiate controller and reconciliation request.
			controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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
				AddDescribeHyperParameterTuningJobResponse(description).
				Build()

			// Instantiate controller and reconciliation request.
			controller := createHpoReconcilerForSageMakerClient(k8sClient, sageMakerClient, "1s")
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

// Helper struct that implements HpoTrainingJobSpawner.
// This tracks calls to SpawnMissingTrainingJob so that we can verify it is properly used
// by the HPO controller.
type callTrackingHpoTrainingJobSpawner struct {
	HpoTrainingJobSpawner

	SpawnMissingTrainingJobHpoJobs *List

	DeleteSpawnedTrainingJobHpoJobs      *List
	DeleteSpawnedTrainingJobReturnValues *List
}

// Store arguments given in the call for later verification.
func (s callTrackingHpoTrainingJobSpawner) SpawnMissingTrainingJobs(ctx context.Context, hpoJob hpojobv1.HyperparameterTuningJob) {
	s.SpawnMissingTrainingJobHpoJobs.PushBack(hpoJob)
}

// Store arguments given in the call for later verification.
func (s callTrackingHpoTrainingJobSpawner) DeleteSpawnedTrainingJobs(ctx context.Context, hpoJob hpojobv1.HyperparameterTuningJob) error {
	s.DeleteSpawnedTrainingJobHpoJobs.PushBack(hpoJob)

	if s.DeleteSpawnedTrainingJobReturnValues != nil && s.DeleteSpawnedTrainingJobReturnValues.Len() > 0 {
		returnValue := s.DeleteSpawnedTrainingJobReturnValues.Front()
		s.DeleteSpawnedTrainingJobReturnValues.Remove(returnValue)
		return returnValue.Value.(error)
	} else {
		return nil
	}
}

// Helper function to create a description for HyperParameterTuning Job
func createDescriptionFromSageMakerHyperParameterTuningJob(job *hpojobv1.HyperparameterTuningJob) sagemaker.DescribeHyperParameterTuningJobOutput {

	return sagemaker.DescribeHyperParameterTuningJobOutput{
		HyperParameterTuningJobName: job.Spec.HyperParameterTuningJobName,
		HyperParameterTuningJobConfig: &sagemaker.HyperParameterTuningJobConfig{
			ResourceLimits: &sagemaker.ResourceLimits{
				MaxNumberOfTrainingJobs: job.Spec.HyperParameterTuningJobConfig.ResourceLimits.MaxNumberOfTrainingJobs,
				MaxParallelTrainingJobs: job.Spec.HyperParameterTuningJobConfig.ResourceLimits.MaxParallelTrainingJobs,
			},
			Strategy: sagemaker.HyperParameterTuningJobStrategyType(job.Spec.HyperParameterTuningJobConfig.Strategy),
		},
		TrainingJobDefinition: &sagemaker.HyperParameterTrainingJobDefinition{
			AlgorithmSpecification: &sagemaker.HyperParameterAlgorithmSpecification{
				TrainingInputMode: sagemaker.TrainingInputMode(job.Spec.TrainingJobDefinition.AlgorithmSpecification.TrainingInputMode),
			},
			OutputDataConfig: &sagemaker.OutputDataConfig{
				S3OutputPath: job.Spec.TrainingJobDefinition.OutputDataConfig.S3OutputPath,
			},
			ResourceConfig: &sagemaker.ResourceConfig{
				InstanceCount:  job.Spec.TrainingJobDefinition.ResourceConfig.InstanceCount,
				InstanceType:   sagemaker.TrainingInstanceType(job.Spec.TrainingJobDefinition.ResourceConfig.InstanceType),
				VolumeSizeInGB: job.Spec.TrainingJobDefinition.ResourceConfig.VolumeSizeInGB,
			},
			RoleArn:               job.Spec.TrainingJobDefinition.RoleArn,
			StaticHyperParameters: ConvertKeyValuePairSliceToMap(job.Spec.TrainingJobDefinition.StaticHyperParameters),
			StoppingCondition:     &sagemaker.StoppingCondition{},
		},
	}
}

func createHpoJobWithGeneratedNames(withFinalizer bool) *hpojobv1.HyperparameterTuningJob {
	k8sName := "hpo-job-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()
	sageMakerName := "sagemaker-" + k8sName
	region := "region-" + uuid.New().String()
	return createHpoJob(withFinalizer, k8sName, k8sNamespace, sageMakerName, region)
}

func createHpoJob(withFinalizer bool, k8sName, k8sNamespace, sageMakerName, region string) *hpojobv1.HyperparameterTuningJob {
	finalizers := []string{}
	if withFinalizer {
		finalizers = append(finalizers, SageMakerResourceFinalizerName)
	}

	return &hpojobv1.HyperparameterTuningJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:       k8sName,
			Namespace:  k8sNamespace,
			Finalizers: finalizers,
		},

		Spec: hpojobv1.HyperparameterTuningJobSpec{

			HyperParameterTuningJobConfig: &commonv1.HyperParameterTuningJobConfig{
				ResourceLimits: &commonv1.ResourceLimits{
					MaxNumberOfTrainingJobs: ToInt64Ptr(100),
					MaxParallelTrainingJobs: ToInt64Ptr(9),
				},
				Strategy: "Bayesian",
			},
			HyperParameterTuningJobName: &sageMakerName,
			Region:                      &region,
			TrainingJobDefinition: &commonv1.HyperParameterTrainingJobDefinition{
				AlgorithmSpecification: &commonv1.HyperParameterAlgorithmSpecification{
					TrainingInputMode: "File",
				},
				OutputDataConfig: &commonv1.OutputDataConfig{
					S3OutputPath: ToStringPtr("s3://outputpath"),
				},
				ResourceConfig: &commonv1.ResourceConfig{
					InstanceCount:  ToInt64Ptr(1),
					InstanceType:   "ml.m4.xlarge",
					VolumeSizeInGB: ToInt64Ptr(50),
				},
				RoleArn:               ToStringPtr("xxxxxxxxxxxxxxxxxxxx"),
				StaticHyperParameters: createMockStaticHyperParameters(10),
				StoppingCondition:     &commonv1.StoppingCondition{},
			},
		},
	}
}

func createMockStaticHyperParameters(n int) []*commonv1.KeyValuePair {
	kvps := []*commonv1.KeyValuePair{}

	for i := 0; i < n; i++ {
		kvps = append(kvps, &commonv1.KeyValuePair{
			Name:  uuid.New().String(),
			Value: uuid.New().String(),
		})
	}

	return kvps
}

func setHpoStatus(job *hpojobv1.HyperparameterTuningJob, status string) {
	job.Status = hpojobv1.HyperparameterTuningJobStatus{
		HyperParameterTuningJobStatus: status,
	}
	err := k8sClient.Status().Update(context.Background(), job)
	Expect(err).ToNot(HaveOccurred())
}
