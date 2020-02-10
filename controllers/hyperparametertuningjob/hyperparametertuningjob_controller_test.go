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
	"errors"

	. "container/list"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	hpojobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hyperparametertuningjob"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
	"github.com/go-logr/logr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("Reconciling a HyperParameterTuningJob while failing to get the Kubernetes job", func() {
	var (
		sageMakerClient sagemakeriface.ClientAPI

		// The custom HPO reconciler to use
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
			reconciler = createReconciler(k8sClient, sageMakerClient, "1s", mockTrackingHPOTrainingJobSpawner{})
		})

		It("should not requeue", func() {
			ExpectNoRequeue(reconcileResult, reconcileError)
		})
	})

	Context("An error occurred with the K8s client", func() {
		BeforeEach(func() {
			mockK8sClient := FailToGetK8sClient{}
			reconciler = createReconciler(mockK8sClient, sageMakerClient, "1s", mockTrackingHPOTrainingJobSpawner{})
		})

		It("should requeue immediately", func() {
			ExpectRequeueImmediately(reconcileResult, reconcileError)
		})
	})
})

var _ = Describe("Reconciling a HyperParameterTuningJob that exists", func() {

	var (
		// The requests received by the mock SageMaker client.
		receivedRequests List

		// SageMaker client builder used to create mock responses.
		mockSageMakerClientBuilder *MockSageMakerClientBuilder

		// A mock job spawner.
		jobSpawner *mockTrackingHPOTrainingJobSpawner

		// The total number of requests added to the mock SageMaker client builder.
		expectedRequestCount int

		// The mock hyperparametertuning job.
		tuningJob *hpojobv1.HyperparameterTuningJob

		// A summary of the HyperParameterTuningJob best training job.
		bestTrainingJob *sagemaker.HyperParameterTrainingJobSummary

		// The status counters to associate with our HPO job.
		statusCounters *sagemaker.TrainingJobStatusCounters

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

		bestTrainingJob = &sagemaker.HyperParameterTrainingJobSummary{
			TrainingJobName: ToStringPtr("best-training-job"),
		}

		statusCounters = &sagemaker.TrainingJobStatusCounters{
			Completed:         ToInt64Ptr(1),
			InProgress:        ToInt64Ptr(2),
			NonRetryableError: ToInt64Ptr(3),
			RetryableError:    ToInt64Ptr(4),
			Stopped:           ToInt64Ptr(5),
		}

		jobSpawner = &mockTrackingHPOTrainingJobSpawner{
			spawnMissingTrainingJobsCalls:  &List{},
			deleteSpawnedTrainingJobsCalls: &List{},
		}
		tuningJob = createHyperParameterTuningJobWithGeneratedNames()
	})

	JustBeforeEach(func() {
		sageMakerClient := mockSageMakerClientBuilder.Build()
		expectedRequestCount = mockSageMakerClientBuilder.GetAddedResponsesLen()

		controller := createReconciler(kubernetesClient, sageMakerClient, pollDuration, jobSpawner)

		err := k8sClient.Create(context.Background(), tuningJob)
		Expect(err).ToNot(HaveOccurred())

		if shouldHaveFinalizer {
			AddFinalizer(tuningJob)
		}

		if shouldHaveDeletionTimestamp {
			SetDeletionTimestamp(tuningJob)
		}

		request := CreateReconciliationRequest(tuningJob.ObjectMeta.GetName(), tuningJob.ObjectMeta.GetNamespace())
		reconcileResult, reconcileError = controller.Reconcile(request)
	})

	AfterEach(func() {
		Expect(receivedRequests.Len()).To(Equal(expectedRequestCount), "Expect that all SageMaker responses were consumed")
	})

	Context("DescribeHyperParameterTuningJob fails", func() {

		var failureMessage string

		BeforeEach(func() {
			failureMessage = "error message " + uuid.New().String()
			mockSageMakerClientBuilder.AddDescribeHyperParameterTuningJobErrorResponse("Exception", failureMessage, 500, "request id")
		})

		It("Requeues immediately", func() {
			ExpectRequeueImmediately(reconcileResult, reconcileError)
		})

		It("Updates status", func() {
			ExpectAdditionalToContain(tuningJob, failureMessage)
			ExpectStatusToBe(tuningJob, ReconcilingTuningJobStatus)
		})
	})

	Context("HyperParameterTuningJob does not exist", func() {

		BeforeEach(func() {
			mockSageMakerClientBuilder.
				AddDescribeHyperParameterTuningJobErrorResponse(clientwrapper.DescribeHyperParameterTuningJob404Code, clientwrapper.DescribeHyperParameterTuningJob404MessagePrefix, 400, "request id")
		})

		Context("HasDeletionTimestamp", func() {

			BeforeEach(func() {
				shouldHaveDeletionTimestamp = true
				shouldHaveFinalizer = true
			})

			It("Removes finalizer and deletes HyperParameterTuningJob", func() {
				ExpectHyperParameterTuningJobToBeDeleted(tuningJob)
			})

			It("Requeues after interval", func() {
				ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
			})
		})

		Context("!HasDeletionTimestamp", func() {
			BeforeEach(func() {
				mockSageMakerClientBuilder.
					AddCreateHyperParameterTuningJobResponse(sagemaker.CreateHyperParameterTuningJobOutput{}).
					AddDescribeHyperParameterTuningJobResponse(CreateDescribeOutputWithOnlyStatus(sagemaker.HyperParameterTuningJobStatusInProgress, bestTrainingJob, statusCounters))

				shouldHaveDeletionTimestamp = false
				shouldHaveFinalizer = true
			})

			It("Creates a HyperParameterTuningJob", func() {
				req := receivedRequests.Front().Next().Value
				ExpectRequestToCreateHyperParameterTuningJob(req, controllers.GetGeneratedJobName(tuningJob.ObjectMeta.GetUID(), tuningJob.ObjectMeta.GetName(), MaxHyperParameterTuningJobNameLength))
			})

			It("Requeues after interval", func() {
				ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
			})

			It("Updates status", func() {
				ExpectStatusToBe(tuningJob, string(sagemaker.HyperParameterTuningJobStatusInProgress))
			})

			Context("Spec defines HyperParameterTuningJobName", func() {
				BeforeEach(func() {
					tuningJob.Spec.HyperParameterTuningJobName = ToStringPtr("tuning-job-name")
				})

				It("Creates a HyperParameterTuningJob", func() {
					req := receivedRequests.Front().Next().Value
					ExpectRequestToCreateHyperParameterTuningJob(req, "tuning-job-name")
				})
			})
		})
	})

	Context("HyperParameterTuningJob exists", func() {

		var expectedStatus sagemaker.HyperParameterTuningJobStatus

		BeforeEach(func() {
			shouldHaveFinalizer = true
		})

		Context("HyperParameterTuningJob has status 'InProgress'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.HyperParameterTuningJobStatusInProgress
				mockSageMakerClientBuilder.
					AddDescribeHyperParameterTuningJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, bestTrainingJob, statusCounters))
			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(tuningJob, string(expectedStatus))
				})

				It("Attempts to spawn missing Training Jobs", func() {
					ExpectSpawnMissingTrainingJobs(*jobSpawner, 1, *tuningJob)
				})

				It("Updates the BestTrainingJob in the status", func() {
					ExpectBestTrainingJobToBe(tuningJob, bestTrainingJob)
				})

				It("Updates the TrainingJobStatusCounters in the status", func() {
					ExpectTrainingJobStatusCountersToBe(tuningJob, statusCounters)
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(tuningJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
					expectedStatus = sagemaker.HyperParameterTuningJobStatusStopping
					mockSageMakerClientBuilder.
						AddStopHyperParameterTuningJobResponse(sagemaker.StopHyperParameterTuningJobOutput{}).
						AddDescribeHyperParameterTuningJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, bestTrainingJob, statusCounters))
				})

				It("Stops the HyperParameterTuningJob", func() {
					ExpectRequestToStopHyperParameterTuningJob(receivedRequests.Front().Next().Value, tuningJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Stopping' and does not delete HyperParameterTuningJob", func() {
					ExpectStatusToBe(tuningJob, string(expectedStatus))
				})
			})
		})

		Context("HyperParameterTuningJob has status 'Stopping'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.HyperParameterTuningJobStatusStopping
				mockSageMakerClientBuilder.
					AddDescribeHyperParameterTuningJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, bestTrainingJob, statusCounters))
			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(tuningJob, string(expectedStatus))
				})

				It("Attempts to spawn missing Training Jobs", func() {
					ExpectSpawnMissingTrainingJobs(*jobSpawner, 1, *tuningJob)
				})

				It("Updates the BestTrainingJob in the status", func() {
					ExpectBestTrainingJobToBe(tuningJob, bestTrainingJob)
				})

				It("Updates the TrainingJobStatusCounters in the status", func() {
					ExpectTrainingJobStatusCountersToBe(tuningJob, statusCounters)
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(tuningJob, controllers.SageMakerResourceFinalizerName)
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

				It("Updates status to 'Stopping' and does not delete HyperParameterTuningJob", func() {
					ExpectStatusToBe(tuningJob, string(sagemaker.HyperParameterTuningJobStatusStopping))
				})
			})
		})

		Context("HyperParameterTuningJob has status 'Failed'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.HyperParameterTuningJobStatusFailed
				mockSageMakerClientBuilder.
					AddDescribeHyperParameterTuningJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, bestTrainingJob, statusCounters))
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectStatusToBe(tuningJob, string(expectedStatus))
				})

				It("Attempts to spawn missing Training Jobs", func() {
					ExpectSpawnMissingTrainingJobs(*jobSpawner, 1, *tuningJob)
				})

				It("Updates the BestTrainingJob in the status", func() {
					ExpectBestTrainingJobToBe(tuningJob, bestTrainingJob)
				})

				It("Updates the TrainingJobStatusCounters in the status", func() {
					ExpectTrainingJobStatusCountersToBe(tuningJob, statusCounters)
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(tuningJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the tuning job", func() {
					ExpectHyperParameterTuningJobToBeDeleted(tuningJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Attempts to delete all spawned training jobs", func() {
					ExpectDeletedSpawnedTrainingJobs(*jobSpawner, 1, *tuningJob)
				})

				Context("Failed to delete spawned training jobs", func() {
					BeforeEach(func() {
						jobSpawner = &mockTrackingHPOTrainingJobSpawner{
							deleteShouldFail:               true,
							spawnMissingTrainingJobsCalls:  &List{},
							deleteSpawnedTrainingJobsCalls: &List{},
						}
					})

					It("Requeue immediately", func() {
						ExpectRequeueImmediately(reconcileResult, reconcileError)
					})
				})
			})
		})

		Context("HyperParameterTuningJob has status 'Stopped'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.HyperParameterTuningJobStatusStopped
				mockSageMakerClientBuilder.
					AddDescribeHyperParameterTuningJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, bestTrainingJob, statusCounters))
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectStatusToBe(tuningJob, string(expectedStatus))
				})

				It("Attempts to spawn missing Training Jobs", func() {
					ExpectSpawnMissingTrainingJobs(*jobSpawner, 1, *tuningJob)
				})

				It("Updates the BestTrainingJob in the status", func() {
					ExpectBestTrainingJobToBe(tuningJob, bestTrainingJob)
				})

				It("Updates the TrainingJobStatusCounters in the status", func() {
					ExpectTrainingJobStatusCountersToBe(tuningJob, statusCounters)
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(tuningJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the tuning job", func() {
					ExpectHyperParameterTuningJobToBeDeleted(tuningJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Attempts to delete all spawned training jobs", func() {
					ExpectDeletedSpawnedTrainingJobs(*jobSpawner, 1, *tuningJob)
				})
			})
		})

		Context("HyperParameterTuningJob has status 'Completed'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.HyperParameterTuningJobStatusCompleted

				mockSageMakerClientBuilder.
					AddDescribeHyperParameterTuningJobResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus, bestTrainingJob, statusCounters))
			})

			When("!HasDeletionTimestamp", func() {
				It("Doesn't requeue", func() {
					ExpectNoRequeue(reconcileResult, reconcileError)
				})

				It("Updates status", func() {
					ExpectStatusToBe(tuningJob, string(expectedStatus))
				})

				It("Attempts to spawn missing Training Jobs", func() {
					ExpectSpawnMissingTrainingJobs(*jobSpawner, 1, *tuningJob)
				})

				It("Updates the BestTrainingJob in the status", func() {
					ExpectBestTrainingJobToBe(tuningJob, bestTrainingJob)
				})

				It("Updates the TrainingJobStatusCounters in the status", func() {
					ExpectTrainingJobStatusCountersToBe(tuningJob, statusCounters)
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(tuningJob, controllers.SageMakerResourceFinalizerName)
					})
				})
			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the tuning job", func() {
					ExpectHyperParameterTuningJobToBeDeleted(tuningJob)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Attempts to delete all spawned training jobs", func() {
					ExpectDeletedSpawnedTrainingJobs(*jobSpawner, 1, *tuningJob)
				})
			})
		})
	})
})

