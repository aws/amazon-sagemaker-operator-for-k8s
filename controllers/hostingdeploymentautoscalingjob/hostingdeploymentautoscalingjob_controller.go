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

	hostingdeploymentautoscalingjobv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingdeploymentautoscalingjob"
)

// All the status used by the controller during reconciliation.
// This operator includes two steps. For the rest of the file these are - Step1: RegisterTargets; Step2: PutScalingPolicy
const (
	// The process of creation has started and is in-progress.
	ReconcilingAutoscalingJobStatus = "ReconcilingAutoscalingJob"

	// This Status signifies that the job has been successfully completed for both steps
	CreatedAutoscalingJobStatus = "CreatedAutoscalingJob"

	// Only the first step completed, after this it could either go to the Failed, Reconciling or Created Status
	// TODO this is not being used at the moment, delete if not required
	RegisteredTargetsJobStatus = "RegisteredTargets"

	// Could have failed either at step1 or step2
	FailedAutoscalingJobStatus = "FailedAutoscalingJob"

	// This Status will likely not show up, is it needed
	DeletedAutoscalingJobStatus = "DeletedAutoscalingJob"

	// https://docs.aws.amazon.com/autoscaling/application/APIReference/API_ScalingPolicy.html
	MaxPolicyNameLength = 256

	// Default values for Autoscaling in the SageMaker Service
	ScalableDimension            = "sagemaker:variant:DesiredInstanceCount"
	PolicyType                   = "TargetTrackingScaling"
	DefaultAutoscalingPolicyName = "SageMakerEndpointInvocationScalingPolicy"
)

// Reconciler reconciles a HostingDeploymentAutoscalingJob object
type Reconciler struct {
	client.Client
	Log                                logr.Logger
	createApplicationAutoscalingClient clientwrapper.ApplicationAutoscalingClientWrapperProvider
	awsConfigLoader                    controllers.AwsConfigLoader
}

// NewHostingDeploymentAutoscalingJobReconciler creates a new reconciler with the default ApplicationAutoscaling client.
func NewHostingDeploymentAutoscalingJobReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client: client,
		Log:    log,
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
// Review: Clean this up
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
}

type reconcileRequestContext struct {
	context.Context

	Log                          logr.Logger
	ApplicationAutoscalingClient clientwrapper.ApplicationAutoscalingClientWrapper

	// The desired state of the HostingDeploymentAutoscalingJob
	HostingDeploymentAutoscalingJob *hostingdeploymentautoscalingjobv1.HostingDeploymentAutoscalingJob

	// The name the ScalingPolicy that is applied to the Variants
	PolicyName string

	// Each endpoint/variant pair in the spec converted to the string format as expected by the API.
	ResourceIDList []string

	// The current state of the Scaling Policies
	ScalingPolicyDescriptionList  []*applicationautoscaling.ScalingPolicy
	ScalableTargetDescriptionList []*applicationautoscaling.DescribeScalableTargetsOutput
}

