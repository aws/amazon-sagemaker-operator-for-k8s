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

package hostingdeploymentautoscalingjob

import (
	"context"
	"time"

	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
	aws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	//commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	hostingdeploymentautoscalingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingdeploymentautoscalingjob"
)

// All the status used by the controller during reconciliation.
// Autoscaling Job does not handle statuses similar to SageMaker.
const (
	//TODO: Change the Strings to be more model like or more application autoscaling like
	ReconcilingAutoscalingJobStatus = "ReconcilingAutoscalingJob"
	CreatedAutoscalingJobStatus     = "CreatedAutoscalingJob"
	FailedAutoscalingJobStatus      = "FailedAutoscalingJob"
	DeletedAutoscalingJobStatus     = "DeletedAutoscalingJob"

	//TODO: Check
	MaxPolicyNameLength = 63
)

// Reconciler reconciles a TrainingJob object
type Reconciler struct {
	client.Client
	Log                                logr.Logger
	PollInterval                       time.Duration
	createApplicationAutoscalingClient clientwrapper.ApplicationAutoscalingClientWrapperProvider
	awsConfigLoader                    controllers.AwsConfigLoader
}

// NewHostingDeploymentAutoscalingJobReconciler creates a new reconciler with the default ApplicationAutoscaling client.
func NewHostingDeploymentAutoscalingJobReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		// TODO: Model calls ClientAPI, check
		createApplicationAutoscalingClient: func(cfg aws.Config) clientwrapper.ApplicationAutoscalingClientWrapper {
			return clientwrapper.NewApplicationAutoscalingClientWrapper(applicationautoscaling.New(cfg))
		},
		awsConfigLoader: controllers.NewAwsConfigLoader(),
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hostingdeploymentautoscalingjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hostingdeploymentautoscalingjobs/status,verbs=get;update;patch

// Reconcile attempts to reconcile the SageMaker resource state with the k8s desired state.
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context:                         context.Background(),
		Log:                             r.Log.WithValues("hostingdeploymentautoscalingjob", req.NamespacedName),
		HostingDeploymentAutoscalingJob: new(hostingdeploymentautoscalingjobv1.HostingDeploymentAutoscalingJob),
	}

	ctx.Log.Info("Getting resource")

	// Get state from etcd
	if err := r.Get(ctx, req.NamespacedName, ctx.HostingDeploymentAutoscalingJob); err != nil {
		ctx.Log.Info("Unable to fetch HostingDeploymentAutoscalingJob", "reason", err)
		if apierrs.IsNotFound(err) {
			return controllers.NoRequeue()
		}
		return controllers.RequeueImmediately()
	}

	if err := r.reconcileHostingDeploymentAutoscalingJob(ctx); err != nil {
		ctx.Log.Info("Got error while reconciling, will retry", "err", err)
		//return controllers.RequeueImmediately()
		return controllers.NoRequeue()
	}

	return controllers.NoRequeue()

	/* Based on Training
	switch ctx.HostingDeploymentAutoscalingJob.Status.HostingDeploymentAutoscalingJobStatus {
	case CompletedAutoscalingJobStatus:
		fallthrough
	case FailedAutoscalingJobStatus:
		return controllers.NoRequeue()
	default:
		//Change to Requeue if needed.
		return controllers.NoRequeue()
	}*/

}

type reconcileRequestContext struct {
	context.Context

	Log                          logr.Logger
	ApplicationAutoscalingClient clientwrapper.ApplicationAutoscalingClientWrapper

	// The desired state of the HostingDeploymentAutoscalingJob
	HostingDeploymentAutoscalingJob *hostingdeploymentautoscalingjobv1.HostingDeploymentAutoscalingJob

	// The 2 HostingDeploymentAutoscalingJobDescription - combine ?
	//TODO Are both of them needed
	ScalingPolicyDescription *applicationautoscaling.DescribeScalingPoliciesOutput
	//ScalableTargetDescription *applicationautoscaling.DescribeScalableTargetOutput

	// The name of the SageMaker TrainingJob.
	PolicyName string

	// This needs to be updated/initialized
	// Should not be needed eventually
	//ResourceIDListfromSpec []*commonv1.AutoscalingResource
	ResourceIDList []*string
}