// Mock HPOTrainingJobSpawner that tracks number of calls to each method.
type mockTrackingHPOTrainingJobSpawner struct {
	// Determine whether the deleteSpawnedTrainingJob call should return an error.
	deleteShouldFail bool

	// The number of times SpawnMissingTrainingJobs was called.
	spawnMissingTrainingJobsCalls *List

	// The number of times DeleteSpawnedTrainingJobs was called.
	deleteSpawnedTrainingJobsCalls *List
}

var _ HPOTrainingJobSpawner = (*mockTrackingHPOTrainingJobSpawner)(nil)

// SpawnMissingTrainingJobs increments the call count when called..
func (s mockTrackingHPOTrainingJobSpawner) SpawnMissingTrainingJobs(_ context.Context, job hpojobv1.HyperparameterTuningJob) {
	s.spawnMissingTrainingJobsCalls.PushBack(job)
}

// DeleteSpawnedTrainingJobs increments the call count when called.
func (s mockTrackingHPOTrainingJobSpawner) DeleteSpawnedTrainingJobs(_ context.Context, job hpojobv1.HyperparameterTuningJob) error {
	s.deleteSpawnedTrainingJobsCalls.PushBack(job)
	if s.deleteShouldFail {
		return errors.New("failed delete spawned training jobs")
	}
	return nil
}

