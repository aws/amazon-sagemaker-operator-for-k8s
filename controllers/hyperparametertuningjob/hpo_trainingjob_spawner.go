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
	"github.com/pkg/errors"
	"sync"

	hpojobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hyperparametertuningjob"
	trainingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/trainingjob"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/go-logr/logr"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This finalizer is added to TrainingJobs that are spawned by the HPO TrainingJob Spawner.
const hpoTrainingJobOwnershipFinalizer = "sagemaker-operator-hpo-trainingjob"

// HPOTrainingJobSpawner is a simple utility for creating and deleting Kubernetes TrainingJobs
// that track SageMaker TrainingJobs that were started by a given HPO job.
type HPOTrainingJobSpawner interface {
	// Spawn TrainingJobs associated with the given HPO job that do not already exist in Kubernetes.
	SpawnMissingTrainingJobs(ctx context.Context, hpoJob hpojobv1.HyperparameterTuningJob)

	// Delete TrainingJobs in Kuberentes that are associated with the given HPO job.
	DeleteSpawnedTrainingJobs(ctx context.Context, hpoJob hpojobv1.HyperparameterTuningJob) error
}

// NewHPOTrainingJobSpawner constructs a new HPOTrainingJobSpawner.
func NewHPOTrainingJobSpawner(k8sClient client.Client, log logr.Logger, sageMakerClient clientwrapper.SageMakerClientWrapper) HPOTrainingJobSpawner {
	spawner := hpoTrainingJobSpawner{
		K8sClient:       k8sClient,
		Log:             log.WithName("HPOTrainingJobSpawner"),
		SageMakerClient: sageMakerClient,
	}
	return &spawner
}

// HPOTrainingJobSpawnerProvider constructs an HPO Training Job Spawner
type HPOTrainingJobSpawnerProvider func(k8sClient client.Client, log logr.Logger, sageMakerClient clientwrapper.SageMakerClientWrapper) HPOTrainingJobSpawner

type hpoTrainingJobSpawner struct {
	HPOTrainingJobSpawner

	K8sClient       client.Client
	Log             logr.Logger
	SageMakerClient clientwrapper.SageMakerClientWrapper
}

// For a given region and HPO job name, get the list of TrainingJobs that the HPO job created. Then, for every
// TrainingJob that does not exist in Kubernetes, create a Kubernetes TrainingJob with the exact same spec as
// the SageMaker TrainingJob. This will allow the TrainingJob operator to track the SageMaker TrainingJobs.
// Note that the TrainingJobs corresponding to the HPO job are created in the same Kubernetes namespace as the HPO job.
func (s hpoTrainingJobSpawner) SpawnMissingTrainingJobs(ctx context.Context, hpoJob hpojobv1.HyperparameterTuningJob) {

	hpoJobName := hpoJob.Spec.HyperParameterTuningJobName
	k8sNamespace := hpoJob.ObjectMeta.GetNamespace()
	awsRegion := *hpoJob.Spec.Region
	sageMakerEndpoint := hpoJob.Spec.SageMakerEndpoint

	paginator := s.SageMakerClient.ListTrainingJobsForHyperParameterTuningJob(ctx, *hpoJobName)

	// WaitGroup allowing us to do checks in parallel.
	var wg sync.WaitGroup

	for paginator.Next(ctx) {
		list := paginator.CurrentPage()
		s.Log.Info("Got a page of TrainingJobs to spawn", "length", len(list))

		// For every training job, check if it exists in Kubernetes. If not, create it.
		for _, trainingJob := range list {

			// Note that we have to wait for the goroutine.
			wg.Add(1)

			// Spawn goroutine that will check for the TrainingJob's existence and create it in Kubernetes if it does not exist.
			go func(trainingJob sagemaker.HyperParameterTrainingJobSummary) {
				defer wg.Done()

				// If job already exists in Kubernetes, or we are unable to tell, do not attempt to spawn the training job.
				if exists, err := s.trainingJobExistsInKubernetes(ctx, *trainingJob.TrainingJobName, k8sNamespace); exists || (err != nil) {
					return
				}

				err := s.spawnTrainingJobInKubernetes(ctx, awsRegion, sageMakerEndpoint, *trainingJob.TrainingJobName, k8sNamespace)

				if err != nil {
					s.Log.Info("Unable to spawn missing training job", "err", err)
				}
			}(trainingJob)
		}
	}

	if err := paginator.Err(); err != nil {
		s.Log.Info("Error while getting training jobs", "err", err)
	}

	// Wait for all requests to finish.
	wg.Wait()

}

