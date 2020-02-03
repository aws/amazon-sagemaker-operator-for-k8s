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

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
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

		ExpectNoRequeue(result, err)
	})

	It("should requeue if there was an error", func() {
		mockK8sClient := FailToGetK8sClient{}
		controller := createReconcilerWithMockedDependencies(mockK8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		ExpectRequeueImmediately(result, err)
	})
})

var _ = Describe("Reconciling a TrainingJob that exists", func() {

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

	Context("K8s client fails to update", func() {
		BeforeEach(func() {
			kubernetesClient = FailToUpdateK8sClient{ActualClient: kubernetesClient}

			shouldHaveDeletionTimestamp = false
			shouldHaveFinalizer = true
		})

		It("Requeues immediately", func() {
			ExpectRequeueImmediately(reconcileResult, reconcileError)
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

			It("Removes finalizer and deletes TrainingJob", func() {
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
				Expect(*createdRequest.TrainingJobName).To(Equal(controllers.GetGeneratedJobName(trainingJob.ObjectMeta.GetUID(), trainingJob.ObjectMeta.GetName(), MaxTrainingJobNameLength)))
			})

			It("Requeues after interval", func() {
				ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
			})

			It("Updates status", func() {
				ExpectStatusToBe(trainingJob, string(sagemaker.TrainingJobStatusInProgress), string(sagemaker.SecondaryStatusStarting))
			})

			It("Adds the training job name to the spec", func() {
				ExpectTrainingJobNameInSpec(controllers.GetGeneratedJobName(trainingJob.ObjectMeta.GetUID(), trainingJob.ObjectMeta.GetName(), MaxTrainingJobNameLength), trainingJob)
			})

			It("Adds the training job name to the status", func() {
				ExpectTrainingJobNameInStatus(controllers.GetGeneratedJobName(trainingJob.ObjectMeta.GetUID(), trainingJob.ObjectMeta.GetName(), MaxTrainingJobNameLength), trainingJob)
			})

			Context("Spec defines TrainingJobName", func() {
				var (
					// Defines the training job name that would be specified in the spec.
					specifiedTrainingJobName string
				)

				BeforeEach(func() {
					specifiedTrainingJobName = "training-job-name"
					trainingJob.Spec.TrainingJobName = ToStringPtr(specifiedTrainingJobName)
				})

				It("Creates a TrainingJob", func() {
					req := receivedRequests.Front().Next().Value
					Expect(req).To(BeAssignableToTypeOf((*sagemaker.CreateTrainingJobInput)(nil)))

					createdRequest := req.(*sagemaker.CreateTrainingJobInput)
					Expect(*createdRequest.TrainingJobName).To(Equal(specifiedTrainingJobName))
				})

				It("Does not modify the job name in the spec", func() {
					ExpectTrainingJobNameInSpec(specifiedTrainingJobName, trainingJob)
				})

				It("Adds the training job name to the status", func() {
					ExpectTrainingJobNameInStatus(specifiedTrainingJobName, trainingJob)
				})
			})
		})
	})

	Context("TrainingJob exists", func() {

		var expectedStatus sagemaker.TrainingJobStatus
		var expectedSecondaryStatus sagemaker.SecondaryStatus

		BeforeEach(func() {
			shouldHaveFinalizer = true

			expectedSecondaryStatus = ""
		})

		Context("TrainingJob has status 'InProgress'('Starting')", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.TrainingJobStatusInProgress
				expectedSecondaryStatus = sagemaker.SecondaryStatusStarting
				mockSageMakerClientBuilder.
					AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), string(expectedSecondaryStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(trainingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
					expectedStatus = sagemaker.TrainingJobStatusStopping
					expectedSecondaryStatus = sagemaker.SecondaryStatusStarting
					mockSageMakerClientBuilder.
						AddStopTrainingJobResponse(sagemaker.StopTrainingJobOutput{}).
						AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
				})

				It("Stops the TrainingJob", func() {
					ExpectRequestToStopTrainingJob(receivedRequests.Front().Next().Value, trainingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Stopping'('') and does not delete TrainingJob", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), "")
				})
			})
		})

		Context("TrainingJob has status 'InProgress'('Training')", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.TrainingJobStatusInProgress
				expectedSecondaryStatus = sagemaker.SecondaryStatusTraining
				mockSageMakerClientBuilder.
					AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), string(expectedSecondaryStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(trainingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
					expectedStatus = sagemaker.TrainingJobStatusStopping
					expectedSecondaryStatus = sagemaker.SecondaryStatusTraining
					mockSageMakerClientBuilder.
						AddStopTrainingJobResponse(sagemaker.StopTrainingJobOutput{}).
						AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
				})

				It("Stops the TrainingJob", func() {
					ExpectRequestToStopTrainingJob(receivedRequests.Front().Next().Value, trainingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Stopping'('') and does not delete TrainingJob", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), "")
				})
			})
		})

		Context("TrainingJob has status 'Stopping'('Starting')", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.TrainingJobStatusStopping
				expectedSecondaryStatus = sagemaker.SecondaryStatusStarting
				mockSageMakerClientBuilder.
					AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), "")
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(trainingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Stopping' and does not delete TrainingJob", func() {
					ExpectStatusToBe(trainingJob, string(sagemaker.TrainingJobStatusStopping), "")
				})
			})
		})

		Context("TrainingJob has status 'Stopping'('Downloading')", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.TrainingJobStatusStopping
				expectedSecondaryStatus = sagemaker.SecondaryStatusDownloading
				mockSageMakerClientBuilder.
					AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), "")
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(trainingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Stopping' and does not delete TrainingJob", func() {
					ExpectStatusToBe(trainingJob, string(sagemaker.TrainingJobStatusStopping), "")
				})
			})
		})

		Context("TrainingJob has status 'Failed'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.TrainingJobStatusFailed
				mockSageMakerClientBuilder.
					AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), string(expectedSecondaryStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(trainingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the training job", func() {
					ExpectTrainingJobToBeDeleted(trainingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})
			})
		})

		Context("TrainingJob has status 'Stopped'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.TrainingJobStatusStopped
				mockSageMakerClientBuilder.
					AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), string(expectedSecondaryStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(trainingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the training job", func() {
					ExpectTrainingJobToBeDeleted(trainingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})
			})
		})

		Context("TrainingJob has status 'Completed'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.TrainingJobStatusCompleted
				mockSageMakerClientBuilder.
					AddDescribeTrainingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, expectedSecondaryStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectStatusToBe(trainingJob, string(expectedStatus), string(expectedSecondaryStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(trainingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the training job", func() {
					ExpectTrainingJobToBeDeleted(trainingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})
			})
		})
	})

})

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
	return createTrainingJob(k8sName, k8sNamespace)
}