// Create a provider that creates a mock HPO TrainingJob Spawner.
func createMockHPOTrainingJobSpawnerProvider(spawner HPOTrainingJobSpawner) HPOTrainingJobSpawnerProvider {
	return func(_ client.Client, _ logr.Logger, _ clientwrapper.SageMakerClientWrapper) HPOTrainingJobSpawner {
		return spawner
	}
}

func createReconciler(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalStr string, hpoJobSpawner HPOTrainingJobSpawner) *Reconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return &Reconciler{
		Client:                      k8sClient,
		Log:                         ctrl.Log,
		PollInterval:                pollInterval,
		createSageMakerClient:       CreateMockSageMakerClientWrapperProvider(sageMakerClient),
		awsConfigLoader:             CreateMockAwsConfigLoader(),
		createHPOTrainingJobSpawner: createMockHPOTrainingJobSpawnerProvider(hpoJobSpawner),
	}
}

func createHyperParameterTuningJobWithGeneratedNames() *hpojobv1.HyperparameterTuningJob {
	k8sName := "hyperparameter-tuning-job-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()
	return createHyperParameterTuningJob(k8sName, k8sNamespace)
}

func createHyperParameterTuningJob(k8sName, k8sNamespace string) *hpojobv1.HyperparameterTuningJob {
	return &hpojobv1.HyperparameterTuningJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: k8sNamespace,
		},
		Spec: hpojobv1.HyperparameterTuningJobSpec{
			Region: ToStringPtr("region-xyz"),
			HyperParameterTuningJobConfig: &commonv1.HyperParameterTuningJobConfig{
				ResourceLimits: &commonv1.ResourceLimits{
					MaxNumberOfTrainingJobs: ToInt64Ptr(15),
					MaxParallelTrainingJobs: ToInt64Ptr(5),
				},
				Strategy: "strategy-type",
			},
			TrainingJobDefinition: &commonv1.HyperParameterTrainingJobDefinition{
				AlgorithmSpecification: &commonv1.HyperParameterAlgorithmSpecification{
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
				StoppingCondition: &commonv1.StoppingCondition{},
			},
		},
	}
}