func (r *Reconciler) reconcileHostingDeploymentAutoscalingJob(ctx reconcileRequestContext) error {
	var err error

	// Set first-touch status
	if ctx.HostingDeploymentAutoscalingJob.Status.HostingDeploymentAutoscalingJobStatus == "" {
		if err = r.updateStatus(ctx, controllers.InitializingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, "", errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if it's not marked for deletion.
	// TODO: Check if the finalizer should be different for autoscaling vs sagemaker
	if !controllers.HasDeletionTimestamp(ctx.HostingDeploymentAutoscalingJob.ObjectMeta) {
		if !controllers.ContainsString(ctx.HostingDeploymentAutoscalingJob.ObjectMeta.GetFinalizers(), controllers.SageMakerResourceFinalizerName) {
			ctx.HostingDeploymentAutoscalingJob.ObjectMeta.Finalizers = append(ctx.HostingDeploymentAutoscalingJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.HostingDeploymentAutoscalingJob); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer added")
		}
	}

	// Get Descriptions
	//if ctx.ScalableTargetDescription, err = ctx.ApplicationAutoscalingClient.DescribeScalableTargets(ctx, ctx.PolicyName); err != nil {
	//	return r.updateStatusAndReturnError(ctx, ReconcilingAutoscalingJobStatus, "", errors.Wrap(err, "Unable to describe Scalable Target"))
	//}

	if ctx.ScalingPolicyDescription, err = ctx.ApplicationAutoscalingClient.DescribeScalingPolicies(ctx, ctx.PolicyName, *ctx.ResourceIDList[0]); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingAutoscalingJobStatus, "", errors.Wrap(err, "Unable to describe Scaling Policy"))
	}

	ctx.Log.Info("ScalingPolicyDescription", "err", ctx.ScalingPolicyDescription)

	var action controllers.ReconcileAction
	if action, err = r.determineActionForAutoscaling(ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, "", errors.Wrap(err, "Unable to determine action for HostingDeploymentAutoscaling."))
	}
	ctx.Log.Info("Determined action for model", "action", action)

	// If update or delete, delete the existing Policy.
	//TODO: This makes no sense, needsUpdate at both places ?
	if action == controllers.NeedsDelete || action == controllers.NeedsUpdate {
		if err = r.deleteAutoscalingPolicy(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, "", errors.Wrap(err, "Unable to deleteAutoscalingPolicy"))
		}

		// Delete succeeded, set ModelDescription to nil.
		ctx.ScalingPolicyDescription = nil
		//ctx.ScalableTargetDescription = nil
	}

	// If update or create, create the desired model.
	if action == controllers.NeedsCreate || action == controllers.NeedsUpdate {
		if err = r.applyAutoscalingPolicy(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, "", errors.Wrap(err, "Unable to applyAutoscalingPolicy"))
		}
	}

	// Update the status accordingly.
	status := CreatedAutoscalingJobStatus
	if ctx.ScalingPolicyDescription == nil {
		status = DeletedAutoscalingJobStatus
	}

	// TODO: Cleanup this function
	if err = r.updateStatusWithAdditional(ctx, status, ""); err != nil {
		return err
	}

	// Remove the Finalizer on delete
	if controllers.HasDeletionTimestamp(ctx.HostingDeploymentAutoscalingJob.ObjectMeta) {
		r.removeFinalizer(ctx)
	}

	return nil
}

