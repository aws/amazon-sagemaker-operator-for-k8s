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
	"time"

	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	trainingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/trainingjob"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	"github.com/go-logr/logr"
)

// All the status used by the controller during reconciliation.
const (
	ReconcilingTrainingJobStatus = "ReconcilingTrainingJob"
)

// Reconciler reconciles a TrainingJob object
type Reconciler struct {
	client.Client
	Log                   logr.Logger
	PollInterval          time.Duration
	createSageMakerClient controllers.SageMakerClientProvider
	awsConfigLoader       controllers.AwsConfigLoader
}

// NewTrainingJobReconciler creates a new reconciler with the default SageMaker client.
func NewTrainingJobReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createSageMakerClient: func(cfg aws.Config) sagemakeriface.ClientAPI {
			return sagemaker.New(cfg)
		},
		awsConfigLoader: controllers.NewAwsConfigLoader(),
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=trainingjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=trainingjobs/status,verbs=get;update;patch

// Reconcile attempts to bring the status of the k8s resource up to date with the SageMaker resource.
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context:     context.Background(),
		Log:         r.Log.WithValues("trainingjob", req.NamespacedName),
		TrainingJob: new(trainingjobv1.TrainingJob),
	}

	ctx.Log.Info("Getting resource")

	////////////////////////////////////////////////////////////////////////////////////////////////////
	// GET STATE FROM ETCD
	////////////////////////////////////////////////////////////////////////////////////////////////////

	if err := r.Get(ctx, req.NamespacedName, ctx.TrainingJob); err != nil {
		ctx.Log.Info("Unable to fetch TrainingJob job", "reason", err)

		if apierrs.IsNotFound(err) {
			return controllers.NoRequeue()
		}

		return controllers.RequeueImmediately()
	}

	if err := r.reconcileTrainingJob(ctx); err != nil {
		ctx.Log.Info("Got error while reconciling, will retry", "err", err)
		return controllers.RequeueImmediately()
	}

	switch ctx.TrainingJob.Status.TrainingJobStatus {
	case string(sagemaker.TrainingJobStatusCompleted):
		fallthrough
	case string(sagemaker.TrainingJobStatusFailed):
		fallthrough
	case string(sagemaker.TrainingJobStatusStopped):
		return controllers.NoRequeue()
	default:
		return controllers.RequeueAfterInterval(r.PollInterval, nil)
	}
}

func (r *Reconciler) reconcileTrainingJob(ctx reconcileRequestContext) error {
	var err error

	// Set first-touch status
	if ctx.TrainingJob.Status.TrainingJobStatus == "" {
		if err = r.updateStatus(ctx, controllers.InitializingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, string(sagemaker.EndpointStatusFailed), errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if it's not marked for deletion.
	if !controllers.HasDeletionTimestamp(ctx.TrainingJob.ObjectMeta) {
		if !controllers.ContainsString(ctx.TrainingJob.ObjectMeta.GetFinalizers(), controllers.SageMakerResourceFinalizerName) {
			ctx.TrainingJob.ObjectMeta.Finalizers = append(ctx.TrainingJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.TrainingJob); err != nil {
				return errors.Wrap(err, "Failed to add finaliver")
			}
			ctx.Log.Info("Finalizer added")
		}
	}

	// Get the TrainingJob from SageMaker
	if ctx.TrainingJobDescription, err = ctx.SageMakerClient.DescribeTrainingJob(ctx, ctx.TrainingJobName); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingTrainingJobStatus, errors.Wrap(err, "Unable to describe SageMaker training job"))
	}

	if ctx.TrainingJobDescription == nil {
		if controllers.HasDeletionTimestamp(ctx.TrainingJob.ObjectMeta) {
			return r.cleanupAndRemoveFinalizer(ctx)
		}

		if err = r.createTrainingJob(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingTrainingJobStatus, errors.Wrap(err, "Unable to create training job"))
		}

		if ctx.TrainingJobDescription, err = ctx.SageMakerClient.DescribeTrainingJob(ctx, ctx.TrainingJobName); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingTrainingJobStatus, errors.Wrap(err, "Unable to describe SageMaker training job"))
		}
	}

	//TODO: Convert it to tinyurl or even better can we expose CW url via API server proxy UI?
	ctx.CloudWatchLogURL = "https://" + *ctx.TrainingJob.Spec.Region + ".console.aws.amazon.com/cloudwatch/home?region=" +
		*ctx.TrainingJob.Spec.Region + "#ctx.LogStream:group=/aws/sagemaker/TrainingJobs;prefix=" +
		*ctx.TrainingJobDescription.TrainingJobName + ";streamFilter=typeLogStreamPrefix"

	switch ctx.TrainingJobDescription.TrainingJobStatus {
	case sagemaker.TrainingJobStatusInProgress, sagemaker.TrainingJobStatusStopping:
		if !controllers.HasDeletionTimestamp(ctx.TrainingJob.ObjectMeta) {
			if err = r.handleUpdates(ctx); err != nil {
				return r.updateStatusAndReturnError(ctx, ReconcilingTrainingJobStatus, errors.Wrap(err, "Unable to update SageMaker trainingjob"))
			}
		} else {
			if _, err := ctx.SageMakerClient.StopTrainingJob(ctx, ctx.TrainingJobName); err != nil {
				return r.updateStatusAndReturnError(ctx, ReconcilingTrainingJobStatus, errors.Wrap(err, "Unable to delete training job"))
			}
		}

		break

	case sagemaker.TrainingJobStatusCompleted:
		if err = r.updateCompleted(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingTrainingJobStatus, errors.Wrap(err, "Unable to update training job"))
		}

		fallthrough

	case sagemaker.TrainingJobStatusStopped, sagemaker.TrainingJobStatusFailed:
		if controllers.HasDeletionTimestamp(ctx.TrainingJob.ObjectMeta) {
			return r.cleanupAndRemoveFinalizer(ctx)
		}

		break

	default:
		unknownStateError := errors.New(string("Unknown Training Job Status " + ctx.TrainingJobDescription.TrainingJobStatus))
		return r.updateStatusAndReturnError(ctx, ReconcilingTrainingJobStatus, errors.Wrap(unknownStateError, "Job is in an unknown status"))
	}

	if err = r.updateBothStatus(ctx, string(ctx.TrainingJobDescription.TrainingJobStatus), string(ctx.TrainingJobDescription.SecondaryStatus)); err != nil {
		return err
	}

	return nil
}