// Add a finalizer to the deployment.
func AddFinalizer(tuningJob *hpojobv1.HyperparameterTuningJob) {
	var actual hpojobv1.HyperparameterTuningJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: tuningJob.ObjectMeta.Namespace,
		Name:      tuningJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	actual.ObjectMeta.Finalizers = []string{controllers.SageMakerResourceFinalizerName}

	Expect(k8sClient.Update(context.Background(), &actual)).To(Succeed())
}

// Set the deletion timestamp to be nonzero.
func SetDeletionTimestamp(tuningJob *hpojobv1.HyperparameterTuningJob) {
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: tuningJob.ObjectMeta.Namespace,
		Name:      tuningJob.ObjectMeta.Name,
	}, tuningJob)
	Expect(err).ToNot(HaveOccurred())

	Expect(k8sClient.Delete(context.Background(), tuningJob)).To(Succeed())
}

// Expect trainingjob.Status to have the given values.
func ExpectAdditionalToContain(tuningJob *hpojobv1.HyperparameterTuningJob, substring string) {
	var actual hpojobv1.HyperparameterTuningJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: tuningJob.ObjectMeta.Namespace,
		Name:      tuningJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.Status.Additional).To(ContainSubstring(substring))
}

// Expect trainingjob status to be as specified.
func ExpectStatusToBe(tuningJob *hpojobv1.HyperparameterTuningJob, primaryStatus string) {
	var actual hpojobv1.HyperparameterTuningJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: tuningJob.ObjectMeta.Namespace,
		Name:      tuningJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(string(actual.Status.HyperParameterTuningJobStatus)).To(Equal(primaryStatus))
}

