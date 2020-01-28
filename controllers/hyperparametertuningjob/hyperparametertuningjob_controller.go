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
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	hpojobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hyperparametertuningjob"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
)

// All the status used by the controller during reconciliation.
const (
	ReconcilingTuningJobStatus = "ReconcilingTuningJob"
)

// SageMaker API returns this API error code with HTTP status code 400 when
// a DescribeHyperparameterTuningJob does not find the specified job.
const hpoResourceNotFoundApiCode string = "ResourceNotFound"

// Reconciler reconciles a HyperparameterTuningJob object
type Reconciler struct {
	client.Client
	Log          logr.Logger
	PollInterval time.Duration

	createSageMakerClient       clientwrapper.SageMakerClientWrapperProvider
	awsConfigLoader             controllers.AwsConfigLoader
	createHpoTrainingJobSpawner HpoTrainingJobSpawnerProvider
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
		createHpoTrainingJobSpawner: NewHpoTrainingJobSpawner,
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
	HpoTrainingJobSpawner HpoTrainingJobSpawner
}

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
			return r.cleanupAndRemoveFinalizer(ctx)
		}

		if err = r.createHyperParameterTuningJob(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to create hyperparameter tuning job"))
		}

		if ctx.TuningJobDescription, err = ctx.SageMakerClient.DescribeHyperParameterTuningJob(ctx, ctx.TuningJobName); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to describe SageMaker hyperparameter tuning job"))
		}
	}

	switch ctx.TuningJobDescription.HyperParameterTuningJobStatus {
	case sagemaker.HyperParameterTuningJobStatusInProgress:
		if controllers.HasDeletionTimestamp(ctx.TuningJob.ObjectMeta) {
			// Request to stop the job
			if _, err := ctx.SageMakerClient.StopHyperParameterTuningJob(ctx, ctx.TuningJobName); err != nil && !clientwrapper.IsStopJob404Error(err) {
				return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to delete hyperparameter tuning job"))
			}
			// Describe the new state of the job
			if ctx.TuningJobDescription, err = ctx.SageMakerClient.DescribeHyperParameterTuningJob(ctx, ctx.TuningJobName); err != nil {
				return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to describe SageMaker hyperparameter tuning job"))
			}
		}
		break

	case sagemaker.HyperParameterTuningJobStatusCompleted:
		if err = r.addModelPathToStatus(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, errors.Wrap(err, "Unable to add model path to status"))
		}
		fallthrough

	case sagemaker.HyperParameterTuningJobStatusStopped, sagemaker.HyperParameterTuningJobStatusFailed:
		if controllers.HasDeletionTimestamp(ctx.TuningJob.ObjectMeta) {
			return r.removeFinalizer(ctx)
		}
		break

	case sagemaker.HyperParameterTuningJobStatusStopping:
		break

	default:
		unknownStateError := errors.New(fmt.Sprintf("Unknown Tuning Job Status: %s", ctx.TuningJobDescription.HyperParameterTuningJobStatus))
		return r.updateStatusAndReturnError(ctx, ReconcilingTuningJobStatus, unknownStateError)
	}

	if err = r.updateStatus(ctx, string(ctx.TuningJobDescription.HyperParameterTuningJobStatus)); err != nil {
		return err
	}

	return nil
}

// Initialize fields on the context object which will be used later.
func (r *Reconciler) initializeContext(ctx *reconcileRequestContext) error {
	// Ensure we are using the job name specified in the spec
	if ctx.TuningJob.Spec.HyperParameterTuningJobName != nil {
		ctx.TuningJobName = *ctx.TuningJob.Spec.HyperParameterTuningJobName
	} else {
		ctx.TuningJobName = controllers.GetGeneratedJobName(ctx.TuningJob.ObjectMeta.GetUID(), ctx.TuningJob.ObjectMeta.GetName(), 32)
	}
	ctx.Log.Info("TuningJob", "name", ctx.TuningJobName)

	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.TuningJob.Spec.Region, ctx.TuningJob.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.SageMakerClient = r.createSageMakerClient(awsConfig)
	ctx.Log.Info("Loaded AWS config")

	ctx.HpoTrainingJobSpawner = r.createHpoTrainingJobSpawner(r, ctx.Log, ctx.SageMakerClient)

	return nil
}

// Creates the hyperparameter tuning job in SageMaker
func (r *Reconciler) createHyperParameterTuningJob(ctx reconcileRequestContext) error {
	var createTuningJobInput sagemaker.CreateHyperParameterTuningJobInput

	if ctx.TuningJob.Spec.HyperParameterTuningJobName == nil || len(*ctx.TuningJob.Spec.HyperParameterTuningJobName) == 0 {
		ctx.TuningJob.Spec.HyperParameterTuningJobName = &ctx.TuningJobName
	}

	createTuningJobInput = sdkutil.CreateCreateHyperParameterTuningJobInputFromSpec(ctx.TuningJob.Spec)

	ctx.Log.Info("Creating TuningJob in SageMaker", "input", createTuningJobInput)

	if _, err := ctx.SageMakerClient.CreateHyperParameterTuningJob(ctx, &createTuningJobInput); err != nil {
		return errors.Wrap(err, "Unable to create HyperParameter Tuning Job")
	}

	return nil
}

