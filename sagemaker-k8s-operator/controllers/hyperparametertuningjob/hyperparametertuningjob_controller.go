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
	"time"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	commonv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/common"
	hpojobv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/hyperparametertuningjob"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/sdkutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
)

// SageMaker API returns this API error code with HTTP status code 400 when
// a DescribeHyperparameterTuningJob does not find the specified job.
const hpoResourceNotFoundApiCode string = "ResourceNotFound"

// HyperparameterTuningJobReconciler reconciles a HyperparameterTuningJob object
type HyperparameterTuningJobReconciler struct {
	client.Client
	Log          logr.Logger
	PollInterval time.Duration

	createSageMakerClient       SageMakerClientProvider
	createHpoTrainingJobSpawner HpoTrainingJobSpawnerProvider
	awsConfigLoader             AwsConfigLoader
}

func NewHyperparameterTuningJobReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *HyperparameterTuningJobReconciler {
	return &HyperparameterTuningJobReconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createSageMakerClient: func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			return sagemaker.New(awsConfig)
		},
		createHpoTrainingJobSpawner: NewHpoTrainingJobSpawner,
		awsConfigLoader:             NewAwsConfigLoader(),
	}
}

type reconcileRequestContext struct {
	context.Context

	Log                  logr.Logger
	Job                  hpojobv1.HyperparameterTuningJob
	SageMakerDescription *sagemaker.DescribeHyperParameterTuningJobOutput

	SageMakerClient       sagemakeriface.ClientAPI
	HpoTrainingJobSpawner HpoTrainingJobSpawner
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hyperparametertuningjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hyperparametertuningjobs/status,verbs=get;update;patch

func (r *HyperparameterTuningJobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context: context.Background(),
		Log:     r.Log.WithValues("hyperparametertuningjob", req.NamespacedName),
	}

	// Get state from etcd
	if err := r.Get(ctx, req.NamespacedName, &ctx.Job); err != nil {
		ctx.Log.Info("Unable to fetch HyperparameterTuningJob", "reason", err)
		return RequeueIfError(IgnoreNotFound(err))
	}

	return r.reconcileJob(ctx)
}

func (r *HyperparameterTuningJobReconciler) reconcileJob(ctx reconcileRequestContext) (ctrl.Result, error) {
	log := ctx.Log.WithName("reconcileJob")

	if ctx.Job.Status.HyperParameterTuningJobStatus == "" {
		status := InitializingJobStatus
		log.Info("Job status is empty, setting to intermediate status", "status", status)
		if err := r.updateJobStatus(ctx, hpojobv1.HyperparameterTuningJobStatus{
			HyperParameterTuningJobStatus: status,
			LastCheckTime:                 Now(),
		}); err != nil {
			return RequeueIfError(err)
		}
		return RequeueImmediately()
	}

	if awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.Job.Spec.Region, ctx.Job.Spec.SageMakerEndpoint); err != nil {
		log.Error(err, "Error loading AWS config")
		return NoRequeue()
	} else {
		ctx.SageMakerClient = r.createSageMakerClient(awsConfig)
		log.Info("Loaded AWS config")
	}

	ctx.HpoTrainingJobSpawner = r.createHpoTrainingJobSpawner(r, ctx.Log, ctx.SageMakerClient)

	if !ctx.Job.ObjectMeta.GetDeletionTimestamp().IsZero() {
		return r.reconcileJobDeletion(ctx)
	}

	if !ContainsString(ctx.Job.ObjectMeta.GetFinalizers(), SageMakerResourceFinalizerName) {
		return r.addFinalizerAndRequeue(ctx)
	}

	// Generate the SageMaker hyperparametertuning job name if user does not specifies in spec
	if ctx.Job.Spec.HyperParameterTuningJobName == nil || len(*ctx.Job.Spec.HyperParameterTuningJobName) == 0 {
		jobName := r.getHyperParameterTuningJobName(ctx.Job)
		ctx.Job.Spec.HyperParameterTuningJobName = &jobName

		log.Info("Adding generated name to spec", "new-name", jobName)
		err := r.Update(ctx, &ctx.Job)

		if err != nil {
			log.Info("Failed to add generated name to spec", "context", ctx, "error", err)
			// Requeue as the update was not successful; this will guarantee another reconciler loop.
			return RequeueIfError(err)
		} else {
			// No requeue required because we generate an update by modifying the Spec.
			// If we return a requeue here, it will cause two concurrent reconciler loops because
			// the spec update generates a new reconcile call.
			// To avoid this, we return NoRequeue here and rely on the update generated by etcd.
			return NoRequeue()
		}
	}

	ctx.Log = ctx.Log.WithValues("hpo-job-name", *ctx.Job.Spec.HyperParameterTuningJobName)

	var requestErr awserr.RequestFailure
	ctx.SageMakerDescription, requestErr = r.getSageMakerDescription(ctx)
	if ctx.SageMakerDescription == nil && requestErr == nil {
		// does not exist
		return r.createHyperParameterTuningJob(ctx)
	} else if ctx.SageMakerDescription != nil {
		// does exist
		return r.reconcileSpecWithDescription(ctx)
	} else {

		ctx.Log.Info("Error getting HPO state in SageMaker", "requestErr", requestErr)
		return r.handleSageMakerApiFailure(ctx, requestErr)
	}
}