// Expect the training job to have the specified finalizer.
func ExpectToHaveFinalizer(tuningJob *hpojobv1.HyperparameterTuningJob, finalizer string) {
	var actual hpojobv1.HyperparameterTuningJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: tuningJob.ObjectMeta.Namespace,
		Name:      tuningJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.ObjectMeta.Finalizers).To(ContainElement(finalizer))
}

// Expect the training job to not exist.
func ExpectHyperParameterTuningJobToBeDeleted(tuningJob *hpojobv1.HyperparameterTuningJob) {
	var actual hpojobv1.HyperparameterTuningJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: tuningJob.ObjectMeta.Namespace,
		Name:      tuningJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).To(HaveOccurred())
	Expect(apierrs.IsNotFound(err)).To(Equal(true))
}

// Expect the BestTrainingJob to be set.
func ExpectBestTrainingJobToBe(tuningJob *hpojobv1.HyperparameterTuningJob, job *sagemaker.HyperParameterTrainingJobSummary) {
	var actual hpojobv1.HyperparameterTuningJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: tuningJob.ObjectMeta.Namespace,
		Name:      tuningJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(*actual.Status.BestTrainingJob.TrainingJobName).To(Equal(*job.TrainingJobName))
}

// Expect the TrainingJobStatusCounters to be converted and match from the initial description.
func ExpectTrainingJobStatusCountersToBe(tuningJob *hpojobv1.HyperparameterTuningJob, counters *sagemaker.TrainingJobStatusCounters) {
	var actual hpojobv1.HyperparameterTuningJob
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: tuningJob.ObjectMeta.Namespace,
		Name:      tuningJob.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(*actual.Status.TrainingJobStatusCounters.Completed).To(Equal(*counters.Completed))
}