// Check if a given TrainingJob exists in Kubernetes. Returns whether or not the job exists and whether or not
// any error happened in the attempt to determine if the job exists in Kubernetes.
func (s hpoTrainingJobSpawner) trainingJobExistsInKubernetes(ctx context.Context, trainingJobName, k8sNamespace string) (bool, error) {

	// Attempt to Get the TrainingJob
	var trainingJob trainingjobv1.TrainingJob
	err := s.K8sClient.Get(ctx, types.NamespacedName{
		Namespace: k8sNamespace,
		Name:      trainingJobName,
	}, &trainingJob)

	// A job by the exact same name already exists in Kubernetes, we will assume that it is the
	// same TrainingJob and not attempt to spawn it.
	if err == nil {
		return true, nil
	}

	// The err is non-nil and something other than a NotFound error.
	if !apierrs.IsNotFound(err) {
		s.Log.Info("Unable to check if TrainingJob spawn needed because k8sClient.Get failed", "err", err)
		return false, err
	}

	// The job does not exist in Kubernetes. We should attempt
	// to spawn the TrainingJob in Kubernetes.
	return false, nil
}

// Create the specified TrainingJob in Kubernetes.
func (s hpoTrainingJobSpawner) spawnTrainingJobInKubernetes(ctx context.Context, awsRegion string, sageMakerEndpoint *string, trainingJobName, k8sNamespace string) error {

	trainingJobSpec, err := s.getKubernetesTrainingJobSpec(ctx, trainingJobName)

	if err != nil {
		return errors.Wrap(err, "Unable to create job spec")
	}

	// Add fields that are not present in DescribeTrainingJob output.
	trainingJobSpec.Region = &awsRegion
	trainingJobSpec.SageMakerEndpoint = sageMakerEndpoint

	// Attempt to create the TrainingJob in the given namespace, with the HPO finalizer.
	trainingJob := &trainingjobv1.TrainingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:       trainingJobName,
			Namespace:  k8sNamespace,
			Finalizers: []string{hpoTrainingJobOwnershipFinalizer},
		},
		Spec: *trainingJobSpec,
	}

	if err := s.K8sClient.Create(ctx, trainingJob); err != nil {
		return errors.Wrap(err, "Unable to create k8s job")
	}

	s.Log.Info("Successfully spawned TrainingJob", "trainingJobName", trainingJobName)

	return nil
}

// Given a TrainingJob, create a Kubernetes spec for a TrainingJob.
// This works by Describing the SageMaker TrainingJob, then using that description to create a Kubernetes spec.
func (s hpoTrainingJobSpawner) getKubernetesTrainingJobSpec(ctx context.Context, trainingJobName string) (*trainingjobv1.TrainingJobSpec, error) {
	response, err := s.SageMakerClient.DescribeTrainingJob(ctx, trainingJobName)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to get TrainingJob description from SageMaker")
	}

	spec, err := sdkutil.CreateTrainingJobSpecFromDescription(*response)
	return &spec, err
}

// Delete TrainingJobs associated with the given HPO job.
// This returns an error if at least one job could not be deleted, indicating to the caller that a retry is needed.
func (s hpoTrainingJobSpawner) DeleteSpawnedTrainingJobs(ctx context.Context, hpoJob hpojobv1.HyperparameterTuningJob) error {

	hpoJobName := hpoJob.Status.SageMakerHyperParameterTuningJobName
	k8sNamespace := hpoJob.ObjectMeta.GetNamespace()

	errors := s.deleteSpawnedTrainingJobsConcurrently(ctx, hpoJobName, k8sNamespace)

	// If any error occurred, return an error to signal that we need to retry.
	if len(errors) > 0 {
		return fmt.Errorf("Error(s) occurred while deleting spawned training jobs: %+v", errors)
	}

	return nil
}