func (r *HyperparameterTuningJobReconciler) reconcileJobDeletion(ctx reconcileRequestContext) (ctrl.Result, error) {
	var requestErr awserr.RequestFailure
	ctx.SageMakerDescription, requestErr = r.getSageMakerDescription(ctx)
	log := ctx.Log.WithName("reconcileJobDeletion")

	// SageMaker API will return `nil` description in two cases
	// Case 1: When job does not exist in SageMaker and client request is valid.
	//         Error code 404
	// Case 2: When job may or may not exist, but description is `nil` due to error
	//         in request (bad request 400) or SageMaker server side error 5xx.
	if ctx.SageMakerDescription == nil {
		if requestErr == nil {
			// Case 1
			log.Info("HyperParameterTuning job does not exist in sagemaker, removing finalizer")
			return r.removeFinalizerAndUpdate(ctx)

		} else {
			// Case 2
			log.Info("Sagemaker returns 4xx or 5xx or unrecoverable API Error")
			if requestErr.StatusCode() == 400 {
				// handleSageMakerAPIFailure does not removes the finalizer
				r.removeFinalizerAndUpdate(ctx)
			}
			// Handle the 500 or unrecoverable API Error
			return r.handleSageMakerApiFailure(ctx, requestErr)
		}
	} else {
		log.Info("Job exists in Sagemaker, lets delete it")
		return r.deleteHyperparameterTuningJobIfFinalizerExists(ctx)
	}
}