// Removes the finalizer held by our controller.
func (r *Reconciler) removeFinalizer(ctx reconcileRequestContext) error {
	var err error

	ctx.HostingDeploymentAutoscalingJob.ObjectMeta.Finalizers = controllers.RemoveString(ctx.HostingDeploymentAutoscalingJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
	if err = r.Update(ctx, ctx.HostingDeploymentAutoscalingJob); err != nil {
		return errors.Wrap(err, "Failed to remove finalizer")
	}
	ctx.Log.Info("Finalizer has been removed")

	return err
}

// Removes the finalizer held by our controller.
func (r *Reconciler) getResourceIDListfromInputSpec(ctx *reconcileRequestContext) error {

	resourceIDListfromSpec := ctx.HostingDeploymentAutoscalingJob.Spec.ResourceID
	for _, resourceIDfromSpec := range resourceIDListfromSpec {
		ResourceID := sdkutil.ConvertAutoscalingResourceToString(*resourceIDfromSpec)
		ctx.ResourceIDList = append(ctx.ResourceIDList, ResourceID)
	}

	// TODO: error handling
	return nil
}

// Initialize fields on the context object which will be used later.
func (r *Reconciler) initializeContext(ctx *reconcileRequestContext) error {
	var err error

	// Ensure we are using the job name specified in the spec
	if ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName != nil && len(*ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName) > 0 {
		ctx.PolicyName = *ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName
	} else {
		ctx.PolicyName = controllers.GetGeneratedJobName(ctx.HostingDeploymentAutoscalingJob.ObjectMeta.GetUID(), ctx.HostingDeploymentAutoscalingJob.ObjectMeta.GetName(), MaxPolicyNameLength)
		ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName = &ctx.PolicyName

		if err := r.Update(ctx, ctx.HostingDeploymentAutoscalingJob); err != nil {
			ctx.Log.Info("Error while updating HostingDeploymentAutoscalingJob policyName in spec")
			return err
		}
	}

	// TODO: Condition needs to change
	if err = r.getResourceIDListfromInputSpec(ctx); err != nil {
		return err
	}

	// TODO: Meghna, Check why the Sagemaker Endpoint is needed.
	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.HostingDeploymentAutoscalingJob.Spec.Region, ctx.HostingDeploymentAutoscalingJob.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.ApplicationAutoscalingClient = r.createApplicationAutoscalingClient(awsConfig)
	ctx.Log.Info("Loaded AWS config")

	return nil
}

// For a desired model and an actual model, determine the action needed to reconcile the two.
// ctx.HostingDeploymentAutoscalingJob: desiredAutoscaling
// ctx.ScalingPolicyDescription: actualAutoscaling
func (r *Reconciler) determineActionForAutoscaling(ctx reconcileRequestContext) (controllers.ReconcileAction, error) {
	var err error
	if controllers.HasDeletionTimestamp(ctx.HostingDeploymentAutoscalingJob.ObjectMeta) {
		if ctx.ScalingPolicyDescription != nil {
			return controllers.NeedsDelete, nil
		}
		return controllers.NeedsNoop, nil
	}

	//TODO This will change for many:1
	if len(ctx.ScalingPolicyDescription.ScalingPolicies) == 0 {
		return controllers.NeedsCreate, nil
	}

	/* TODO: Fix this part to update the autoscaling as needed
	var comparison sdkutil.Comparison
	var err error
	if comparison, err = sdkutil.ModelSpecMatchesDescription(*actualModel, desiredModel.Spec); err != nil {
		return NeedsNoop, err
	}

	r.Log.Info("Compared existing model to actual model to determine if model needs to be updated.", "differences", comparison.Differences, "equal", comparison.Equal)

	if comparison.Equal {
		return NeedsNoop, nil
	}

	return NeedsUpdate, nil */

	return controllers.NeedsNoop, err

}

// deleteAutoscalingPolicy converts Spec to Input, Registers Target, Creates second input, applies the scalingPolicy
// same as reconcileCreation of model
func (r *Reconciler) deleteAutoscalingPolicy(ctx reconcileRequestContext) error {
	var deregisterScalableTargetInput applicationautoscaling.DeregisterScalableTargetInput
	var deleteScalingPolicyInput applicationautoscaling.DeleteScalingPolicyInput

	for _, ResourceID := range ctx.ResourceIDList {
		deleteScalingPolicyInput = sdkutil.CreateDeleteScalingPolicyInput(*ResourceID, ctx.PolicyName)
		if _, err := ctx.ApplicationAutoscalingClient.DeleteScalingPolicy(ctx, &deleteScalingPolicyInput); err != nil {
			return errors.Wrap(err, "Unable to DeleteScalingPolicy")
		}
	}

	for _, ResourceID := range ctx.ResourceIDList {
		deregisterScalableTargetInput = sdkutil.CreateDeregisterScalableTargetInput(*ResourceID)
		if _, err := ctx.ApplicationAutoscalingClient.DeregisterScalableTarget(ctx, &deregisterScalableTargetInput); err != nil {
			return errors.Wrap(err, "Unable DeregisterScalableTarget")
		}
	}

	return nil
}

// applyAutoscalingPolicy converts Spec to Input, Registers Target, Creates second input, applies the scalingPolicy
// same as reconcileCreation of model
func (r *Reconciler) applyAutoscalingPolicy(ctx reconcileRequestContext) error {
	var registerScalableTargetInputList []applicationautoscaling.RegisterScalableTargetInput
	var putScalingPolicyInputList []applicationautoscaling.PutScalingPolicyInput

	if ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName == nil || len(*ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName) == 0 {
		ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName = &ctx.PolicyName
	}

	// TODO: Meghna Add a similar check for endpoints and variants and also error handling for all requests

	registerScalableTargetInputList = sdkutil.CreateRegisterScalableTargetInputFromSpec(ctx.HostingDeploymentAutoscalingJob.Spec)

	for _, registerScalableTargetInput := range registerScalableTargetInputList {
		if _, err := ctx.ApplicationAutoscalingClient.RegisterScalableTarget(ctx, &registerScalableTargetInput); err != nil {
			return errors.Wrap(err, "Unable to Register Target")
		}
	}

	putScalingPolicyInputList = sdkutil.CreatePutScalingPolicyInputFromSpec(ctx.HostingDeploymentAutoscalingJob.Spec)
	for _, putScalingPolicyInput := range putScalingPolicyInputList {
		if _, err := ctx.ApplicationAutoscalingClient.PutScalingPolicy(ctx, &putScalingPolicyInput); err != nil {
			return errors.Wrap(err, "Unable to Put Scaling Policy")
		}
	}

	//ctx.Log.Info("Input", "err", putScalingPolicyInput)

	return nil
}

// If this function returns an error, the status update has failed, and the reconciler should always requeue.
// This prevents the case where a terminal status fails to persist to the Kubernetes datastore yet we stop
// reconciling and thus leave the job in an unfinished state.
func (r *Reconciler) updateStatus(ctx reconcileRequestContext, hostingDeploymentAutoscalingJobStatus string) error {
	return r.updateStatusWithAdditional(ctx, hostingDeploymentAutoscalingJobStatus, "AdditionalStatus")
}

func (r *Reconciler) updateStatusAndReturnError(ctx reconcileRequestContext, trainingJobPrimaryStatus, trainingJobSecondaryStatus string, reconcileErr error) error {
	if err := r.updateStatusWithAdditional(ctx, trainingJobPrimaryStatus, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

func (r *Reconciler) updateStatusWithAdditional(ctx reconcileRequestContext, hostingDeploymentAutoscalingJobStatus, additional string) error {

	jobStatus := &ctx.HostingDeploymentAutoscalingJob.Status

	// When you call this function, update/refresh all the fields since we overwrite.
	jobStatus.HostingDeploymentAutoscalingJobStatus = hostingDeploymentAutoscalingJobStatus
	jobStatus.Additional = additional
	jobStatus.PolicyName = &ctx.PolicyName

	//TODO: Convert it to tinyurl or even better can we expose CW url via API server proxy UI?

	if err := r.Status().Update(ctx, ctx.HostingDeploymentAutoscalingJob); err != nil {
		err = errors.Wrap(err, "Unable to update status")
		ctx.Log.Info("Error while updating status.", "err", err)
		return err
	}

	return nil
}

// SetupWithManager configures the manager to recognise the controller.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hostingdeploymentautoscalingjobv1.HostingDeploymentAutoscalingJob{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