// Spawn goroutines that each delete one TrainingJob.
// This waits for the goroutines and collects any errors that prevented TrainingJobs from being deleted.
// The errors are returned via a slice.
func (s hpoTrainingJobSpawner) deleteSpawnedTrainingJobsConcurrently(ctx context.Context, hpoJobName, k8sNamespace string) []error {

	// Create a WaitGroup so that we can await all goroutines.
	var wg sync.WaitGroup

	// Create a channel so that we can collect errors encounterd by goroutines.
	// The goroutines will block on writes; this goroutine will consume concurrently
	// so that they finish.
	errorsChannel := make(chan error)

	// Create paginated request to get TrainingJobs associated with HPO job.
	paginator := s.SageMakerClient.ListTrainingJobsForHyperParameterTuningJob(ctx, hpoJobName)

	// For every TrainingJob, spawn a goroutine that deletes the k8s job if it exists.
	for paginator.Next(ctx) {
		list := paginator.CurrentPage()
		for _, trainingJobSummary := range list {
			wg.Add(1)
			go func(trainingJobSummary sagemaker.HyperParameterTrainingJobSummary) {
				defer wg.Done()

				key := types.NamespacedName{
					Namespace: k8sNamespace,
					Name:      *trainingJobSummary.TrainingJobName,
				}

				var trainingJob trainingjobv1.TrainingJob
				if err := s.K8sClient.Get(ctx, key, &trainingJob); err != nil {
					// If the job has previously been deleted then we don't need to do
					return
				}

				if err := s.deleteSpawnedTrainingJob(ctx, &trainingJob); err != nil {
					errorsChannel <- err
				}
			}(trainingJobSummary)
		}
	}

	// If the ListTrainingJobs operation failed, store the error in the channel.
	// This is done in a goroutine to prevent the consumer goroutine from being
	// blocked before it consumes from the channel.
	wg.Add(1)
	go func(paginatorError error) {
		defer wg.Done()
		if paginatorError != nil {
			s.Log.Info("Error while getting training jobs", "err", paginatorError)
			errorsChannel <- paginatorError
		}
	}(paginator.Err())

	// Spawn a goroutine that concurrently waits for all of the worker goroutines, then closes the errors channel.
	go func() {
		defer close(errorsChannel)
		wg.Wait()
	}()

	// Concurrent to the deletion goroutines, read every error from the channel and
	// save them in a slice.
	errors := []error{}
	for err := range errorsChannel {
		errors = append(errors, err)
	}

	return errors
}

// Delete a single training job and remove its finalizer, if present.
// Returns an error if any operation failed and needs to be retried.
func (s hpoTrainingJobSpawner) deleteSpawnedTrainingJob(ctx context.Context, trainingJob *trainingjobv1.TrainingJob) error {
	needsRemoveFinalizer := controllers.ContainsString(trainingJob.ObjectMeta.GetFinalizers(), hpoTrainingJobOwnershipFinalizer)
	needsDelete := trainingJob.ObjectMeta.GetDeletionTimestamp().IsZero()

	if needsRemoveFinalizer {
		s.Log.Info("Removing HPO ownership finalizer from TrainingJob", "trainingJobName", trainingJob.Status.SageMakerTrainingJobName)
		trainingJob.ObjectMeta.Finalizers = controllers.RemoveString(trainingJob.ObjectMeta.GetFinalizers(), hpoTrainingJobOwnershipFinalizer)
		if err := s.K8sClient.Update(ctx, trainingJob); err != nil {
			return errors.Wrap(err, "Failed to remove finalizer")
		}
	}

	if needsDelete {
		s.Log.Info("Deleting TrainingJob", "trainingJobName", trainingJob.Status.SageMakerTrainingJobName)
		if err := s.K8sClient.Delete(ctx, trainingJob); err != nil {
			return errors.Wrap(err, "Failed to delete training job")
		}
	}

	return nil
}
