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
	"math/rand"

	. "container/list"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	hpojobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hyperparametertuningjob"
	trainingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/trainingjob"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	// +kubebuilder:scaffold:imports
)

// Helper function to create a HpoTrainingJobSpawner
func createHPOTrainingJobSpawner(k8sClient client.Client, log logr.Logger, sageMakerClient sagemakeriface.ClientAPI) hpoTrainingJobSpawner {
	return hpoTrainingJobSpawner{
		K8sClient:       k8sClient,
		Log:             log,
		SageMakerClient: clientwrapper.NewSageMakerClientWrapper(sageMakerClient),
	}
}

// Create a SageMaker job description.
func createSageMakerJob(name string) sagemaker.DescribeTrainingJobOutput {
	return sagemaker.DescribeTrainingJobOutput{
		TrainingJobName: &name,
		AlgorithmSpecification: &sagemaker.AlgorithmSpecification{
			TrainingInputMode: sagemaker.TrainingInputModeFile,
		},
		OutputDataConfig: &sagemaker.OutputDataConfig{
			S3OutputPath: ToStringPtr("s3://outputpath"),
		},
		ResourceConfig: &sagemaker.ResourceConfig{
			InstanceCount:  ToInt64Ptr(1),
			InstanceType:   sagemaker.TrainingInstanceTypeMlM4Xlarge,
			VolumeSizeInGB: ToInt64Ptr(50),
		},
		RoleArn:           ToStringPtr("xxxxxxxxxxxxxxxxxxxx"),
		StoppingCondition: &sagemaker.StoppingCondition{},
	}
}

// Create a Kubernetes job description.
func createKubernetesJob(withFinalizer bool, name, namespace string) *trainingjobv1.TrainingJob {
	return &trainingjobv1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
			TrainingJobName:   &name,
		},
	}
}

func createHyperParameterTuningJobWithStatus(name, namespace string) *hpojobv1.HyperparameterTuningJob {
	// Create the base spec
	original := createHyperParameterTuningJob(name, namespace)

	jobName := GetGeneratedJobName("uid", name, MaxHyperParameterTuningJobNameLength)

	original.Spec.HyperParameterTuningJobName = &jobName

	// Apply a status over it
	original.Status = hpojobv1.HyperparameterTuningJobStatus{
		HyperParameterTuningJobStatus:        string(sagemaker.HyperParameterTuningJobStatusInProgress),
		SageMakerHyperParameterTuningJobName: jobName,
	}

	return original
}

