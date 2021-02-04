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

package processingjob

import (
	"context"

	. "container/list"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	processingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/processingjob"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/aws/aws-sdk-go/service/sagemaker/sagemakeriface"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("Reconciling a ProcessingJob while failing to get the Kubernetes job", func() {
	var (
		sageMakerClient sagemakeriface.SageMakerAPI

		// The custom reconciler to use
		reconciler *Reconciler

		// The controller result.
		reconcileResult ctrl.Result

		// The controller error result.
		reconcileError error
	)

	BeforeEach(func() {
		sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).Build()
	})

	JustBeforeEach(func() {
		request := CreateReconciliationRequest("non-existent-name", "namespace")

		reconcileResult, reconcileError = reconciler.Reconcile(request)
	})

	Context("No error with the K8s client", func() {
		BeforeEach(func() {
			reconciler = createReconcilerWithMockedDependencies(k8sClient, sageMakerClient, "1s")
		})

		It("should not requeue", func() {
			ExpectNoRequeue(reconcileResult, reconcileError)
		})
	})

	Context("An error occurred with the K8s client", func() {
		BeforeEach(func() {
			mockK8sClient := FailToGetK8sClient{}
			reconciler = createReconcilerWithMockedDependencies(mockK8sClient, sageMakerClient, "1s")
		})

		It("should requeue immediately", func() {
			ExpectRequeueImmediately(reconcileResult, reconcileError)
		})
	})
})