// reconcileHostingDeploymentAutoscalingJob initialized the
func (r *Reconciler) reconcileHostingDeploymentAutoscalingJob(ctx reconcileRequestContext) error {
	var err error

	// Set first-touch status
	if ctx.HostingDeploymentAutoscalingJob.Status.HostingDeploymentAutoscalingJobStatus == "" {
		if err = r.updateStatus(ctx, controllers.InitializingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if it's not marked for deletion.
	if !controllers.HasDeletionTimestamp(ctx.HostingDeploymentAutoscalingJob.ObjectMeta) {
		if !controllers.ContainsString(ctx.HostingDeploymentAutoscalingJob.ObjectMeta.GetFinalizers(), controllers.SageMakerResourceFinalizerName) {
			ctx.HostingDeploymentAutoscalingJob.ObjectMeta.Finalizers = append(ctx.HostingDeploymentAutoscalingJob.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.HostingDeploymentAutoscalingJob); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer added")
		}
	}

	// Update Descriptions in ctx
	if ctx.ScalableTargetDescriptionList, ctx.ScalingPolicyDescriptionList, err = r.describeAutoscalingPolicy(ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, errors.Wrap(err, "Unable to describe HostingDeploymentAutoscaling."))
	}
	ctx.Log.Info("ScalingPolicyDescription", "err", ctx.ScalingPolicyDescriptionList)

	var action controllers.ReconcileAction
	if action, err = r.determineActionForAutoscaling(ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, errors.Wrap(err, "Unable to determine action for HostingDeploymentAutoscaling."))
	}
	ctx.Log.Info("Determined action for AutoscalingJob", "action", action)

	// If update or delete, delete the existing Policy.
	if action == controllers.NeedsDelete || action == controllers.NeedsUpdate {
		if err = r.deleteAutoscalingPolicy(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, errors.Wrap(err, "Unable to deleteAutoscalingPolicy"))
		}

		// Delete succeeded, set Description to nil.
		ctx.ScalingPolicyDescriptionList = nil
		ctx.ScalableTargetDescriptionList = nil
	}

	// If update or create, create the desired hostingdeploymentautoscaling.
	if action == controllers.NeedsCreate || action == controllers.NeedsUpdate {
		if err = r.applyAutoscalingPolicy(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, FailedAutoscalingJobStatus, errors.Wrap(err, "Unable to applyAutoscalingPolicy"))
		}
	}

	// Update the status accordingly.
	// Review: Deleted Status may be unnecessary
	status := CreatedAutoscalingJobStatus
	if ctx.ScalingPolicyDescriptionList == nil {
		status = DeletedAutoscalingJobStatus
	}
	if err = r.updateStatus(ctx, status); err != nil {
		return err
	}

	// Remove the Finalizer on delete
	if controllers.HasDeletionTimestamp(ctx.HostingDeploymentAutoscalingJob.ObjectMeta) {
		if err = r.removeFinalizer(ctx); err != nil {
			return err
		}
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

// getResourceIDListfromInputSpec converts the list of resources into a string list to be used for various API calls
func (r *Reconciler) getResourceIDListfromInputSpec(ctx *reconcileRequestContext) error {

	resourceIDListfromSpec := ctx.HostingDeploymentAutoscalingJob.Spec.ResourceID
	for _, resourceIDfromSpec := range resourceIDListfromSpec {
		ResourceID := sdkutil.ConvertAutoscalingResourceToString(*resourceIDfromSpec)
		ctx.ResourceIDList = append(ctx.ResourceIDList, *ResourceID)
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
		ctx.PolicyName = DefaultAutoscalingPolicyName
		ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName = &ctx.PolicyName

		// Review: Is this needed
		if err := r.Update(ctx, ctx.HostingDeploymentAutoscalingJob); err != nil {
			ctx.Log.Info("Error while updating HostingDeploymentAutoscalingJob policyName in spec")
			return err
		}
	}

	// Save the ResourceIDs into ctx as usable strings
	if err = r.getResourceIDListfromInputSpec(ctx); err != nil {
		ctx.Log.Error(err, "Error reading the ResourceIDs from Spec")
		return err
	}

	// Initialize other values to defaults if not in Spec
	if ctx.HostingDeploymentAutoscalingJob.Spec.ScalableDimension == nil {
		namespace := sdkutil.HostingDeploymentAutoscalingServiceNamespace
		ctx.HostingDeploymentAutoscalingJob.Spec.ServiceNamespace = &namespace
	}

	if ctx.HostingDeploymentAutoscalingJob.Spec.ScalableDimension == nil {
		dimension := ScalableDimension
		ctx.HostingDeploymentAutoscalingJob.Spec.ScalableDimension = &dimension
	}

	if ctx.HostingDeploymentAutoscalingJob.Spec.PolicyType == nil {
		policyType := PolicyType
		ctx.HostingDeploymentAutoscalingJob.Spec.PolicyType = &policyType
	}

	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.HostingDeploymentAutoscalingJob.Spec.Region, ctx.HostingDeploymentAutoscalingJob.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.ApplicationAutoscalingClient = r.createApplicationAutoscalingClient(awsConfig)
	ctx.Log.Info("Loaded AWS config")

	return nil
}

// ctx.HostingDeploymentAutoscalingJob: desiredAutoscaling
// ctx.ScalingPolicyDescription: actualAutoscaling
func (r *Reconciler) determineActionForAutoscaling(ctx reconcileRequestContext) (controllers.ReconcileAction, error) {
	var err error
	if controllers.HasDeletionTimestamp(ctx.HostingDeploymentAutoscalingJob.ObjectMeta) {
		ctx.Log.Info("Object Has Deletion Timestamp")
		if len(ctx.ScalingPolicyDescriptionList) > 0 {
			return controllers.NeedsDelete, nil
		}
		return controllers.NeedsNoop, nil
	}

	//Review: Second condition is not needed, cleanup after test
	if ctx.ScalingPolicyDescriptionList == nil || len(ctx.ScalingPolicyDescriptionList) == 0 {
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

// describeAutoscalingPolicy adds current descriptions to the context for each resource in the spec list
func (r *Reconciler) describeAutoscalingPolicy(ctx reconcileRequestContext) ([]*applicationautoscaling.DescribeScalableTargetsOutput, []*applicationautoscaling.ScalingPolicy, error) {
	var err error

	var scalingPolicyDescriptionList []*applicationautoscaling.ScalingPolicy
	var scalableTargetDescriptionList []*applicationautoscaling.DescribeScalableTargetsOutput

	for _, ResourceID := range ctx.ResourceIDList {
		var scalableTargetDescription *applicationautoscaling.DescribeScalableTargetsOutput
		var scalingPolicyDescription *applicationautoscaling.ScalingPolicy

		if scalableTargetDescription, err = ctx.ApplicationAutoscalingClient.DescribeScalableTargets(ctx, ResourceID); err != nil {
			return scalableTargetDescriptionList, scalingPolicyDescriptionList, r.updateStatusAndReturnError(ctx, ReconcilingAutoscalingJobStatus, errors.Wrap(err, "Unable to describe ScalableTarget"))
		}

		if scalingPolicyDescription, err = ctx.ApplicationAutoscalingClient.DescribeScalingPolicies(ctx, ctx.PolicyName, ResourceID); err != nil {
			return scalableTargetDescriptionList, scalingPolicyDescriptionList, r.updateStatusAndReturnError(ctx, ReconcilingAutoscalingJobStatus, errors.Wrap(err, "Unable to describe ScalingPolicy"))
		}

		if scalableTargetDescription != nil {
			scalableTargetDescriptionList = append(ctx.ScalableTargetDescriptionList, scalableTargetDescription)
		}
		if scalingPolicyDescription != nil {
			scalingPolicyDescriptionList = append(scalingPolicyDescriptionList, scalingPolicyDescription)
		}
	}
	return scalableTargetDescriptionList, scalingPolicyDescriptionList, nil
}

// deleteAutoscalingPolicy converts Spec to Input, Registers Target, Creates second input, applies the scalingPolicy
// same as reconcileCreation of model
func (r *Reconciler) deleteAutoscalingPolicy(ctx reconcileRequestContext) error {
	var deregisterScalableTargetInput applicationautoscaling.DeregisterScalableTargetInput
	var deleteScalingPolicyInput applicationautoscaling.DeleteScalingPolicyInput

	for _, ResourceID := range ctx.ResourceIDList {
		deleteScalingPolicyInput = sdkutil.CreateDeleteScalingPolicyInput(ctx.HostingDeploymentAutoscalingJob.Spec, ResourceID)
		if _, err := ctx.ApplicationAutoscalingClient.DeleteScalingPolicy(ctx, &deleteScalingPolicyInput); err != nil {
			return errors.Wrap(err, "Unable to DeleteScalingPolicy")
		}
	}

	for _, ResourceID := range ctx.ResourceIDList {
		deregisterScalableTargetInput = sdkutil.CreateDeregisterScalableTargetInput(ctx.HostingDeploymentAutoscalingJob.Spec, ResourceID)
		if _, err := ctx.ApplicationAutoscalingClient.DeregisterScalableTarget(ctx, &deregisterScalableTargetInput); err != nil {
			return errors.Wrap(err, "Unable DeregisterScalableTarget")
		}
	}

	return nil
}

// applyAutoscalingPolicy converts Spec to Input, Registers Target, Creates scalingPolicy input, applies the scalingPolicy
func (r *Reconciler) applyAutoscalingPolicy(ctx reconcileRequestContext) error {
	var registerScalableTargetInputList []applicationautoscaling.RegisterScalableTargetInput
	var putScalingPolicyInputList []applicationautoscaling.PutScalingPolicyInput

	// Review: Is this needed
	if ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName == nil || len(*ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName) == 0 {
		ctx.HostingDeploymentAutoscalingJob.Spec.PolicyName = &ctx.PolicyName
	}

	// For each resourceID, register the scalableTarget
	registerScalableTargetInputList = sdkutil.CreateRegisterScalableTargetInputFromSpec(ctx.HostingDeploymentAutoscalingJob.Spec)
	for _, registerScalableTargetInput := range registerScalableTargetInputList {
		if _, err := ctx.ApplicationAutoscalingClient.RegisterScalableTarget(ctx, &registerScalableTargetInput); err != nil {
			return errors.Wrap(err, "Unable to Register Target")
		}
	}

	// For each resourceID, apply the scalingPolicy
	putScalingPolicyInputList = sdkutil.CreatePutScalingPolicyInputFromSpec(ctx.HostingDeploymentAutoscalingJob.Spec)
	for _, putScalingPolicyInput := range putScalingPolicyInputList {
		if _, err := ctx.ApplicationAutoscalingClient.PutScalingPolicy(ctx, &putScalingPolicyInput); err != nil {
			return errors.Wrap(err, "Unable to Put Scaling Policy")
		}
	}

	return nil
}

// If this function returns an error, the status update has failed, and the reconciler should always requeue.
// This prevents the case where a terminal status fails to persist to the Kubernetes datastore yet we stop
// reconciling and thus leave the job in an unfinished state.
func (r *Reconciler) updateStatus(ctx reconcileRequestContext, hostingDeploymentAutoscalingJobStatus string) error {
	return r.updateStatusWithAdditional(ctx, hostingDeploymentAutoscalingJobStatus, "")
}

func (r *Reconciler) updateStatusAndReturnError(ctx reconcileRequestContext, status string, reconcileErr error) error {
	if err := r.updateStatusWithAdditional(ctx, status, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

func (r *Reconciler) updateStatusWithAdditional(ctx reconcileRequestContext, hostingDeploymentAutoscalingJobStatus, additional string) error {

	jobStatus := &ctx.HostingDeploymentAutoscalingJob.Status

	// When you call this function, update/refresh all the fields since we overwrite.
	jobStatus.HostingDeploymentAutoscalingJobStatus = hostingDeploymentAutoscalingJobStatus
	jobStatus.Additional = additional
	jobStatus.PolicyName = ctx.PolicyName

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
