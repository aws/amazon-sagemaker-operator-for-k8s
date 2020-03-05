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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hpojobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hyperparametertuningjob"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
)

// Status used when no SageMaker status is available
const (
	ReconcilingTuningJobStatus = "ReconcilingTuningJob"
)

// Defines the maximum number of characters in a SageMaker HyperParameter Job name
const (
	MaxHyperParameterTuningJobNameLength = 32
)

// Reconciler reconciles a HyperparameterTuningJob object
type Reconciler struct {
	client.Client
	Log          logr.Logger
	PollInterval time.Duration

	createSageMakerClient       clientwrapper.SageMakerClientWrapperProvider
	awsConfigLoader             controllers.AwsConfigLoader
	createHPOTrainingJobSpawner HPOTrainingJobSpawnerProvider
}

// NewHyperparameterTuningJobReconciler creates a new reconciler with the default SageMaker client.
func NewHyperparameterTuningJobReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createSageMakerClient: func(cfg aws.Config) clientwrapper.SageMakerClientWrapper {
			return clientwrapper.NewSageMakerClientWrapper(sagemaker.New(cfg))
		},
		createHPOTrainingJobSpawner: NewHPOTrainingJobSpawner,
		awsConfigLoader:             controllers.NewAwsConfigLoader(),
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hyperparametertuningjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hyperparametertuningjobs/status,verbs=get;update;patch

// Reconcile attempts to reconcile the SageMaker resource state with the k8s desired state.
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context:   context.Background(),
		Log:       r.Log.WithValues("hyperparametertuningjob", req.NamespacedName),
		TuningJob: new(hpojobv1.HyperparameterTuningJob),
	}

	ctx.Log.Info("Getting resource")

	if err := r.Get(ctx, req.NamespacedName, ctx.TuningJob); err != nil {
		ctx.Log.Info("Unable to fetch TuningJob job", "reason", err)

		if apierrs.IsNotFound(err) {
			return controllers.NoRequeue()
		}

		return controllers.RequeueImmediately()
	}

	if err := r.reconcileTuningJob(ctx); err != nil {
		ctx.Log.Info("Got error while reconciling, will retry", "err", err)
		return controllers.RequeueImmediately()
	}
	ctx.Log.Info("TuningJob", "name", ctx.TuningJobName)

	switch ctx.TuningJob.Status.HyperParameterTuningJobStatus {
	case string(sagemaker.HyperParameterTuningJobStatusCompleted):
		fallthrough
	case string(sagemaker.HyperParameterTuningJobStatusFailed):
		fallthrough
	case string(sagemaker.HyperParameterTuningJobStatusStopped):
		return controllers.NoRequeue()
	default:
		return controllers.RequeueAfterInterval(r.PollInterval, nil)
	}
}

type reconcileRequestContext struct {
	context.Context

	Log             logr.Logger
	SageMakerClient clientwrapper.SageMakerClientWrapper

	// The desired state of the HyperParameterTuningJob.
	TuningJob *hpojobv1.HyperparameterTuningJob

	// The SageMaker HyperParameterTuningJob description.
	TuningJobDescription *sagemaker.DescribeHyperParameterTuningJobOutput

	// The name of the SageMaker HyperParameterTuningJob.
	TuningJobName string

	// Responsible for creating child k8s TrainingJob resources.
	HPOTrainingJobSpawner HPOTrainingJobSpawner
}