var _ = Describe("SpawnMissingTrainingJobs", func() {

	var (
		sageMakerClientBuilder *MockSageMakerClientBuilder
		spawner                hpoTrainingJobSpawner
		receivedRequests       List
		err                    error
	)

	BeforeEach(func() {
		receivedRequests = List{}
		sageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
	})

	It("should create K8s TrainingJobs that are missing", func() {

		missing1Name := "missing1-" + uuid.New().String()
		missing2Name := "missing2-" + uuid.New().String()
		present1Name := "present1-" + uuid.New().String()
		present2Name := "present2-" + uuid.New().String()

		// Create a mock response for ListTrainingJobs.
		listResponse := sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput{
			NextToken: ToStringPtr(""),
			TrainingJobSummaries: []sagemaker.HyperParameterTrainingJobSummary{
				sagemaker.HyperParameterTrainingJobSummary{
					TrainingJobName: &present1Name,
				},
				sagemaker.HyperParameterTrainingJobSummary{
					TrainingJobName: &missing1Name,
				},
				sagemaker.HyperParameterTrainingJobSummary{
					TrainingJobName: &missing2Name,
				},
				sagemaker.HyperParameterTrainingJobSummary{
					TrainingJobName: &present2Name,
				},
			},
		}

		namespace := "namespace-1"

		// Create child jobs that already exist in Kubernetes
		err = k8sClient.Create(context.Background(), createKubernetesJob(false, present1Name, namespace))
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Create(context.Background(), createKubernetesJob(false, present2Name, namespace))
		Expect(err).ToNot(HaveOccurred())

		// Create child jobs that only exist in SageMaker
		missing1 := createSageMakerJob(missing1Name)
		missing2 := createSageMakerJob(missing2Name)

		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobResponse(listResponse).
			AddDescribeTrainingJobResponse(missing1).
			AddDescribeTrainingJobResponse(missing2).
			Build()
		spawner = createHPOTrainingJobSpawner(k8sClient, logf.Log, sageMakerClient)

		spawner.SpawnMissingTrainingJobs(context.Background(), *createHyperParameterTuningJobWithStatus("hpo-job", namespace))

		// Verify that missing jobs were created.

		var createdTrainingJob1 trainingjobv1.TrainingJob
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      missing1Name,
		}, &createdTrainingJob1)
		Expect(err).ToNot(HaveOccurred())

		var createdTrainingJob2 trainingjobv1.TrainingJob
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      missing2Name,
		}, &createdTrainingJob2)
		Expect(err).ToNot(HaveOccurred())

		// Cleanup
		k8sClient.Delete(context.Background(), createKubernetesJob(false, present1Name, namespace))
		k8sClient.Delete(context.Background(), createKubernetesJob(false, present2Name, namespace))

		// Remove finalizers and delete the created jobs.
		createdTrainingJob1.ObjectMeta.Finalizers = []string{}
		k8sClient.Delete(context.Background(), &createdTrainingJob1)
		createdTrainingJob2.ObjectMeta.Finalizers = []string{}
		k8sClient.Delete(context.Background(), &createdTrainingJob2)
	})

	It("should fail gracefully for ListTrainingJobs errors", func() {
		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobErrorResponse("error code", 500, "req id").
			Build()

		// FailTestOnGetK8sClient is designed to fail the test when Get is called.
		spawner = createHPOTrainingJobSpawner(FailTestOnGetK8sClient{}, logf.Log, sageMakerClient)
		spawner.SpawnMissingTrainingJobs(context.Background(), *createHyperParameterTuningJobWithStatus("hpo-job", "custom-namespace"))

		// Verify that the SageMaker request was made.
		Expect(receivedRequests.Len()).To(Equal(1))
	})

	It("should fail gracefully for DescribeTrainingJob errors", func() {
		listResponse := sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput{
			NextToken: ToStringPtr(""),
			TrainingJobSummaries: []sagemaker.HyperParameterTrainingJobSummary{
				sagemaker.HyperParameterTrainingJobSummary{
					TrainingJobName: ToStringPtr("job-1"),
				},
			},
		}
		failureMessage := "error message " + uuid.New().String()
		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobResponse(listResponse).
			AddDescribeTrainingJobErrorResponse("Exception", failureMessage, 500, "request id").
			Build()

		// FailTestOnCreateK8sClient is designed to fail the test when Create is called.
		spawner = createHPOTrainingJobSpawner(FailTestOnCreateK8sClient{ActualClient: k8sClient}, logf.Log, sageMakerClient)
		spawner.SpawnMissingTrainingJobs(context.Background(), *createHyperParameterTuningJobWithStatus("hpo-job", "custom-namespace"))

		// Verify that the SageMaker requests were made.
		Expect(receivedRequests.Len()).To(Equal(2))
	})

	It("should create K8s TrainingJobs in the same region as the HPO job", func() {

		missingName := "missing-" + uuid.New().String()

		listResponse := sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput{
			NextToken: ToStringPtr(""),
			TrainingJobSummaries: []sagemaker.HyperParameterTrainingJobSummary{
				sagemaker.HyperParameterTrainingJobSummary{
					TrainingJobName: ToStringPtr(missingName),
				},
			},
		}

		// Create chuld jobs that only exist in SageMaker
		missing := createSageMakerJob(missingName)

		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobResponse(listResponse).
			AddDescribeTrainingJobResponse(missing).
			Build()
		spawner = createHPOTrainingJobSpawner(k8sClient, logf.Log, sageMakerClient)

		hpoRegion := "hpo-region"
		namespace := "namespace-1"
		hpoJob := createHyperParameterTuningJobWithStatus("hpo-job", namespace)
		hpoJob.Spec.Region = ToStringPtr(hpoRegion)

		spawner.SpawnMissingTrainingJobs(context.Background(), *hpoJob)

		// Verify that missing jobs were created.
		var createdTrainingJob trainingjobv1.TrainingJob
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      missingName,
		}, &createdTrainingJob)
		Expect(err).ToNot(HaveOccurred())

		// Verify region is correct.
		Expect(*createdTrainingJob.Spec.Region).To(Equal(hpoRegion))

		// Cleanup.
		k8sClient.Delete(context.Background(), &createdTrainingJob)
	})

	It("should create K8s TrainingJobs with the same SageMakerEndpoint as the HPO job, if specified", func() {

		missingName := "missing-" + uuid.New().String()

		listResponse := sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput{
			NextToken: ToStringPtr(""),
			TrainingJobSummaries: []sagemaker.HyperParameterTrainingJobSummary{
				sagemaker.HyperParameterTrainingJobSummary{
					TrainingJobName: ToStringPtr(missingName),
				},
			},
		}

		// Create chuld jobs that only exist in SageMaker
		missing := createSageMakerJob(missingName)

		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobResponse(listResponse).
			AddDescribeTrainingJobResponse(missing).
			Build()
		spawner = createHPOTrainingJobSpawner(k8sClient, logf.Log, sageMakerClient)

		namespace := "namespace-1"
		hpoJob := *createHyperParameterTuningJobWithStatus("hpo-job", namespace)

		sageMakerEndpoint := "https://" + uuid.New().String() + ".com"
		hpoJob.Spec.SageMakerEndpoint = &sageMakerEndpoint

		spawner.SpawnMissingTrainingJobs(context.Background(), hpoJob)

		// Verify that missing jobs were created.
		var createdTrainingJob trainingjobv1.TrainingJob
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      missingName,
		}, &createdTrainingJob)
		Expect(err).ToNot(HaveOccurred())

		// Verify region is correct.
		Expect(*createdTrainingJob.Spec.SageMakerEndpoint).To(Equal(sageMakerEndpoint))

		// Cleanup.
		k8sClient.Delete(context.Background(), &createdTrainingJob)
	})

	It("should create K8s TrainingJobs with the correct finalizer", func() {

		missingName := "missing-" + uuid.New().String()

		listResponse := sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput{
			NextToken: ToStringPtr(""),
			TrainingJobSummaries: []sagemaker.HyperParameterTrainingJobSummary{
				sagemaker.HyperParameterTrainingJobSummary{
					TrainingJobName: ToStringPtr(missingName),
				},
			},
		}

		// Create chuld jobs that only exist in SageMaker
		missing := createSageMakerJob(missingName)

		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobResponse(listResponse).
			AddDescribeTrainingJobResponse(missing).
			Build()
		spawner = createHPOTrainingJobSpawner(k8sClient, logf.Log, sageMakerClient)

		namespace := "namespace-1"
		spawner.SpawnMissingTrainingJobs(context.Background(), *createHyperParameterTuningJobWithStatus("hpo-job", namespace))

		// Verify that missing jobs were created.
		var createdTrainingJob trainingjobv1.TrainingJob
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      missingName,
		}, &createdTrainingJob)
		Expect(err).ToNot(HaveOccurred())

		// Verify finalizer is correct.
		Expect(len(createdTrainingJob.ObjectMeta.GetFinalizers())).To(Equal(1))
		Expect(createdTrainingJob.ObjectMeta.GetFinalizers()[0]).To(Equal(hpoTrainingJobOwnershipFinalizer))

		// Cleanup.
		createdTrainingJob.ObjectMeta.Finalizers = RemoveString(createdTrainingJob.ObjectMeta.GetFinalizers(), hpoTrainingJobOwnershipFinalizer)
		err = k8sClient.Update(context.Background(), &createdTrainingJob)
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      missingName,
		}, &createdTrainingJob)
		Expect(err).ToNot(HaveOccurred())

		k8sClient.Delete(context.Background(), &createdTrainingJob)
	})
})