var _ = Describe("Reconciling a ProcessingJob that exists", func() {

	var (
		// The requests received by the mock SageMaker client.
		receivedRequests List

		// SageMaker client builder used to create mock responses.
		mockSageMakerClientBuilder *MockSageMakerClientBuilder

		// The total number of requests added to the mock SageMaker client builder.
		expectedRequestCount int

		// The mock processing job.
		processingJob *processingjobv1.ProcessingJob

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

		processingJob = createProcessingJobWithGeneratedNames()
	})

	JustBeforeEach(func() {
		sageMakerClient := mockSageMakerClientBuilder.Build()
		expectedRequestCount = mockSageMakerClientBuilder.GetAddedResponsesLen()

		controller := createReconciler(kubernetesClient, sageMakerClient, pollDuration)

		err := k8sClient.Create(context.Background(), processingJob)
		Expect(err).ToNot(HaveOccurred())

		if shouldHaveFinalizer {
			AddFinalizer(processingJob)
		}

		if shouldHaveDeletionTimestamp {
			SetDeletionTimestamp(processingJob)
		}

		request := CreateReconciliationRequest(processingJob.ObjectMeta.GetName(), processingJob.ObjectMeta.GetNamespace())
		reconcileResult, reconcileError = controller.Reconcile(request)
	})

	AfterEach(func() {
		Expect(receivedRequests.Len()).To(Equal(expectedRequestCount), "Expect that all SageMaker responses were consumed")
	})

	Context("DescribeProcessingJob fails", func() {

		var failureMessage string

		BeforeEach(func() {
			failureMessage = "error message " + uuid.New().String()
			mockSageMakerClientBuilder.AddDescribeProcessingJobErrorResponse("Exception", failureMessage, 500, "request id")
		})

		It("Requeues immediately", func() {
			ExpectRequeueImmediately(reconcileResult, reconcileError)
		})

		It("Updates status", func() {
			ExpectAdditionalToContain(processingJob, failureMessage)
			ExpectProcessingJobStatusToBe(processingJob, controllers.ErrorStatus)
		})
	})

	Context("K8s client fails to update generated spec name", func() {
		BeforeEach(func() {
			kubernetesClient = FailToUpdateK8sClient{ActualClient: kubernetesClient}

			shouldHaveDeletionTimestamp = false
			shouldHaveFinalizer = true
		})

		It("Requeues immediately", func() {
			ExpectRequeueImmediately(reconcileResult, reconcileError)
		})
	})

	Context("ProcessingJob does not exist", func() {

		BeforeEach(func() {
			mockSageMakerClientBuilder.
				AddDescribeProcessingJobErrorResponse(clientwrapper.DescribeProcessingJob404Code, clientwrapper.DescribeProcessingJob404MessagePrefix, 400, "request id")
		})

		Context("HasDeletionTimestamp", func() {
			BeforeEach(func() {
				shouldHaveDeletionTimestamp = true
				shouldHaveFinalizer = true
			})

			It("Removes finalizer and deletes ProcessingJob", func() {
				ExpectProcessingJobToBeDeleted(processingJob)
			})

			It("Requeues after interval", func() {
				ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
			})
		})

		Context("!HasDeletionTimestamp", func() {
			When("CreateJobReturnsNonRecoverableError", func() {
				failureReason := "ValidationException"
				expectedStatus := controllers.ErrorStatus
				BeforeEach(func() {
					mockSageMakerClientBuilder.AddCreateProcessingJobErrorResponse(failureReason, "Invalid Parameter", 400, "request-id")
				})

				It("Requeue Immediately", func() {
					ExpectRequeueImmediately(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(expectedStatus))
				})

				It("Has the additional field set", func() {
					ExpectAdditionalToContain(processingJob, failureReason)
				})
			})

			When("DescribeJobReturnsThrottlingError", func() {
				failureReason := "ThrottlingException"
				expectedStatus := ReconcilingProcessingJobStatus
				BeforeEach(func() {
					mockSageMakerClientBuilder.AddCreateProcessingJobErrorResponse(failureReason, "Invalid Parameter", 400, "request-id")
				})

				It("Requeue Immediately", func() {
					ExpectRequeueImmediately(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(expectedStatus))
				})

				It("Has the additional field set", func() {
					ExpectAdditionalToContain(processingJob, failureReason)
				})
			})

			When("CreateJobisSuccessful", func() {
				BeforeEach(func() {
					mockSageMakerClientBuilder.
						AddCreateProcessingJobResponse(sagemaker.CreateProcessingJobOutput{}).
						AddDescribeProcessingJobResponse(CreateDescribeOutputWithOnlyStatus(sagemaker.ProcessingJobStatusInProgress))

					shouldHaveDeletionTimestamp = false
					shouldHaveFinalizer = true
				})

				It("Creates a ProcessingJob", func() {
					req := receivedRequests.Front().Next().Value
					Expect(req).To(BeAssignableToTypeOf((*sagemaker.CreateProcessingJobInput)(nil)))

					createdRequest := req.(*sagemaker.CreateProcessingJobInput)
					Expect(*createdRequest.ProcessingJobName).To(Equal(controllers.GetGeneratedJobName(processingJob.ObjectMeta.GetUID(), processingJob.ObjectMeta.GetName(), MaxProcessingJobNameLength)))
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(sagemaker.ProcessingJobStatusInProgress))
				})

				It("Adds the processing job name to the status", func() {
					ExpectProcessingJobNameInStatus(controllers.GetGeneratedJobName(processingJob.ObjectMeta.GetUID(), processingJob.ObjectMeta.GetName(), MaxProcessingJobNameLength), processingJob)
				})
			})
		})
	})

	Context("ProcessingJob exists", func() {

		var expectedStatus sagemaker.ProcessingJobStatus

		BeforeEach(func() {
			shouldHaveFinalizer = true
		})

		Context("ProcessingJob has status 'InProgress'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.ProcessingJobStatusInProgress
				mockSageMakerClientBuilder.
					AddDescribeProcessingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(processingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
					expectedStatus = sagemaker.ProcessingJobStatusStopping
					mockSageMakerClientBuilder.
						AddStopProcessingJobResponse(sagemaker.StopProcessingJobOutput{}).
						AddDescribeProcessingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))
				})

				It("Stops the ProcessingJob", func() {
					ExpectRequestToStopProcessingJob(receivedRequests.Front().Next().Value, processingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Stopping' and does not delete ProcessingJob", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(expectedStatus))
				})
			})
		})

		Context("ProcessingJob has status 'Stopping'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.ProcessingJobStatusStopping
				mockSageMakerClientBuilder.
					AddDescribeProcessingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(processingJob, controllers.SageMakerResourceFinalizerName)
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

				It("Updates status to 'Stopping' and does not delete ProcessingJob", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(sagemaker.ProcessingJobStatusStopping))
				})
			})
		})

		Context("ProcessingJob has status 'Failed'", func() {
			var failureReason string

			BeforeEach(func() {
				expectedStatus = sagemaker.ProcessingJobStatusFailed
				failureReason = "Failure within the processing job"

				// Add the failure reason to the describe output
				describeOutput := CreateDescribeOutputWithOnlyStatus(expectedStatus)
				describeOutput.FailureReason = ToStringPtr(failureReason)

				mockSageMakerClientBuilder.
					AddDescribeProcessingJobResponse(describeOutput)
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(expectedStatus))
				})

				It("Has the additional field set", func() {
					ExpectAdditionalToContain(processingJob, failureReason)
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(processingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the processingJob job", func() {
					ExpectProcessingJobToBeDeleted(processingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})
			})
		})

		Context("ProcessingJob has status 'Stopped'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.ProcessingJobStatusStopped
				mockSageMakerClientBuilder.
					AddDescribeProcessingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(processingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the processing job", func() {
					ExpectProcessingJobToBeDeleted(processingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})
			})
		})

		Context("ProcessingJob has status 'Completed'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.ProcessingJobStatusCompleted
				mockSageMakerClientBuilder.
					AddDescribeProcessingJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectProcessingJobStatusToBe(processingJob, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(processingJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the processing job", func() {
					ExpectProcessingJobToBeDeleted(processingJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})
			})
		})
	})
})