// Queries SageMaker for the HyperParamaterTuningJob. If it does not exist, it creates the
// Tuning job. If it does exist, it attempts to update the status of the k8s resource
// to match the current state of the SageMaker resource.
func (r *Reconciler) reconcileTuningJob(ctx reconcileRequestContext) error {
	var err error

	if ctx.TuningJob.Status.HyperParameterTuningJobStatus == "" {
		if err = r.updateStatus(ctx, controllers.InitializingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, string(sagemaker.HyperParameterTuningJobStatusFailed), errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if it's not marked for deletion.
	if !controllers.HasDeletionTimestamp(ctx.TuningJob.ObjectMeta) {
		if !controllers.ContainsString(ctx.TuningJob.ObjectMeta.GetFinalizers(), controllers.SageMakerResourceFinalizerName) {
			ctx.TuningJob.ObjectMeta.Finalizers = append(ctx.TuningJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.TuningJob); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer added")
		}
	}

	// Get the HyperParameterTuningJob from SageMaker
	if ctx.TuningJobDescription, err = ctx.SageMakerClient.DescribeHyperParameterTuningJob(ctx, ctx.TuningJobName); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to describe SageMaker hyperparameter tuning job"))
	}

	// The resource does not exist within SageMaker yet.
	if ctx.TuningJobDescription == nil {
		if controllers.HasDeletionTimestamp(ctx.TuningJob.ObjectMeta) {
			// Don't attempt to clean up resources as none should exist yet
			return r.removeFinalizer(ctx)
		}

		if err = r.createHyperParameterTuningJob(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to create hyperparameter tuning job"))
		}

		if ctx.TuningJobDescription, err = ctx.SageMakerClient.DescribeHyperParameterTuningJob(ctx, ctx.TuningJobName); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to describe SageMaker hyperparameter tuning job"))
		}
	}

	// Spawn training jobs regardless of the status
	ctx.HPOTrainingJobSpawner.SpawnMissingTrainingJobs(ctx, *ctx.TuningJob)
	if err = r.addBestTrainingJobToStatus(ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to add best training job to status"))
	}

	switch ctx.TuningJobDescription.HyperParameterTuningJobStatus {
	case sagemaker.HyperParameterTuningJobStatusInProgress:
		if controllers.HasDeletionTimestamp(ctx.TuningJob.ObjectMeta) {
			// Request to stop the job. If SageMaker returns a 404 then the job has already been deleted.
			if _, err := ctx.SageMakerClient.StopHyperParameterTuningJob(ctx, ctx.TuningJobName); err != nil && !clientwrapper.IsStopHyperParameterTuningJob404Error(err) {
				return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to delete hyperparameter tuning job"))
			}
			// Describe the new state of the job
			if ctx.TuningJobDescription, err = ctx.SageMakerClient.DescribeHyperParameterTuningJob(ctx, ctx.TuningJobName); err != nil {
				return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to describe SageMaker hyperparameter tuning job"))
			}
		}

	case sagemaker.HyperParameterTuningJobStatusStopped, sagemaker.HyperParameterTuningJobStatusFailed, sagemaker.HyperParameterTuningJobStatusCompleted:
		if controllers.HasDeletionTimestamp(ctx.TuningJob.ObjectMeta) {
			return r.cleanupAndRemoveFinalizer(ctx)
		}

	case sagemaker.HyperParameterTuningJobStatusStopping:
		break

	default:
		return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, fmt.Errorf("Unknown Tuning Job Status: %s", ctx.TuningJobDescription.HyperParameterTuningJobStatus))
	}

	status := string(ctx.TuningJobDescription.HyperParameterTuningJobStatus)
	additional := controllers.GetOrDefault(ctx.TuningJobDescription.FailureReason, "")

	if err = r.updateStatusWithAdditional(ctx, status, additional); err != nil {
		return err
	}

	return nil
}

// Initialize fields on the context object which will be used later.
func (r *Reconciler) initializeContext(ctx *reconcileRequestContext) error {
	// Ensure we generate a new name and populate the spec if none was specified
	if ctx.TuningJob.Spec.HyperParameterTuningJobName == nil || len(*ctx.TuningJob.Spec.HyperParameterTuningJobName) == 0 {
		generatedName := controllers.GetGeneratedJobName(ctx.TuningJob.ObjectMeta.GetUID(), ctx.TuningJob.ObjectMeta.GetName(), MaxHyperParameterTuningJobNameLength)
		ctx.TuningJob.Spec.HyperParameterTuningJobName = &generatedName

		if err := r.Update(ctx, ctx.TuningJob); err != nil {
			ctx.Log.Info("Error while updating hyperparameter tuning job name in spec")
			return err
		}
	}
	ctx.TuningJobName = *ctx.TuningJob.Spec.HyperParameterTuningJobName
	ctx.Log.Info("TuningJob", "name", ctx.TuningJobName)

	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.TuningJob.Spec.Region, ctx.TuningJob.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.SageMakerClient = r.createSageMakerClient(awsConfig)
	ctx.Log.Info("Loaded AWS config")

	ctx.HPOTrainingJobSpawner = r.createHPOTrainingJobSpawner(r, ctx.Log, ctx.SageMakerClient)

	return nil
}