var _ = Describe("DeleteSpawnedTrainingJobs", func() {

	var (
		sageMakerClientBuilder *MockSageMakerClientBuilder
		spawner                hpoTrainingJobSpawner
		namespace              string
		existingJobNames       []string
		listResponse           sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput
		err                    error
	)

	BeforeEach(func() {
		sageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT())

		namespace = "namespace-" + uuid.New().String()

		// Create job names
		existingJobNames = []string{}
		for i := 0; i < 2; i++ {
			existingJobNames = append(existingJobNames, "present-"+uuid.New().String())
		}

		// Create jobs to delete
		for _, name := range existingJobNames {
			err = k8sClient.Create(context.Background(), createKubernetesJob(true, name, namespace))
			Expect(err).ToNot(HaveOccurred())
		}

		// Setup SageMaker response
		listResponse = sagemaker.ListTrainingJobsForHyperParameterTuningJobOutput{
			NextToken: ToStringPtr(""),
		}
		for _, name := range existingJobNames {
			nameCopy := name
			listResponse.TrainingJobSummaries = append(listResponse.TrainingJobSummaries, sagemaker.HyperParameterTrainingJobSummary{
				TrainingJobName: &nameCopy,
			})
		}
	})

	AfterEach(func() {

		// Delete created jobs.
		for _, name := range existingJobNames {
			var createdJob trainingjobv1.TrainingJob
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &createdJob)

			if err != nil {
				continue
			}
			err = k8sClient.Delete(context.Background(), &createdJob)
			Expect(err).ToNot(HaveOccurred())
		}
	})

	It("Deletes TrainingJobs corresponding to the HPO job", func() {
		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobResponse(listResponse).
			Build()

		// Create spawner
		spawner = createHPOTrainingJobSpawner(k8sClient, logf.Log, sageMakerClient)

		// Run test
		err = spawner.DeleteSpawnedTrainingJobs(context.Background(), *createHyperParameterTuningJobWithStatus("hpo-job", namespace))

		// Verify expectations
		Expect(err).ToNot(HaveOccurred())
		for _, name := range existingJobNames {
			var deletedTrainingJob trainingjobv1.TrainingJob
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &deletedTrainingJob)
			Expect(apierrs.IsNotFound(err)).To(Equal(true))
		}
	})

	It("Removes finalizer if present even on jobs that are already being deleted", func() {

		// Mark jobs as deleted without removing finalizer.
		for _, name := range existingJobNames {
			var createdJob trainingjobv1.TrainingJob
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &createdJob)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.Background(), &createdJob)
			Expect(err).ToNot(HaveOccurred())
		}

		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobResponse(listResponse).
			Build()

		// Create spawner
		spawner = createHPOTrainingJobSpawner(k8sClient, logf.Log, sageMakerClient)

		// Run test
		err = spawner.DeleteSpawnedTrainingJobs(context.Background(), *createHyperParameterTuningJobWithStatus("hpo-job", namespace))

		// Verify expectations
		Expect(err).ToNot(HaveOccurred())
		for _, name := range existingJobNames {
			var deletedTrainingJob trainingjobv1.TrainingJob
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &deletedTrainingJob)
			Expect(apierrs.IsNotFound(err)).To(Equal(true))
		}
	})

	It("Gracefully handles TrainingJobs that were already deleted", func() {

		// Delete one job before the test.
		var jobToDelete trainingjobv1.TrainingJob
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      existingJobNames[rand.Int()%len(existingJobNames)],
		}, &jobToDelete)
		Expect(err).ToNot(HaveOccurred())
		err = k8sClient.Delete(context.Background(), &jobToDelete)
		Expect(err).ToNot(HaveOccurred())

		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobResponse(listResponse).
			Build()

		// Create spawner
		spawner = createHPOTrainingJobSpawner(k8sClient, logf.Log, sageMakerClient)

		// Run test
		err = spawner.DeleteSpawnedTrainingJobs(context.Background(), *createHyperParameterTuningJobWithStatus("hpo-job", namespace))

		// Verify expectations
		Expect(err).ToNot(HaveOccurred())
		for _, name := range existingJobNames {
			var deletedTrainingJob trainingjobv1.TrainingJob
			err = k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &deletedTrainingJob)
			Expect(apierrs.IsNotFound(err)).To(Equal(true))
		}
	})

	It("Gracefully handles ListTrainingJobsForHyperParameterTuningJob SageMaker failures", func() {
		sageMakerClient := sageMakerClientBuilder.
			AddListTrainingJobsForHyperParameterTuningJobErrorResponse("error code", 500, "req id").
			Build()

		// Create spawner
		spawner = createHPOTrainingJobSpawner(k8sClient, logf.Log, sageMakerClient)

		// Run test
		err = spawner.DeleteSpawnedTrainingJobs(context.Background(), *createHyperParameterTuningJobWithStatus("hpo-job", namespace))

		// Verify expectations
		Expect(err).To(HaveOccurred())
	})
})
