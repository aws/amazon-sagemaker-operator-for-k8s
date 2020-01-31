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

package batchtransformjob

import (
	"context"
	//"errors"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	awserr "github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"

	batchtransformjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/batchtransformjob"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
)

// The HTTP message returned with HTTP code 400 if a DescribeTransformJob request
// cannot find the given transform job.
const transformResourceNotFoundApiCode string = "ValidationException"

// BatchTransformJobReconciler reconciles a BatchTransformJob object
type BatchTransformJobReconciler struct {
	client.Client
	Log logr.Logger

	PollInterval          time.Duration
	createSageMakerClient SageMakerClientProvider
	awsConfigLoader       AwsConfigLoader
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=batchtransformjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=batchtransformjobs/status,verbs=get;update;patch

// Create a new reconciler with the default SageMaker client.
func NewBatchTransformJobReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *BatchTransformJobReconciler {
	return &BatchTransformJobReconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createSageMakerClient: func(cfg aws.Config) sagemakeriface.ClientAPI {
			return sagemaker.New(cfg)
		},
		awsConfigLoader: NewAwsConfigLoader(),
	}
}

type reconcileRequestContext struct {
	context.Context

	Log                  logr.Logger
	Job                  batchtransformjobv1.BatchTransformJob
	SageMakerDescription *sagemaker.DescribeTransformJobOutput

	SageMakerClient sagemakeriface.ClientAPI
}

func (r *BatchTransformJobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context: context.Background(),
		Log:     r.Log.WithValues("batchtransformjob", req.NamespacedName),
	}

	// Get state from etcd
	if err := r.Get(ctx, req.NamespacedName, &ctx.Job); err != nil {
		ctx.Log.Info("Unable to fetch Batchtransformjob", "reason", err)
		return RequeueIfError(IgnoreNotFound(err))
	}

	return r.reconcileJob(ctx)
}