type reconcileRequestContext struct {
	context.Context

	Log             logr.Logger
	SageMakerClient clientwrapper.SageMakerClientWrapper

	// The desired state of the TrainingJob
	TrainingJob *trainingjobv1.TrainingJob

	// The SageMaker TrainingJob description.
	TrainingJobDescription *sagemaker.DescribeTrainingJobOutput

	// The name of the SageMaker TrainingJob.
	TrainingJobName string

	// The path to the CloudWatch logs for the current training job
	CloudWatchLogURL string
}

// Initialize fields on the context object which will be used later.
func (r *Reconciler) initializeContext(ctx *reconcileRequestContext) error {
	ctx.TrainingJobName = getTrainingJobName(ctx.TrainingJob)
	ctx.Log.Info("TrainingJob", "name", ctx.TrainingJobName)

	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.TrainingJob.Spec.Region, ctx.TrainingJob.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.SageMakerClient = clientwrapper.NewSageMakerClientWrapper(r.createSageMakerClient(awsConfig))
	ctx.Log.Info("Loaded AWS config")

	return nil
}

// Function to construct the sagemaker training job name
func getTrainingJobName(state *trainingjobv1.TrainingJob) string {
	return controllers.GetGeneratedJobName(state.ObjectMeta.GetUID(), state.ObjectMeta.GetName(), 63)
}

func (r *Reconciler) etcdMatchesSmAPI(state trainingjobv1.TrainingJob, describeResponse *sagemaker.DescribeTrainingJobResponse) bool {
	primaryStatusMatches := state.Status.TrainingJobStatus == string(describeResponse.DescribeTrainingJobOutput.TrainingJobStatus)
	secondaryStatusMatches := state.Status.SecondaryStatus == string(describeResponse.DescribeTrainingJobOutput.SecondaryStatus)
	allMatch := primaryStatusMatches && secondaryStatusMatches
	return allMatch
}

// Creates the training job in SageMaker
func (r *Reconciler) createTrainingJob(ctx reconcileRequestContext) error {
	var createTrainingJobInput sagemaker.CreateTrainingJobInput

	if ctx.TrainingJob.Spec.TrainingJobName == nil || len(*ctx.TrainingJob.Spec.TrainingJobName) == 0 {
		ctx.TrainingJob.Spec.TrainingJobName = &ctx.TrainingJobName
	}

	createTrainingJobInput = sdkutil.CreateCreateTrainingJobInputFromSpec(ctx.TrainingJob.Spec)

	ctx.Log.Info("Creating TrainingJob in SageMaker", "input", createTrainingJobInput)

	if _, err := ctx.SageMakerClient.CreateTrainingJob(ctx, &createTrainingJobInput); err != nil {
		return errors.Wrap(err, "Unable to create Training Job")
	}

	return nil
}