// Remove the finalizer and update etcd
func (r *HyperparameterTuningJobReconciler) removeFinalizerAndUpdate(ctx reconcileRequestContext) (ctrl.Result, error) {
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
func (r *HyperparameterTuningJobReconciler) deleteHyperparameterTuningJobIfFinalizerExists(ctx reconcileRequestContext) (ctrl.Result, error) {
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
			return r.handleSageMakerApiFailure(ctx, err)
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

func (r *HyperparameterTuningJobReconciler) addFinalizerAndRequeue(ctx reconcileRequestContext) (ctrl.Result, error) {
	ctx.Job.ObjectMeta.Finalizers = append(ctx.Job.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
	ctx.Log.Info("Add finalizer and Requeue")
	prevGeneration := ctx.Job.ObjectMeta.GetGeneration()
	if err := r.Update(ctx, &ctx.Job); err != nil {
		ctx.Log.Error(err, "Failed to add finalizer", "StatusUpdateError", ctx)
		return RequeueIfError(err)
	}

	return RequeueImmediatelyUnlessGenerationChanged(prevGeneration, ctx.Job.ObjectMeta.GetGeneration())
}

func (r *HyperparameterTuningJobReconciler) getHyperParameterTuningJobName(state hpojobv1.HyperparameterTuningJob) string {
	return GetGeneratedJobName(state.ObjectMeta.GetUID(), state.ObjectMeta.GetName(), 32)
}

func (r *HyperparameterTuningJobReconciler) getSageMakerDescription(ctx reconcileRequestContext) (*sagemaker.DescribeHyperParameterTuningJobOutput, awserr.RequestFailure) {
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

// Create a CreateHPO request and send it to SageMaker.
func (r *HyperparameterTuningJobReconciler) createHyperParameterTuningJob(ctx reconcileRequestContext) (ctrl.Result, error) {

	input := CreateCreateHyperParameterTuningJobInputFromSpec(ctx.Job.Spec)
	ctx.Log.Info("Creating HyperParameterTuningJob in SageMaker", "Request Parameters", input)

	request := ctx.SageMakerClient.CreateHyperParameterTuningJobRequest(&input)

	// Add `sagemaker-on-kubernetes` string literal to identify the k8s job in sagemaker
	aws.AddToUserAgent(request.Request, SagemakerOnKubernetesUserAgentAddition)

	if _, createError := request.Send(ctx); createError == nil {
		ctx.Log.Info("HyperParameterTuningJob created in SageMaker")
		return RequeueImmediately()
	} else {
		ctx.Log.Info("Unable to create HPO job", "createError", createError)
		return r.handleSageMakerApiFailure(ctx, createError)

	}
}

// Update job status with error. If error had a 400 HTTP error code then do not requeue, otherwise requeue after interval.
func (r *HyperparameterTuningJobReconciler) handleSageMakerApiFailure(ctx reconcileRequestContext, apiErr error) (ctrl.Result, error) {

	if err := r.updateJobStatus(ctx, hpojobv1.HyperparameterTuningJobStatus{
		Additional:                           apiErr.Error(),
		LastCheckTime:                        Now(),
		HyperParameterTuningJobStatus:        string(sagemaker.HyperParameterTuningJobStatusFailed),
		SageMakerHyperParameterTuningJobName: *ctx.Job.Spec.HyperParameterTuningJobName,
		TrainingJobStatusCounters:            newTrainingJobStatusCountersFromDescription(ctx.SageMakerDescription),
	}); err != nil {
		return RequeueIfError(err)
	}

	if awsErr, apiErrIsRequestFailure := apiErr.(awserr.RequestFailure); apiErrIsRequestFailure {
		if r.isSageMaker429Response(awsErr) {
			ctx.Log.Info("SageMaker rate limit exceeded, will retry", "err", awsErr)
			return RequeueAfterInterval(r.PollInterval, nil)
		} else if awsErr.StatusCode() == 400 {
			return NoRequeue()
		} else {
			return RequeueAfterInterval(r.PollInterval, nil)
		}
	} else {
		ctx.Log.Info("Unknown request failure type for error.", "error", apiErr)
		return RequeueAfterInterval(r.PollInterval, nil)
	}
}

func (r *HyperparameterTuningJobReconciler) reconcileSpecWithDescription(ctx reconcileRequestContext) (ctrl.Result, error) {

	if comparison := HyperparameterTuningJobSpecMatchesDescription(*ctx.SageMakerDescription, ctx.Job.Spec); !comparison.Equal {
		ctx.Log.Info("Spec does not match description. Creating error message and not requeueing")
		const status = string(sagemaker.HyperParameterTuningJobStatusFailed)
		if err := r.updateJobStatus(ctx, hpojobv1.HyperparameterTuningJobStatus{
			LastCheckTime:                        Now(),
			SageMakerHyperParameterTuningJobName: *ctx.Job.Spec.HyperParameterTuningJobName,
			HyperParameterTuningJobStatus:        status,
			TrainingJobStatusCounters:            newTrainingJobStatusCountersFromDescription(ctx.SageMakerDescription),
			Additional:                           CreateSpecDiffersFromDescriptionErrorMessage(ctx.Job, status, comparison.Differences),
		}); err != nil {
			return RequeueIfError(err)
		}
		return NoRequeue()
	}

	ctx.Log.Info("Attempting to spawn TrainingJobs that the HPO job created")
	ctx.HpoTrainingJobSpawner.SpawnMissingTrainingJobs(ctx, ctx.Job)

	// No update required.
	if r.statusMatchesDescription(ctx) {
		ctx.Log.Info("Status matches SageMaker status. Skipping update.")
		// We did this in the TrainingJob controller, do we want to do so here?
		// The downside is that the LastCheckTime will not be up to date, which will be confusing
		// to customers who invoke "describe".
		return RequeueAfterInterval(r.PollInterval, nil)
	}

	if err := r.updateJobStatus(ctx, hpojobv1.HyperparameterTuningJobStatus{
		LastCheckTime:                        Now(),
		BestTrainingJob:                      r.createBestTrainingJob(ctx),
		SageMakerHyperParameterTuningJobName: *ctx.Job.Spec.HyperParameterTuningJobName,
		TrainingJobStatusCounters:            newTrainingJobStatusCountersFromDescription(ctx.SageMakerDescription),
		HyperParameterTuningJobStatus:        string(ctx.SageMakerDescription.HyperParameterTuningJobStatus),
	}); err != nil {
		return RequeueIfError(err)
	}

	// Stop requeuing on completion, stopped and failed status.
	observedStatus := ctx.SageMakerDescription.HyperParameterTuningJobStatus
	completed := sagemaker.HyperParameterTuningJobStatusCompleted
	stopped := sagemaker.HyperParameterTuningJobStatusStopped
	failed := sagemaker.HyperParameterTuningJobStatusFailed
	inProgress := sagemaker.HyperParameterTuningJobStatusInProgress
	stopping := sagemaker.HyperParameterTuningJobStatusStopping

	if observedStatus == inProgress || observedStatus == stopping {
		return RequeueAfterInterval(r.PollInterval, nil)
	}

	if observedStatus == completed || observedStatus == stopped || observedStatus == failed {
		return NoRequeue()
	}
	// norequeue for unknown status
	ctx.Log.Info("Job is in unknown status, no requeue")
	return NoRequeue()
}

func (r *HyperparameterTuningJobReconciler) updateJobStatus(ctx reconcileRequestContext, desiredStatus hpojobv1.HyperparameterTuningJobStatus) error {
	ctx.Log.Info("Updating job status", "new-status", desiredStatus)

	// Perform deep copy so as to avoid side-effect in ctx.
	root := ctx.Job.DeepCopy()
	root.Status = desiredStatus

	if err := r.Status().Update(ctx, root); err != nil {
		ctx.Log.Error(err, "Error updating job status", "job", root)
		return err
	}

	return nil
}

func (r *HyperparameterTuningJobReconciler) statusMatchesDescription(ctx reconcileRequestContext) bool {
	// TODO implement
	return false
}

// Extract the BestTrainingJob from the SageMaker description if it exists and convert it to our Kubernetes type.
func (r *HyperparameterTuningJobReconciler) createBestTrainingJob(ctx reconcileRequestContext) *commonv1.HyperParameterTrainingJobSummary {

	if ctx.SageMakerDescription.BestTrainingJob == nil {
		ctx.Log.Info("Cannot get BestTrainingJob: No BestTrainingJob in HPO description")
		return nil
	}

	bestTrainingJob, err := convertHyperParameterTrainingJobSummaryFromSageMaker(ctx.SageMakerDescription.BestTrainingJob)

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

func (r *HyperparameterTuningJobReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&hpojobv1.HyperparameterTuningJob{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// TrainingJob error responses use a different mechanism, so this code cannot be common to both reconcilers.
func (r *HyperparameterTuningJobReconciler) isSageMaker404Response(awsError awserr.RequestFailure) bool {
	return awsError.Code() == hpoResourceNotFoundApiCode
}

// When we run describeHPOJob with the name of job, sagemaker returns throttling exception
// with error code 400 instead of 429.
func (r *HyperparameterTuningJobReconciler) isSageMaker429Response(awsError awserr.RequestFailure) bool {
	return (awsError.Code() == "ThrottlingException") && (awsError.Message() == "Rate exceeded")
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