func (r *BatchTransformJobReconciler) reconcileJob(ctx reconcileRequestContext) (ctrl.Result, error) {
	log := ctx.Log.WithName("reconcileJob")

	if ctx.Job.Status.TransformJobStatus == "" {
		status := InitializingJobStatus
		log.Info("Job status is empty, setting to intermediate status", "status", status)
		if err := r.updateJobStatus(ctx, batchtransformjobv1.BatchTransformJobStatus{
			TransformJobStatus: status,
			LastCheckTime:      Now(),
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

	if !ctx.Job.ObjectMeta.GetDeletionTimestamp().IsZero() {
		return r.reconcileJobDeletion(ctx)
	}

	if !ContainsString(ctx.Job.ObjectMeta.GetFinalizers(), SageMakerResourceFinalizerName) {
		return r.addFinalizerAndRequeue(ctx)
	}

	// Generate the SageMaker transform job name if user does not specifies in spec
	if ctx.Job.Spec.TransformJobName == nil || len(*ctx.Job.Spec.TransformJobName) == 0 {
		jobName := r.getTransformJobName(ctx.Job)
		ctx.Job.Spec.TransformJobName = &jobName

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

	ctx.Log = ctx.Log.WithValues("batchtransform-job-name", *ctx.Job.Spec.TransformJobName)

	var requestErr awserr.RequestFailure
	ctx.SageMakerDescription, requestErr = r.getSageMakerDescription(ctx)
	if ctx.SageMakerDescription == nil && requestErr == nil {
		return r.createBatchTransformJob(ctx)
	} else if ctx.SageMakerDescription != nil {
		return r.reconcileSpecWithDescription(ctx)
	} else {

		ctx.Log.Info("Error getting batchtransformjob state in SageMaker", "requestErr", requestErr)
		return r.handleSageMakerApiFailure(ctx, requestErr, false)
	}

}

// Batch Transform Job Deletion
func (r *BatchTransformJobReconciler) reconcileJobDeletion(ctx reconcileRequestContext) (ctrl.Result, error) {
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
			log.Info("BatchTransform job does not exist in Sagemaker, removing finalizer")
			return r.removeFinalizerAndUpdate(ctx)

		} else {
			// Case 2
			log.Info("Sagemaker returns 4xx or 5xx or unrecoverable API Error")

			// Handle the 500 or unrecoverable API Error
			return r.handleSageMakerApiFailure(ctx, requestErr, true)
		}
	} else {
		log.Info("Job exists in Sagemaker, lets delete it")
		return r.deleteBatchTransformJobIfFinalizerExists(ctx)
	}
}

// Remove the finalizer and update etcd
func (r *BatchTransformJobReconciler) removeFinalizerAndUpdate(ctx reconcileRequestContext) (ctrl.Result, error) {
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
func (r *BatchTransformJobReconciler) deleteBatchTransformJobIfFinalizerExists(ctx reconcileRequestContext) (ctrl.Result, error) {
	log := ctx.Log.WithName("deleteBatchTransformJobIfFinalizerExists")

	if !ContainsString(ctx.Job.ObjectMeta.Finalizers, SageMakerResourceFinalizerName) {
		log.Info("Object does not have finalizer nothing to do!!!")
		return NoRequeue()
	}

	log.Info("Object has been scheduled for deletion")
	switch ctx.SageMakerDescription.TransformJobStatus {
	case sagemaker.TransformJobStatusInProgress:
		log.Info("Job is in_progress and has finalizer, so we need to delete it")
		req := ctx.SageMakerClient.StopTransformJobRequest(&sagemaker.StopTransformJobInput{
			TransformJobName: ctx.Job.Spec.TransformJobName,
		})
		_, err := req.Send(ctx)
		if err != nil {
			log.Error(err, "Unable to stop the job in sagemaker", "context", ctx)
			return r.handleSageMakerApiFailure(ctx, err, false)
		}

		return RequeueImmediately()

	case sagemaker.TransformJobStatusStopping:

		log.Info("Job is stopping, nothing to do")
		if err := r.updateJobStatus(ctx, batchtransformjobv1.BatchTransformJobStatus{
			LastCheckTime:             Now(),
			SageMakerTransformJobName: *ctx.Job.Spec.TransformJobName,
			TransformJobStatus:        string(ctx.SageMakerDescription.TransformJobStatus),
		}); err != nil {
			return RequeueIfError(err)
		}
		return RequeueAfterInterval(r.PollInterval, nil)
	case sagemaker.TransformJobStatusCompleted, sagemaker.TransformJobStatusFailed, sagemaker.TransformJobStatusStopped:
		log.Info("Job is in terminal state. Done")
		return r.removeFinalizerAndUpdate(ctx)
	default:
		log.Info("Job is in unknown status")
		return NoRequeue()
	}
}

func (r *BatchTransformJobReconciler) addFinalizerAndRequeue(ctx reconcileRequestContext) (ctrl.Result, error) {
	ctx.Job.ObjectMeta.Finalizers = append(ctx.Job.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
	ctx.Log.Info("Add finalizer and Requeue")
	prevGeneration := ctx.Job.ObjectMeta.GetGeneration()
	if err := r.Update(ctx, &ctx.Job); err != nil {
		ctx.Log.Error(err, "Failed to add finalizer", "StatusUpdateError", ctx)
		return RequeueIfError(err)
	}

	return RequeueImmediatelyUnlessGenerationChanged(prevGeneration, ctx.Job.ObjectMeta.GetGeneration())
}

func (r *BatchTransformJobReconciler) getTransformJobName(state batchtransformjobv1.BatchTransformJob) string {
	return GetGeneratedResourceName(state.ObjectMeta.GetUID(), state.ObjectMeta.GetName(), 63)
}

func (r *BatchTransformJobReconciler) reconcileSpecWithDescription(ctx reconcileRequestContext) (ctrl.Result, error) {
	if comparison := TransformJobSpecMatchesDescription(*ctx.SageMakerDescription, ctx.Job.Spec); !comparison.Equal {
		ctx.Log.Info("Spec does not match description. Creating error message and not requeueing")
		const status = string(sagemaker.TransformJobStatusFailed)
		if err := r.updateJobStatus(ctx, batchtransformjobv1.BatchTransformJobStatus{
			LastCheckTime:             Now(),
			SageMakerTransformJobName: *ctx.Job.Spec.TransformJobName,
			TransformJobStatus:        status,
			Additional:                CreateSpecDiffersFromDescriptionErrorMessage(ctx.Job, status, comparison.Differences),
		}); err != nil {
			return RequeueIfError(err)
		}
	}

	if err := r.updateJobStatus(ctx, batchtransformjobv1.BatchTransformJobStatus{
		LastCheckTime:             Now(),
		TransformJobStatus:        string(ctx.SageMakerDescription.TransformJobStatus),
		SageMakerTransformJobName: *ctx.Job.Spec.TransformJobName,
	}); err != nil {
		return RequeueIfError(err)
	}

	observedStatus := ctx.SageMakerDescription.TransformJobStatus
	inProgress := sagemaker.TransformJobStatusInProgress
	stopping := sagemaker.TransformJobStatusStopping

	if observedStatus == inProgress || observedStatus == stopping {
		return RequeueAfterInterval(r.PollInterval, nil)
	}

	return NoRequeue()
}

func (r *BatchTransformJobReconciler) handleSageMakerApiFailure(ctx reconcileRequestContext, apiErr error, allowRemoveFinalizer bool) (ctrl.Result, error) {
	if err := r.updateJobStatus(ctx, batchtransformjobv1.BatchTransformJobStatus{
		Additional:                apiErr.Error(),
		LastCheckTime:             Now(),
		TransformJobStatus:        string(sagemaker.TransformJobStatusFailed),
		SageMakerTransformJobName: *ctx.Job.Spec.TransformJobName,
	}); err != nil {
		return RequeueIfError(err)
	}

	if awsErr, apiErrIsRequestFailure := apiErr.(awserr.RequestFailure); apiErrIsRequestFailure {
		if r.isSageMaker429Response(awsErr) {
			ctx.Log.Info("SageMaker rate limit exceeded, will retry", "err", awsErr)
			return RequeueAfterInterval(r.PollInterval, nil)
		} else if awsErr.StatusCode() == 400 {

			if allowRemoveFinalizer {
				return r.removeFinalizerAndUpdate(ctx)
			}

			return NoRequeue()
		} else {
			return RequeueAfterInterval(r.PollInterval, nil)
		}
	} else {
		ctx.Log.Info("Unknown request failure type for error.", "error", apiErr)
		return RequeueAfterInterval(r.PollInterval, nil)
	}
}

func (r *BatchTransformJobReconciler) updateJobStatus(ctx reconcileRequestContext, desiredStatus batchtransformjobv1.BatchTransformJobStatus) error {
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

func (r *BatchTransformJobReconciler) createBatchTransformJob(ctx reconcileRequestContext) (ctrl.Result, error) {
	ctx.Log.Info("Creating BatchTransformJob in SageMaker")

	input := CreateCreateBatchTransformJobInputFromSpec(ctx.Job.Spec)
	request := ctx.SageMakerClient.CreateTransformJobRequest(&input)
	ctx.Log.Info("Transform job request", "request", input)

	// Add `sagemaker-on-kubernetes` string literal to identify the k8s job in sagemaker
	aws.AddToUserAgent(request.Request, SagemakerOnKubernetesUserAgentAddition)

	_, createError := request.Send(ctx)
	if createError == nil {
		ctx.Log.Info("TransformJob created in SageMaker")
		return RequeueImmediately()
	}
	ctx.Log.Info("Unable to create Transform job", "createError", createError)
	return r.handleSageMakerApiFailure(ctx, createError, false)
}

func (r *BatchTransformJobReconciler) getSageMakerDescription(ctx reconcileRequestContext) (*sagemaker.DescribeTransformJobOutput, awserr.RequestFailure) {
	describeRequest := ctx.SageMakerClient.DescribeTransformJobRequest(&sagemaker.DescribeTransformJobInput{
		TransformJobName: ctx.Job.Spec.TransformJobName,
	})

	describeResponse, describeError := describeRequest.Send(ctx)
	log := ctx.Log.WithName("getSageMakerDescription")

	if awsErr, requestFailed := describeError.(awserr.RequestFailure); requestFailed {
		if r.isSageMaker404Response(awsErr) {
			log.Info("Job does not exist in sagemaker")
			return nil, nil
		} else {
			log.Info("Non-404 error response from DescribeTransformJob")
			return nil, awsErr
		}
	} else if describeError != nil {
		// TODO: Add unit test for this
		log.Info("Failed to parse the describe error output from sagemaker")
		return nil, awsErr
	}
	return describeResponse.DescribeTransformJobOutput, nil
}

func (r *BatchTransformJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchtransformjobv1.BatchTransformJob{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// TransformJob error responses use a different mechanism, so this code cannot be common to both reconcilers.
func (r *BatchTransformJobReconciler) isSageMaker404Response(awsError awserr.RequestFailure) bool {
	return awsError.Code() == transformResourceNotFoundApiCode
}

// When we run DescribeTransformJob with the name of job, sagemaker returns throttling exception
// with error code 400 instead of 429.
func (r *BatchTransformJobReconciler) isSageMaker429Response(awsError awserr.RequestFailure) bool {
	return (awsError.Code() == "ThrottlingException") && (awsError.Message() == "Rate exceeded")
}