// Clean up all models and endpoints, then remove the HostingDeployment finalizer.
func (r *Reconciler) cleanupAndRemoveFinalizer(ctx reconcileRequestContext) error {
	var err error
	if err = r.reconcileEndpointResources(ctx, true); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingEndpointStatus, errors.Wrap(err, "Unable to clean up HostingDeployment"))
	}

	if !ctx.Deployment.ObjectMeta.GetDeletionTimestamp().IsZero() {
		ctx.Deployment.ObjectMeta.Finalizers = RemoveString(ctx.Deployment.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
		if err = r.Update(ctx, ctx.Deployment); err != nil {
			return errors.Wrap(err, "Failed to remove finalizer")
		}
		ctx.Log.Info("Finalizer has been removed")
	}

	return nil
}

// Remove the finalizer and update etcd
func (r *Reconciler) removeFinalizerAndUpdate(ctx reconcileRequestContext) (ctrl.Result, error) {
	log := ctx.Log.WithName("removeFinalizerAndUpdate")
	ctx.Job.ObjectMeta.Finalizers = RemoveString(ctx.Job.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
	err := r.Update(ctx, &ctx.Job)
	if err != nil {
		log.Info("Failed to remove finalizer", "error", err)
	} else {
		log.Info("Finalizer has been removed from the job")
	}
	return RequeueIfError(err)
}

// If job is running, stop it and requeue
// If job is stopping, update status and requeue
// If job has finished, failed or stopped remove finalizer and don't requeue
func (r *Reconciler) deleteHyperparameterTuningJobIfFinalizerExists(ctx reconcileRequestContext) (ctrl.Result, error) {
	log := ctx.Log.WithName("deleteHyperparameterTuningJobIfFinalizerExists")

	if !ContainsString(ctx.Job.ObjectMeta.Finalizers, SageMakerResourceFinalizerName) {
		log.Info("Object does not have finalizer nothing to do!!!")
		return NoRequeue()
	}

	log.Info("Object has been scheduled for deletion")
	switch ctx.SageMakerDescription.HyperParameterTuningJobStatus {
	case sagemaker.HyperParameterTuningJobStatusInProgress:
		log.Info("Job is in_progress and has finalizer, so we need to delete it")
		req := ctx.SageMakerClient.StopHyperParameterTuningJobRequest(&sagemaker.StopHyperParameterTuningJobInput{
			HyperParameterTuningJobName: ctx.Job.Spec.HyperParameterTuningJobName,
		})
		_, err := req.Send(ctx)
		if err != nil {
			log.Error(err, "Unable to stop the job in sagemaker", "context", ctx)
			return r.handleSageMakerApiFailure(ctx, err, false)
		}

		return RequeueImmediately()

	case sagemaker.HyperParameterTuningJobStatusStopping:

		log.Info("Job is stopping, nothing to do")
		if err := r.updateJobStatus(ctx, hpojobv1.HyperparameterTuningJobStatus{
			HyperParameterTuningJobStatus:        string(ctx.SageMakerDescription.HyperParameterTuningJobStatus),
			LastCheckTime:                        Now(),
			SageMakerHyperParameterTuningJobName: *ctx.Job.Spec.HyperParameterTuningJobName,
			TrainingJobStatusCounters:            newTrainingJobStatusCountersFromDescription(ctx.SageMakerDescription),
		}); err != nil {
			return RequeueIfError(err)
		}
		return RequeueAfterInterval(r.PollInterval, nil)
	case sagemaker.HyperParameterTuningJobStatusCompleted, sagemaker.HyperParameterTuningJobStatusFailed, sagemaker.HyperParameterTuningJobStatusStopped:
		log.Info("Job is in terminal state. Deleting spawned TrainingJobs and removing finalizer.")

		// Delete all spawned TrainingJobs, retry if failure.
		err := ctx.HpoTrainingJobSpawner.DeleteSpawnedTrainingJobs(ctx, ctx.Job)
		if err != nil {
			log.Info("Not all associated TrainingJobs jobs were deleted, will retry", "error", err)
			return RequeueAfterInterval(r.PollInterval, nil)
		}

		return r.removeFinalizerAndUpdate(ctx)
	default:
		log.Info("Job is in unknown status")
		return NoRequeue()
	}
}

func (r *Reconciler) addFinalizerAndRequeue(ctx reconcileRequestContext) (ctrl.Result, error) {
	ctx.Job.ObjectMeta.Finalizers = append(ctx.Job.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
	ctx.Log.Info("Add finalizer and Requeue")
	prevGeneration := ctx.Job.ObjectMeta.GetGeneration()
	if err := r.Update(ctx, &ctx.Job); err != nil {
		ctx.Log.Error(err, "Failed to add finalizer", "StatusUpdateError", ctx)
		return RequeueIfError(err)
	}

	return RequeueImmediatelyUnlessGenerationChanged(prevGeneration, ctx.Job.ObjectMeta.GetGeneration())
}

func (r *Reconciler) getSageMakerDescription(ctx reconcileRequestContext) (*sagemaker.DescribeHyperParameterTuningJobOutput, awserr.RequestFailure) {
	describeRequest := ctx.SageMakerClient.DescribeHyperParameterTuningJobRequest(&sagemaker.DescribeHyperParameterTuningJobInput{
		HyperParameterTuningJobName: ctx.Job.Spec.HyperParameterTuningJobName,
	})

	describeResponse, describeError := describeRequest.Send(ctx)
	log := ctx.Log.WithName("getSageMakerDescription")

	if awsErr, requestFailed := describeError.(awserr.RequestFailure); requestFailed {
		if r.isSageMaker404Response(awsErr) {
			log.Info("Job does not exist in sagemaker")
			return nil, nil
		} else {
			log.Info("Non-404 error response from DescribeHyperparameterTuningJob")
			return nil, awsErr
		}
	} else if describeError != nil {
		// TODO: Add unit test for this
		log.Info("Failed to parse the describe error output from sagemaker")
		return nil, awsErr
	}
	return describeResponse.DescribeHyperParameterTuningJobOutput, nil
}

// Extract the BestTrainingJob from the SageMaker description if it exists and convert it to our Kubernetes type.
func (r *Reconciler) createBestTrainingJob(ctx reconcileRequestContext) *commonv1.HyperParameterTrainingJobSummary {

	if ctx.TuningJobDescription.BestTrainingJob == nil {
		ctx.Log.Info("Cannot get BestTrainingJob: No BestTrainingJob in HPO description")
		return nil
	}

	bestTrainingJob, err := convertHyperParameterTrainingJobSummaryFromSageMaker(ctx.TuningJobDescription.BestTrainingJob)

	if err != nil {
		ctx.Log.Info("Unable to create TrainingJobSummary for BestTrainingJob.", "err", err)
		return nil
	}

	return bestTrainingJob
}

// Convert an aws-go-sdk-v2 HyperParameterTrainingJobSummary to a Kubernetes SageMaker type, returning errors if there are any.
func convertHyperParameterTrainingJobSummaryFromSageMaker(source *sagemaker.HyperParameterTrainingJobSummary) (*commonv1.HyperParameterTrainingJobSummary, error) {
	var target commonv1.HyperParameterTrainingJobSummary

	// Kubebuilder does not support arbitrary maps, so we encode these as KeyValuePairs.
	// After the JSON conversion, we will re-set the KeyValuePairs as map elements.
	var tunedHyperParameters []*commonv1.KeyValuePair = []*commonv1.KeyValuePair{}

	for name, value := range source.TunedHyperParameters {
		tunedHyperParameters = append(tunedHyperParameters, &commonv1.KeyValuePair{
			Name:  name,
			Value: value,
		})
	}

	// TODO we should consider an alternative approach, see comments in TrainingController.
	str, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(str, &target)

	target.TunedHyperParameters = tunedHyperParameters
	return &target, nil
}

func (r *Reconciler) updateStatus(ctx reconcileRequestContext, tuningJobStatus string) error {
	return r.updateStatusWithAdditional(ctx, tuningJobStatus, "")
}

func (r *Reconciler) updateStatusAndReturnError(ctx reconcileRequestContext, tuningJobStatus string, reconcileErr error) error {
	if err := r.updateStatusWithAdditional(ctx, tuningJobStatus, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

func (r *Reconciler) updateStatusWithAdditional(ctx reconcileRequestContext, tuningJobStatus, additional string) error {
	ctx.Log.Info("updateStatusWithAdditional", "tuningJobStatus", tuningJobStatus, "additional", additional)

	jobStatus := &ctx.TuningJob.Status
	// When you call this function, update/refresh all the fields since we overwrite.
	jobStatus.HyperParameterTuningJobStatus = tuningJobStatus
	jobStatus.Additional = additional

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

func newTrainingJobStatusCountersFromDescription(sageMakerDescription *sagemaker.DescribeHyperParameterTuningJobOutput) *commonv1.TrainingJobStatusCounters {
	if sageMakerDescription != nil && sageMakerDescription.TrainingJobStatusCounters != nil {
		var totalError *int64 = nil

		if sageMakerDescription.TrainingJobStatusCounters.NonRetryableError != nil && sageMakerDescription.TrainingJobStatusCounters.RetryableError != nil {
			totalErrorVal := *sageMakerDescription.TrainingJobStatusCounters.NonRetryableError + *sageMakerDescription.TrainingJobStatusCounters.RetryableError
			totalError = &totalErrorVal
		}

		return &commonv1.TrainingJobStatusCounters{
			Completed:         sageMakerDescription.TrainingJobStatusCounters.Completed,
			InProgress:        sageMakerDescription.TrainingJobStatusCounters.InProgress,
			NonRetryableError: sageMakerDescription.TrainingJobStatusCounters.NonRetryableError,
			RetryableError:    sageMakerDescription.TrainingJobStatusCounters.RetryableError,
			TotalError:        totalError,
			Stopped:           sageMakerDescription.TrainingJobStatusCounters.Stopped,
		}
	} else {
		return &commonv1.TrainingJobStatusCounters{}
	}
}