func createTrainingJob(k8sName, k8sNamespace string) *trainingjobv1.TrainingJob {
	return &trainingjobv1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: k8sNamespace,
		},
		Spec: trainingjobv1.TrainingJobSpec{
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
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, trainingJob)
	Expect(err).ToNot(HaveOccurred())

	Expect(k8sClient.Delete(context.Background(), trainingJob)).To(Succeed())
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

// Helper function to verify that the specified object is a StopTrainingJobInput and that it requests to delete the TrainingJob.
func ExpectRequestToStopTrainingJob(req interface{}, trainingJob *trainingjobv1.TrainingJob) {
	Expect(req).To(BeAssignableToTypeOf((*sagemaker.StopTrainingJobInput)(nil)))

	stopRequest := req.(*sagemaker.StopTrainingJobInput)
	Expect(*stopRequest.TrainingJobName).To(Equal(controllers.GetGeneratedJobName(trainingJob.ObjectMeta.GetUID(), trainingJob.ObjectMeta.GetName(), MaxTrainingJobNameLength)))
}

// Expect the SageMakerTrainingJobName to be set with a given value in the training job status.
func ExpectTrainingJobNameInStatus(trainingJobName string, trainingJob *trainingjobv1.TrainingJob) {
	var actual trainingjobv1.TrainingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.Status.SageMakerTrainingJobName).To(Equal(trainingJobName))
}

// Expect the TrainingJobName to be set with a given value in the spec.
func ExpectTrainingJobNameInSpec(trainingJobName string, trainingJob *trainingjobv1.TrainingJob) {
	var actual trainingjobv1.TrainingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: trainingJob.ObjectMeta.Namespace,
		Name:      trainingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(*actual.Spec.TrainingJobName).To(Equal(trainingJobName))
}