func createReconcilerWithMockedDependencies(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.SageMakerAPI, pollIntervalStr string) *Reconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return &Reconciler{
		Client:                k8sClient,
		Log:                   ctrl.Log,
		PollInterval:          pollInterval,
		createSageMakerClient: CreateMockSageMakerClientWrapperProvider(sageMakerClient),
		awsConfigLoader:       CreateMockAwsConfigLoader(),
	}
}

func createReconciler(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.SageMakerAPI, pollIntervalStr string) *Reconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return &Reconciler{
		Client:                k8sClient,
		Log:                   ctrl.Log,
		PollInterval:          pollInterval,
		createSageMakerClient: CreateMockSageMakerClientWrapperProvider(sageMakerClient),
		awsConfigLoader:       CreateMockAwsConfigLoader(),
	}
}

func createProcessingJobWithGeneratedNames() *processingjobv1.ProcessingJob {
	k8sName := "processing-job-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()

	CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)

	return createProcessingJob(k8sName, k8sNamespace)
}

func createProcessingJob(k8sName, k8sNamespace string) *processingjobv1.ProcessingJob {
	return &processingjobv1.ProcessingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: k8sNamespace,
		},
		Spec: processingjobv1.ProcessingJobSpec{
			AppSpecification: &commonv1.AppSpecification{
				ContainerEntrypoint: []string{"python", "run_me.py"},
				ContainerArguments:  []string{"--region", "usa"},
				ImageURI:            "hello-world",
			},
			ProcessingOutputConfig: &commonv1.ProcessingOutputConfig{
				Outputs: []commonv1.ProcessingOutputStruct{
					{
						OutputName: "xyz",
						S3Output: commonv1.ProcessingS3Output{
							LocalPath:    "/opt/ml/output",
							S3UploadMode: "EndOfJob",
							S3Uri:        "s3://bucket/processing_output",
						},
					},
				},
			},
			ProcessingResources: &commonv1.ProcessingResources{
				ClusterConfig: &commonv1.ResourceConfig{
					InstanceCount:  ToInt64Ptr(1),
					InstanceType:   "xyz",
					VolumeSizeInGB: ToInt64Ptr(50),
				},
			},
			RoleArn:           ToStringPtr("xxxxxxxxxxxxxxxxxxxx"),
			Region:            ToStringPtr("region-xyz"),
			StoppingCondition: &commonv1.StoppingConditionNoSpot{},
		},
	}
}

