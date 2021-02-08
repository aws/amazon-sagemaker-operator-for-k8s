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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	processingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/processingjob"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
	"github.com/aws/aws-sdk-go/aws"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sagemaker"
)

const (
	// ReconcilingProcessingJobStatus is the status used by the controller during reconciliation.
	ReconcilingProcessingJobStatus = "Reconciling"

	// MaxProcessingJobNameLength defines the maximum number of characters in a SageMaker Processing Job name
	MaxProcessingJobNameLength = 63
)

// Reconciler reconciles a ProcessingJob object
type Reconciler struct {
	client.Client
	Log                   logr.Logger
	PollInterval          time.Duration
	createSageMakerClient clientwrapper.SageMakerClientWrapperProvider
	awsConfigLoader       controllers.AwsConfigLoader
}

// NewProcessingJobReconciler creates a new reconciler with the default SageMaker client.
func NewProcessingJobReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createSageMakerClient: func(cfg aws.Config) clientwrapper.SageMakerClientWrapper {
			return clientwrapper.NewSageMakerClientWrapper(sagemaker.New(awssession.New(), &cfg))
		},
		awsConfigLoader: controllers.NewAwsConfigLoader(),
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=processingjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=processingjobs/status,verbs=get;update;patch

// Reconcile attempts to reconcile the SageMaker resource state with the k8s desired state.
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context:       context.Background(),
		Log:           r.Log.WithValues("processingjob", req.NamespacedName),
		ProcessingJob: new(processingjobv1.ProcessingJob),
	}

	ctx.Log.Info("Getting resource")
	if err := r.Get(ctx, req.NamespacedName, ctx.ProcessingJob); err != nil {
		ctx.Log.Info("Unable to fetch ProcessingJob job", "reason", err)

		if apierrs.IsNotFound(err) {
			return controllers.NoRequeue()
		}

		return controllers.RequeueImmediately()
	}

	if err := r.reconcileProcessingJob(ctx); err != nil {
		ctx.Log.Info("Got error while reconciling, will retry", "err", err)
		return controllers.RequeueImmediately()
	}

	switch ctx.ProcessingJob.Status.ProcessingJobStatus {
	case string(sagemaker.ProcessingJobStatusCompleted):
		fallthrough
	case string(sagemaker.ProcessingJobStatusFailed):
		fallthrough
	case string(sagemaker.ProcessingJobStatusStopped):
		return controllers.NoRequeue()
	default:
		return controllers.RequeueAfterInterval(r.PollInterval, nil)
	}
}

type reconcileRequestContext struct {
	context.Context

	Log             logr.Logger
	SageMakerClient clientwrapper.SageMakerClientWrapper

	// The desired state of the ProcessingJob
	ProcessingJob *processingjobv1.ProcessingJob

	// The SageMaker ProcessingJob description.
	ProcessingJobDescription *sagemaker.DescribeProcessingJobOutput

	// The name of the SageMaker ProcessingJob.
	ProcessingJobName string
}