// Helper function to create a DescribeHyperParameterTuningJobOutput.
func CreateDescribeOutputWithOnlyStatus(status sagemaker.HyperParameterTuningJobStatus, bestTrainingJob *sagemaker.HyperParameterTrainingJobSummary, statusCounters *sagemaker.TrainingJobStatusCounters) sagemaker.DescribeHyperParameterTuningJobOutput {
	return sagemaker.DescribeHyperParameterTuningJobOutput{
		HyperParameterTuningJobStatus: status,
		BestTrainingJob:               bestTrainingJob,
		TrainingJobStatusCounters:     statusCounters,
	}
}

// Helper function to verify that there was an attempt to create a HyperParameterTuningJob with a given name.
func ExpectRequestToCreateHyperParameterTuningJob(req interface{}, tuningJobName string) {
	Expect(req).To(BeAssignableToTypeOf((*sagemaker.CreateHyperParameterTuningJobInput)(nil)))

	createdRequest := req.(*sagemaker.CreateHyperParameterTuningJobInput)
	Expect(*createdRequest.HyperParameterTuningJobName).To(Equal(tuningJobName))
}

// Helper function to verify that the specified object is a StopHyperParameterTuningJobInput and that it requests to delete the HyperParameterTuningJob.
func ExpectRequestToStopHyperParameterTuningJob(req interface{}, tuningJob *hpojobv1.HyperparameterTuningJob) {
	Expect(req).To(BeAssignableToTypeOf((*sagemaker.StopHyperParameterTuningJobInput)(nil)))

	stopRequest := req.(*sagemaker.StopHyperParameterTuningJobInput)
	Expect(*stopRequest.HyperParameterTuningJobName).To(Equal(controllers.GetGeneratedJobName(tuningJob.ObjectMeta.GetUID(), tuningJob.ObjectMeta.GetName(), MaxHyperParameterTuningJobNameLength)))
}

// Helper function to verify that the controller attempted to delete the spawned training jobs.
func ExpectDeletedSpawnedTrainingJobs(spawner mockTrackingHPOTrainingJobSpawner, calls int, job hpojobv1.HyperparameterTuningJob) {
	Expect(spawner.deleteSpawnedTrainingJobsCalls.Len()).To(Equal(calls))

	calledJob := spawner.spawnMissingTrainingJobsCalls.Front().Value.(hpojobv1.HyperparameterTuningJob)
	Expect(*calledJob.Spec.Region).To(Equal(*job.Spec.Region))
	Expect(calledJob.ObjectMeta.GetName()).To(Equal(job.ObjectMeta.GetName()))
	Expect(calledJob.ObjectMeta.GetNamespace()).To(Equal(job.ObjectMeta.GetNamespace()))
}

// Helper function to verify that the controller attempted to spawn the missing child training jobs.
func ExpectSpawnMissingTrainingJobs(spawner mockTrackingHPOTrainingJobSpawner, calls int, job hpojobv1.HyperparameterTuningJob) {
	Expect(spawner.spawnMissingTrainingJobsCalls.Len()).To(Equal(calls))

	calledJob := spawner.spawnMissingTrainingJobsCalls.Front().Value.(hpojobv1.HyperparameterTuningJob)
	Expect(*calledJob.Spec.Region).To(Equal(*job.Spec.Region))
	Expect(calledJob.ObjectMeta.GetName()).To(Equal(job.ObjectMeta.GetName()))
	Expect(calledJob.ObjectMeta.GetNamespace()).To(Equal(job.ObjectMeta.GetNamespace()))
}