// Add a finalizer to the deployment.
func AddFinalizer(processingJob *processingjobv1.ProcessingJob) {
	var actual processingjobv1.ProcessingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: processingJob.ObjectMeta.Namespace,
		Name:      processingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	actual.ObjectMeta.Finalizers = []string{controllers.SageMakerResourceFinalizerName}

	Expect(k8sClient.Update(context.Background(), &actual)).To(Succeed())
}

// Set the deletion timestamp to be nonzero.
func SetDeletionTimestamp(processingJob *processingjobv1.ProcessingJob) {
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: processingJob.ObjectMeta.Namespace,
		Name:      processingJob.ObjectMeta.Name,
	}, processingJob)
	Expect(err).ToNot(HaveOccurred())

	Expect(k8sClient.Delete(context.Background(), processingJob)).To(Succeed())
}

// Expect processingJob.Status to have the given values.
func ExpectAdditionalToContain(processingJob *processingjobv1.ProcessingJob, substring string) {
	var actual processingjobv1.ProcessingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: processingJob.ObjectMeta.Namespace,
		Name:      processingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.Status.Additional).To(ContainSubstring(substring))
}

// Expect processingjob status to be as specified.
func ExpectProcessingJobStatusToBe(processingJob *processingjobv1.ProcessingJob, primaryStatus string) {
	var actual processingjobv1.ProcessingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: processingJob.ObjectMeta.Namespace,
		Name:      processingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(string(actual.Status.ProcessingJobStatus)).To(Equal(primaryStatus))
}

// Expect the processing job to have the specified finalizer.
func ExpectToHaveFinalizer(processingJob *processingjobv1.ProcessingJob, finalizer string) {
	var actual processingjobv1.ProcessingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: processingJob.ObjectMeta.Namespace,
		Name:      processingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.ObjectMeta.Finalizers).To(ContainElement(finalizer))
}

// Expect the processing job to not exist.
func ExpectProcessingJobToBeDeleted(processingJob *processingjobv1.ProcessingJob) {
	var actual processingjobv1.ProcessingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: processingJob.ObjectMeta.Namespace,
		Name:      processingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).To(HaveOccurred())
	Expect(apierrs.IsNotFound(err)).To(Equal(true))
}

// Helper function to create a DescribeProcessingJobOutput.
func CreateDescribeOutputWithOnlyStatus(status sagemaker.ProcessingJobStatus) sagemaker.DescribeProcessingJobOutput {
	return sagemaker.DescribeProcessingJobOutput{
		ProcessingJobStatus: status,
	}
}

// Helper function to verify that the specified object is a StopProcessingJobInput and that it requests to delete the ProcessingJob.
func ExpectRequestToStopProcessingJob(req interface{}, processingJob *processingjobv1.ProcessingJob) {
	Expect(req).To(BeAssignableToTypeOf((*sagemaker.StopProcessingJobInput)(nil)))

	stopRequest := req.(*sagemaker.StopProcessingJobInput)
	Expect(*stopRequest.ProcessingJobName).To(Equal(controllers.GetGeneratedJobName(processingJob.ObjectMeta.GetUID(), processingJob.ObjectMeta.GetName(), MaxProcessingJobNameLength)))
}

// Expect the SageMakerProcessingJobName to be set with a given value in the processing job status.
func ExpectProcessingJobNameInStatus(processingJobName string, processingJob *processingjobv1.ProcessingJob) {
	var actual processingjobv1.ProcessingJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: processingJob.ObjectMeta.Namespace,
		Name:      processingJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.Status.SageMakerProcessingJobName).To(Equal(processingJobName))
}