func (r *Reconciler) reconcileProcessingJob(ctx reconcileRequestContext) error {
	var err error

	// Set first-touch status
	if ctx.ProcessingJob.Status.ProcessingJobStatus == "" {
		if err = r.updateStatus(ctx, controllers.InitializingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if it's not marked for deletion.
	if !controllers.HasDeletionTimestamp(ctx.ProcessingJob.ObjectMeta) {
		if !controllers.ContainsString(ctx.ProcessingJob.ObjectMeta.GetFinalizers(), controllers.SageMakerResourceFinalizerName) {
			ctx.ProcessingJob.ObjectMeta.Finalizers = append(ctx.ProcessingJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.ProcessingJob); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer added")
		}
	}

	// Get the ProcessingJob from SageMaker
	if ctx.ProcessingJobDescription, err = ctx.SageMakerClient.DescribeProcessingJob(ctx, ctx.ProcessingJobName); err != nil {
		return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to describe SageMaker processing job"))
	}

	// The resource does not exist within SageMaker yet.
	if ctx.ProcessingJobDescription == nil {
		if controllers.HasDeletionTimestamp(ctx.ProcessingJob.ObjectMeta) {
			return r.removeFinalizer(ctx)
		}

		if err = r.createProcessingJob(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to create processing job"))
		}

		if ctx.ProcessingJobDescription, err = ctx.SageMakerClient.DescribeProcessingJob(ctx, ctx.ProcessingJobName); err != nil {
			return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to describe SageMaker processing job"))
		}
	}

	switch *ctx.ProcessingJobDescription.ProcessingJobStatus {
	case sagemaker.ProcessingJobStatusInProgress:
		if controllers.HasDeletionTimestamp(ctx.ProcessingJob.ObjectMeta) {
			// Request to stop the job
			if _, err := ctx.SageMakerClient.StopProcessingJob(ctx, ctx.ProcessingJobName); err != nil && !clientwrapper.IsStopTrainingJob404Error(err) {
				return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to delete processing job"))
			}
			// Describe the new state of the job
			if ctx.ProcessingJobDescription, err = ctx.SageMakerClient.DescribeProcessingJob(ctx, ctx.ProcessingJobName); err != nil {
				return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to describe SageMaker processing job"))
			}
		}
		break

	case sagemaker.ProcessingJobStatusCompleted:
		fallthrough

	case sagemaker.ProcessingJobStatusStopped, sagemaker.ProcessingJobStatusFailed:
		if controllers.HasDeletionTimestamp(ctx.ProcessingJob.ObjectMeta) {
			return r.removeFinalizer(ctx)
		}
		break

	case sagemaker.ProcessingJobStatusStopping:
		break

	default:
		unknownStateError := errors.New(fmt.Sprintf("Unknown Processing Job Status: %s", *ctx.ProcessingJobDescription.ProcessingJobStatus))
		return r.updateStatusAndReturnError(ctx, unknownStateError)
	}

	status := *ctx.ProcessingJobDescription.ProcessingJobStatus
	additional := controllers.GetOrDefault(ctx.ProcessingJobDescription.FailureReason, "")

	if err = r.updateStatusWithAdditional(ctx, status, additional); err != nil {
		return err
	}
	return nil
}

// Initialize fields on the context object which will be used later.
func (r *Reconciler) initializeContext(ctx *reconcileRequestContext) error {

	ctx.ProcessingJobName = controllers.GetGeneratedJobName(ctx.ProcessingJob.ObjectMeta.GetUID(), ctx.ProcessingJob.ObjectMeta.GetName(), MaxProcessingJobNameLength)

	ctx.Log.Info("ProcessingJob", "Using job name: ", ctx.ProcessingJobName)

	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.ProcessingJob.Spec.Region, ctx.ProcessingJob.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.SageMakerClient = r.createSageMakerClient(awsConfig)
	ctx.Log.Info("Loaded AWS config")

	return nil
}

// Creates the processing job in SageMaker
func (r *Reconciler) createProcessingJob(ctx reconcileRequestContext) error {
	var createProcessingJobInput sagemaker.CreateProcessingJobInput

	createProcessingJobInput = sdkutil.CreateCreateProcessingJobInputFromSpec(ctx.ProcessingJob.Spec, &ctx.ProcessingJobName)

	ctx.Log.Info("Creating ProcessingJob in SageMaker", "input", createProcessingJobInput)

	if _, err := ctx.SageMakerClient.CreateProcessingJob(ctx, &createProcessingJobInput); err != nil {
		return errors.Wrap(err, "Unable to create Processing Job")
	}

	return nil
}

// Removes the finalizer held by our controller.
func (r *Reconciler) removeFinalizer(ctx reconcileRequestContext) error {
	var err error

	ctx.ProcessingJob.ObjectMeta.Finalizers = controllers.RemoveString(ctx.ProcessingJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
	if err = r.Update(ctx, ctx.ProcessingJob); err != nil {
		return errors.Wrap(err, "Failed to remove finalizer")
	}
	ctx.Log.Info("Finalizer has been removed")

	return nil
}

// If this function returns an error, the status update has failed, and the reconciler should always requeue.
// This prevents the case where a terminal status fails to persist to the Kubernetes datastore yet we stop
// reconciling and thus leave the job in an unfinished state.
func (r *Reconciler) updateStatus(ctx reconcileRequestContext, processingJobStatus string) error {
	return r.updateStatusWithAdditional(ctx, processingJobStatus, "")
}

func (r *Reconciler) updateStatusAndReturnError(ctx reconcileRequestContext, reconcileErr error) error {
	processingJobStatus := controllers.ErrorStatus
	if clientwrapper.IsRecoverableError(reconcileErr) {
		processingJobStatus = ReconcilingProcessingJobStatus
	}
	if err := r.updateStatusWithAdditional(ctx, processingJobStatus, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

func (r *Reconciler) updateStatusWithAdditional(ctx reconcileRequestContext, processingJobStatus, additional string) error {
	ctx.Log.Info("updateStatusWithAdditional", "processingJobStatus", processingJobStatus, "additional", additional)

	jobStatus := &ctx.ProcessingJob.Status

	// When you call this function, update/refresh all the fields since we overwrite.
	jobStatus.ProcessingJobStatus = processingJobStatus
	jobStatus.Additional = additional
	jobStatus.LastCheckTime = controllers.Now()
	jobStatus.SageMakerProcessingJobName = ctx.ProcessingJobName
	jobStatus.CloudWatchLogURL = "https://" + *ctx.ProcessingJob.Spec.Region + ".console.aws.amazon.com/cloudwatch/home?region=" +
		*ctx.ProcessingJob.Spec.Region + "#logStream:group=/aws/sagemaker/ProcessingJobs;prefix=" +
		ctx.ProcessingJobName + ";streamFilter=typeLogStreamPrefix"

	if err := r.Status().Update(ctx, ctx.ProcessingJob); err != nil {
		err = errors.Wrap(err, "Unable to update status")
		ctx.Log.Info("Error while updating status.", "err", err)
		return err
	}
	return nil
}

// SetupWithManager adds this reconciler to the manager, so that it gets started when the manager is started
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&processingjobv1.ProcessingJob{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