// Creates the hyperparameter tuning job in SageMaker
func (r *Reconciler) createHyperParameterTuningJob(ctx reconcileRequestContext) error {
	createTuningJobInput, err := sdkutil.CreateCreateHyperParameterTuningJobInputFromSpec(ctx.TuningJob.Spec)

	if err != nil {
		return errors.Wrap(err, "Unable to create a CreateHyperParameterTuningJobInput from spec")
	}

	ctx.Log.Info("Creating TuningJob in SageMaker", "input", createTuningJobInput)

	if _, err := ctx.SageMakerClient.CreateHyperParameterTuningJob(ctx, &createTuningJobInput); err != nil {
		return errors.Wrap(err, "Unable to create HyperParameter Tuning Job")
	}

	return nil
}

// Clean up all spawned training jobs, then remove the HyperParameterTuningJob finalizer.
func (r *Reconciler) cleanupAndRemoveFinalizer(ctx reconcileRequestContext) error {
	var err error

	if err = ctx.HPOTrainingJobSpawner.DeleteSpawnedTrainingJobs(ctx, *ctx.TuningJob); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Not all associated TrainingJobs jobs were deleted"))
	}

	return r.removeFinalizer(ctx)
}

// Removes the operator finalizer from the HyperParameterTuningJob.
func (r *Reconciler) removeFinalizer(ctx reconcileRequestContext) error {
	var err error

	ctx.TuningJob.ObjectMeta.Finalizers = controllers.RemoveString(ctx.TuningJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
	if err = r.Update(ctx, ctx.TuningJob); err != nil {
		return errors.Wrap(err, "Failed to remove finalizer")
	}
	ctx.Log.Info("Finalizer has been removed")

	return nil
}

// Add information regarding the best training job from the tuning job to the status inputs.
func (r *Reconciler) addBestTrainingJobToStatus(ctx reconcileRequestContext) error {
	if ctx.TuningJobDescription.BestTrainingJob == nil {
		// Best training job information is not available yet.
		ctx.Log.Info("BestTrainingJob was not specified in HPO description")
		return nil
	}

	bestTrainingJob, err := sdkutil.ConvertHyperParameterTrainingJobSummaryFromSageMaker(ctx.TuningJobDescription.BestTrainingJob)

	if err != nil {
		return err
	}

	ctx.TuningJob.Status.BestTrainingJob = bestTrainingJob

	return nil
}

// Update the status and other informational fields.
// Returns an error if there was a failure to update.
func (r *Reconciler) updateStatus(ctx reconcileRequestContext, tuningJobStatus string) error {
	return r.updateStatusWithAdditional(ctx, tuningJobStatus, "")
}

// Helper method to update the status with the error message and status. If there was an error updating the status, return
// that error instead.
func (r *Reconciler) updateStatusAndReturnError(ctx reconcileRequestContext, tuningJobStatus string, reconcileErr error) error {
	if err := r.updateStatusWithAdditional(ctx, tuningJobStatus, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

// Update the status and other informational fields. The "additional" parameter should be used to convey additional error information. Leave empty to omit.
// Returns an error if there was a failure to update.
func (r *Reconciler) updateStatusWithAdditional(ctx reconcileRequestContext, tuningJobStatus, additional string) error {
	ctx.Log.Info("updateStatusWithAdditional", "tuningJobStatus", tuningJobStatus, "additional", additional)

	jobStatus := &ctx.TuningJob.Status
	// When you call this function, update/refresh all the fields since we overwrite.
	jobStatus.HyperParameterTuningJobStatus = tuningJobStatus
	jobStatus.Additional = additional

	jobStatus.SageMakerHyperParameterTuningJobName = ctx.TuningJobName
	jobStatus.TrainingJobStatusCounters = sdkutil.CreateTrainingJobStatusCountersFromDescription(ctx.TuningJobDescription)

	if err := r.Status().Update(ctx, ctx.TuningJob); err != nil {
		err = errors.Wrap(err, "Unable to update status")
		ctx.Log.Info("Error while updating status.", "err", err)
		return err
	}

	return nil
}

// SetupWithManager configures the manager to recognise the controller.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&hpojobv1.HyperparameterTuningJob{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