func (r *Reconciler) handleUpdates(ctx reconcileRequestContext) error {
	// Don't handle any update functionality

	return nil
}

// Remove the finalizer and update etcd
func (r *Reconciler) removeFinalizerAndUpdate(ctx reconcileRequestContext) (ctrl.Result, error) {
	ctx.Log.Info("removeFinalizerAndUpdate")
	ctx.TrainingJob.ObjectMeta.Finalizers = controllers.RemoveString(ctx.TrainingJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)

	err := r.Update(ctx, ctx.TrainingJob)
	return controllers.RequeueIfError(err)
}

func (r *Reconciler) updateCompleted(ctx reconcileRequestContext) error {
	var err error

	// If job has completed populate the model full path
	ctx.Log.Info("Training has completed updating model path")

	// SageMaker stores the model artifact in OutputDataConfig path with path /output/model.tar.gz
	// SageMaker documentation https://docs.aws.amazon.com/sagemaker/latest/dg/cdf-training.html
	const outputPath string = "/output/model.tar.gz"
	ctx.TrainingJob.Status.ModelPath = *ctx.TrainingJob.Spec.OutputDataConfig.S3OutputPath + ctx.TrainingJob.Status.SageMakerTrainingJobName + outputPath
	if err = r.Update(ctx, ctx.TrainingJob); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingTrainingJobStatus, errors.Wrap(err, "Error updating ETCD to sync with SM API ctx.TrainingJob"))
	}

	return nil
}

// Clean up any deployments artifacts, then removes the finalizer.
func (r *Reconciler) cleanupAndRemoveFinalizer(ctx reconcileRequestContext) error {
	var err error

	if controllers.HasDeletionTimestamp(ctx.TrainingJob.ObjectMeta) {
		ctx.TrainingJob.ObjectMeta.Finalizers = controllers.RemoveString(ctx.TrainingJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
		if err = r.Update(ctx, ctx.TrainingJob); err != nil {
			return errors.Wrap(err, "Failed to remove finalizer")
		}
		ctx.Log.Info("Finalizer has been removed")
	}

	return nil
}

// If this function returns an error, the status update has failed, and the reconciler should always requeue.
// This prevents the case where a terminal status fails to persist to the Kubernetes datastore yet we stop
// reconciling and thus leave the job in an unfinished state.
func (r *Reconciler) updateStatus(ctx reconcileRequestContext, trainingJobPrimaryStatus string) error {
	return r.updateStatusWithAdditional(ctx, trainingJobPrimaryStatus, "", "")
}

func (r *Reconciler) updateBothStatus(ctx reconcileRequestContext, trainingJobPrimaryStatus, trainingJobSecondaryStatus string) error {
	return r.updateStatusWithAdditional(ctx, trainingJobPrimaryStatus, trainingJobSecondaryStatus, "")
}

func (r *Reconciler) updateStatusAndReturnError(ctx reconcileRequestContext, trainingJobPrimaryStatus string, reconcileErr error) error {
	if err := r.updateBothStatusAndReturnError(ctx, trainingJobPrimaryStatus, "", reconcileErr); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

func (r *Reconciler) updateBothStatusAndReturnError(ctx reconcileRequestContext, trainingJobPrimaryStatus, trainingJobSecondaryStatus string, reconcileErr error) error {
	if err := r.updateStatusWithAdditional(ctx, trainingJobPrimaryStatus, trainingJobSecondaryStatus, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

func (r *Reconciler) updateStatusWithAdditional(ctx reconcileRequestContext, trainingJobPrimaryStatus, trainingJobSecondaryStatus, additional string) error {
	ctx.Log.Info("updateStatusWithAdditional", "trainingJobPrimaryStatus", trainingJobPrimaryStatus, "trainingJobSecondaryStatus", trainingJobSecondaryStatus, "additional", additional)

	jobStatus := &ctx.TrainingJob.Status
	// When you call this function, update/refresh all the fields since we overwrite.
	jobStatus.TrainingJobStatus = trainingJobPrimaryStatus
	jobStatus.SecondaryStatus = trainingJobSecondaryStatus
	jobStatus.Additional = additional

	if err := r.Status().Update(ctx, ctx.TrainingJob); err != nil {
		err = errors.Wrap(err, "Unable to update status")
		ctx.Log.Info("Error while updating status.", "err", err)
		return err
	}

	return nil
}

// SetupWithManager configures the manager to recognise the controller.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&trainingjobv1.TrainingJob{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
