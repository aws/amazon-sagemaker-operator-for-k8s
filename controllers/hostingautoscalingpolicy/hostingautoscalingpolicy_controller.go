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

package hostingautoscalingpolicy

import (
	"context"
	"fmt"
	"time"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
	aws "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	hostingautoscalingpolicyv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingautoscalingpolicy"
)

// All the status used by the controller during reconciliation.
// This operator includes two steps. For the rest of the file these are - Step1: RegisterTargets; Step2: PutScalingPolicy
const (
	// The process of creation has started and is in-progress.
	ReconcilingAutoscalingJobStatus = "Reconciling"

	// This Status signifies that the job has been successfully completed for both steps
	CreatedAutoscalingJobStatus = "Created"

	// Could have failed either at step1 or step2
	FailedAutoscalingJobStatus = "Error"

	// This Status will likely not show up, is it needed
	DeletedAutoscalingJobStatus = "Deleted"

	// https://docs.aws.amazon.com/autoscaling/application/APIReference/API_ScalingPolicy.html
	MaxPolicyNameLength = 256

	// Default values for Autoscaling in the SageMaker Service
	ScalableDimension                   = "sagemaker:variant:DesiredInstanceCount"
	PolicyType                          = "TargetTrackingScaling"
	DefaultAutoscalingPolicyName        = "SageMakerEndpointInvocationScalingPolicy"
	DefaultSuspendedStateAttributeValue = false
)

// Reconciler reconciles a HAP object
type Reconciler struct {
	client.Client
	Log                                logr.Logger
	PollInterval                       time.Duration
	createApplicationAutoscalingClient clientwrapper.ApplicationAutoscalingClientWrapperProvider
	awsConfigLoader                    controllers.AWSConfigLoader
}

// NewHostingAutoscalingPolicyReconciler creates a new reconciler with the default ApplicationAutoscaling client.
func NewHostingAutoscalingPolicyReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *Reconciler {
	return &Reconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createApplicationAutoscalingClient: func(cfg aws.Config) clientwrapper.ApplicationAutoscalingClientWrapper {
			sess := controllers.CreateNewAWSSessionFromConfig(cfg)
			return clientwrapper.NewApplicationAutoscalingClientWrapper(applicationautoscaling.New(sess))
		},
		awsConfigLoader: controllers.NewAWSConfigLoader(),
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hostingautoscalingpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hostingautoscalingpolicies/status,verbs=get;update;patch

// Reconcile attempts to reconcile the SageMaker resource state with the k8s desired state.
// TODO: Check if resource name above is correct or plural
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context:                  context.Background(),
		Log:                      r.Log.WithValues("hostingautoscalingpolicy", req.NamespacedName),
		HostingAutoscalingPolicy: new(hostingautoscalingpolicyv1.HostingAutoscalingPolicy),
	}

	ctx.Log.Info("Getting resource")

	// Get state from etcd
	if err := r.Get(ctx, req.NamespacedName, ctx.HostingAutoscalingPolicy); err != nil {
		ctx.Log.Info("Unable to fetch HostingAutoscalingPolicy", "reason", err)
		if apierrs.IsNotFound(err) {
			return controllers.NoRequeue()
		}
		return controllers.RequeueImmediately()
	}

	if err := r.reconcileHostingAutoscalingPolicy(ctx); err != nil {
		ctx.Log.Info("Got an error while reconciling HostingAutoscalingPolicy, will retry", "err", err)
		return controllers.RequeueImmediately()
	}

	return controllers.RequeueAfterInterval(r.PollInterval, nil)
}

type reconcileRequestContext struct {
	context.Context

	Log                          logr.Logger
	ApplicationAutoscalingClient clientwrapper.ApplicationAutoscalingClientWrapper

	// The desired state of the HostingAutoscalingPolicy
	HostingAutoscalingPolicy *hostingautoscalingpolicyv1.HostingAutoscalingPolicy

	// The name the ScalingPolicy that is applied to the Variants
	PolicyName string

	// Each endpoint/variant pair in the spec converted to the string format as expected by the API.
	ResourceIDList []string

	// The current state of the Scaling Policies
	ScalingPolicyDescriptionList  []*applicationautoscaling.ScalingPolicy
	ScalableTargetDescriptionList []*applicationautoscaling.DescribeScalableTargetsOutput
}

// reconcileHostingAutoscalingPolicy initializes, adds finalizer and then determines and calls the HAP action
func (r *Reconciler) reconcileHostingAutoscalingPolicy(ctx reconcileRequestContext) error {
	var err error

	// Set first-touch status
	if ctx.HostingAutoscalingPolicy.Status.HostingAutoscalingPolicyStatus == "" {
		if err = r.updateStatus(ctx, ReconcilingAutoscalingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if it's not marked for deletion.
	if !controllers.HasDeletionTimestamp(ctx.HostingAutoscalingPolicy.ObjectMeta) {
		if !controllers.ContainsString(ctx.HostingAutoscalingPolicy.ObjectMeta.GetFinalizers(), controllers.SageMakerResourceFinalizerName) {
			ctx.HostingAutoscalingPolicy.ObjectMeta.Finalizers = append(ctx.HostingAutoscalingPolicy.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.HostingAutoscalingPolicy); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer added")
		}
	}

	// Update Descriptions in ctx
	if ctx.ScalableTargetDescriptionList, ctx.ScalingPolicyDescriptionList, err = r.describeAutoscalingPolicy(ctx); err != nil && !clientwrapper.IsDescribeHAP404Error(err) {
		return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to describe HostingAutoscalingPolicy."))
	}

	var action controllers.ReconcileAction
	if action, err = r.determineActionForAutoscaling(ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to determine action for HostingAutoscalingPolicy."))
	}
	ctx.Log.Info("Determined action for AutoscalingJob", "action", action)

	// If update or delete, delete the existing Policy.
	if action == controllers.NeedsDelete || action == controllers.NeedsUpdate {
		if err = r.deleteAutoscalingPolicy(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to delete HostingAutoscalingPolicy"))
		}

		// Delete succeeded, set Description, ResourceIDList to nil.
		ctx.ScalingPolicyDescriptionList = nil
		ctx.ScalableTargetDescriptionList = nil
		ctx.HostingAutoscalingPolicy.Status.ResourceIDList = []string{}
	}

	// If update or create, create the desired HAP.
	if action == controllers.NeedsCreate || action == controllers.NeedsUpdate {
		if ctx.ScalableTargetDescriptionList, ctx.ScalingPolicyDescriptionList, err = r.applyAutoscalingPolicy(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to apply HostingAutoscalingPolicy"))
		}

		// If create succeeded, save the resourceIDs before next spec update.
		r.saveCurrentResourceIDsToStatus(&ctx)
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
	if controllers.HasDeletionTimestamp(ctx.HostingAutoscalingPolicy.ObjectMeta) {
		if err = r.removeFinalizer(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Removes the finalizer held by our controller.
func (r *Reconciler) removeFinalizer(ctx reconcileRequestContext) error {
	var err error

	ctx.HostingAutoscalingPolicy.ObjectMeta.Finalizers = controllers.RemoveString(ctx.HostingAutoscalingPolicy.ObjectMeta.Finalizers, controllers.SageMakerResourceFinalizerName)
	if err = r.Update(ctx, ctx.HostingAutoscalingPolicy); err != nil {
		return errors.Wrap(err, "Failed to remove finalizer")
	}
	ctx.Log.Info("Finalizer has been removed")

	return nil
}

// getResourceIDListfromInputSpec converts the list of resources into a string list to be used for various API calls
func (r *Reconciler) getResourceIDListfromInputSpec(ctx *reconcileRequestContext) error {

	resourceIDListfromSpec := ctx.HostingAutoscalingPolicy.Spec.ResourceID
	for _, resourceIDfromSpec := range resourceIDListfromSpec {
		resourceID := sdkutil.ConvertAutoscalingResourceToString(*resourceIDfromSpec)
		ctx.ResourceIDList = append(ctx.ResourceIDList, *resourceID)
	}

	return nil
}

// Initialize fields on the context object which will be used later.
func (r *Reconciler) initializeContext(ctx *reconcileRequestContext) error {
	var err error

	// Ensure we are using the job name specified in the spec
	if ctx.HostingAutoscalingPolicy.Spec.PolicyName != nil && len(*ctx.HostingAutoscalingPolicy.Spec.PolicyName) > 0 {
		ctx.PolicyName = *ctx.HostingAutoscalingPolicy.Spec.PolicyName
	} else {
		ctx.PolicyName = DefaultAutoscalingPolicyName
		ctx.HostingAutoscalingPolicy.Spec.PolicyName = &ctx.PolicyName

		if err := r.Update(ctx, ctx.HostingAutoscalingPolicy); err != nil {
			ctx.Log.Info("Error while updating HostingAutoscalingPolicy policyName in spec")
			return err
		}
	}

	// Save the ResourceIDs into ctx as usable strings
	if err = r.getResourceIDListfromInputSpec(ctx); err != nil {
		ctx.Log.Error(err, "Error reading the ResourceIDs from Spec")
		return err
	}

	if ctx.HostingAutoscalingPolicy.Status.ResourceIDList == nil {
		ctx.HostingAutoscalingPolicy.Status.ResourceIDList = []string{}
	}

	// Initialize other values to defaults if not in Spec
	if ctx.HostingAutoscalingPolicy.Spec.ScalableDimension == nil {
		namespace := sdkutil.HostingAutoscalingPolicyServiceNamespace
		ctx.HostingAutoscalingPolicy.Spec.ServiceNamespace = &namespace
	}

	if ctx.HostingAutoscalingPolicy.Spec.ScalableDimension == nil {
		dimension := ScalableDimension
		ctx.HostingAutoscalingPolicy.Spec.ScalableDimension = &dimension
	}

	if ctx.HostingAutoscalingPolicy.Spec.PolicyType == nil {
		policyType := PolicyType
		ctx.HostingAutoscalingPolicy.Spec.PolicyType = &policyType
	}

	if ctx.HostingAutoscalingPolicy.Spec.SuspendedState == nil {
		ctx.HostingAutoscalingPolicy.Spec.SuspendedState = &commonv1.HAPSuspendedState{}
	}

	if ctx.HostingAutoscalingPolicy.Spec.SuspendedState.DynamicScalingInSuspended == nil {
		ctx.HostingAutoscalingPolicy.Spec.SuspendedState.DynamicScalingInSuspended = controllertest.ToBoolPtr(DefaultSuspendedStateAttributeValue)
	}

	if ctx.HostingAutoscalingPolicy.Spec.SuspendedState.DynamicScalingOutSuspended == nil {
		ctx.HostingAutoscalingPolicy.Spec.SuspendedState.DynamicScalingOutSuspended = controllertest.ToBoolPtr(DefaultSuspendedStateAttributeValue)
	}

	if ctx.HostingAutoscalingPolicy.Spec.SuspendedState.ScheduledScalingSuspended == nil {
		ctx.HostingAutoscalingPolicy.Spec.SuspendedState.ScheduledScalingSuspended = controllertest.ToBoolPtr(DefaultSuspendedStateAttributeValue)
	}

	awsConfig, err := r.awsConfigLoader.LoadAWSConfigWithOverrides(ctx.HostingAutoscalingPolicy.Spec.Region, ctx.HostingAutoscalingPolicy.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.ApplicationAutoscalingClient = r.createApplicationAutoscalingClient(awsConfig)
	ctx.Log.Info("Loaded AWS config")

	return nil
}

// saveCurrentResourceIDsToStatus before updating the ResourceIDs
func (r *Reconciler) saveCurrentResourceIDsToStatus(ctx *reconcileRequestContext) {
	ctx.HostingAutoscalingPolicy.Status.ResourceIDList = ctx.ResourceIDList
}

// determineActionForAutoscaling checks if controller needs to create/delete/update HAP
func (r *Reconciler) determineActionForAutoscaling(ctx reconcileRequestContext) (controllers.ReconcileAction, error) {
	var err error
	if controllers.HasDeletionTimestamp(ctx.HostingAutoscalingPolicy.ObjectMeta) {
		ctx.Log.Info("Object Has Deletion Timestamp")
		// Both conditions are needed because if apply fails, its possible only registering target was done.
		if len(ctx.ScalingPolicyDescriptionList) > 0 || len(ctx.ScalableTargetDescriptionList) > 0 {
			return controllers.NeedsDelete, nil
		}
		return controllers.NeedsNoop, nil
	}

	// TODO: one condition should be enough since the object is initialized
	if ctx.ScalingPolicyDescriptionList == nil || len(ctx.ScalingPolicyDescriptionList) == 0 {
		return controllers.NeedsCreate, nil
	}

	var comparison sdkutil.Comparison
	if comparison, err = sdkutil.HostingAutoscalingPolicySpecMatchesDescription(ctx.ScalableTargetDescriptionList, ctx.ScalingPolicyDescriptionList, ctx.HostingAutoscalingPolicy.Spec, ctx.HostingAutoscalingPolicy.Status.ResourceIDList); err != nil {
		return controllers.NeedsNoop, err
	}

	r.Log.Info("Compared existing HAP to actual HAP to determine if HAP needs to be updated.", "differences", comparison.Differences, "is-equal", comparison.Equal)

	if comparison.Equal {
		return controllers.NeedsNoop, nil
	}

	return controllers.NeedsUpdate, nil
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
			return scalableTargetDescriptionList, scalingPolicyDescriptionList, errors.Wrap(err, "Unable to describe ScalableTarget")
		}

		if scalingPolicyDescription, err = ctx.ApplicationAutoscalingClient.DescribeScalingPolicies(ctx, ctx.PolicyName, ResourceID); err != nil {
			return scalableTargetDescriptionList, scalingPolicyDescriptionList, errors.Wrap(err, "Unable to describe ScalingPolicy")
		}

		if scalableTargetDescription != nil {
			scalableTargetDescriptionList = append(scalableTargetDescriptionList, scalableTargetDescription)
		}
		if scalingPolicyDescription != nil {
			scalingPolicyDescriptionList = append(scalingPolicyDescriptionList, scalingPolicyDescription)
		}
	}
	return scalableTargetDescriptionList, scalingPolicyDescriptionList, nil
}

// deleteAutoscalingPolicy converts Spec to Input, Registers Target, Creates second input, applies the scalingPolicy
// For deletion, take the resourceIDList based on the current status which is updated after creation. This is needed to ensure update works as expected.
func (r *Reconciler) deleteAutoscalingPolicy(ctx reconcileRequestContext) error {
	var deregisterScalableTargetInput applicationautoscaling.DeregisterScalableTargetInput
	var deleteScalingPolicyInput applicationautoscaling.DeleteScalingPolicyInput

	// For delete if Object is not found, don't throw an error.
	for _, ResourceID := range ctx.HostingAutoscalingPolicy.Status.ResourceIDList {
		deleteScalingPolicyInput = sdkutil.CreateDeleteScalingPolicyInput(ctx.HostingAutoscalingPolicy.Spec, ResourceID)
		if _, err := ctx.ApplicationAutoscalingClient.DeleteScalingPolicy(ctx, &deleteScalingPolicyInput); err != nil && !clientwrapper.IsDeleteHAP404Error(err) {
			return errors.Wrap(err, "Unable to DeleteScalingPolicy")
		}
	}

	for _, ResourceID := range ctx.HostingAutoscalingPolicy.Status.ResourceIDList {
		deregisterScalableTargetInput = sdkutil.CreateDeregisterScalableTargetInput(ctx.HostingAutoscalingPolicy.Spec, ResourceID)
		if _, err := ctx.ApplicationAutoscalingClient.DeregisterScalableTarget(ctx, &deregisterScalableTargetInput); err != nil && !clientwrapper.IsDeleteHAP404Error(err) {
			return errors.Wrap(err, "Unable to DeregisterScalableTarget")
		}
	}

	return nil
}

// applyAutoscalingPolicy converts Spec to Input, Registers Target, Creates scalingPolicy input, applies the scalingPolicy
// This method needs to be broken down
func (r *Reconciler) applyAutoscalingPolicy(ctx reconcileRequestContext) ([]*applicationautoscaling.DescribeScalableTargetsOutput, []*applicationautoscaling.ScalingPolicy, error) {
	var registerScalableTargetInputList []applicationautoscaling.RegisterScalableTargetInput
	var putScalingPolicyInputList []applicationautoscaling.PutScalingPolicyInput

	// For describe
	var scalableTargetDescriptionList []*applicationautoscaling.DescribeScalableTargetsOutput
	var scalingPolicyDescriptionList []*applicationautoscaling.ScalingPolicy

	if ctx.HostingAutoscalingPolicy.Spec.PolicyName == nil || len(*ctx.HostingAutoscalingPolicy.Spec.PolicyName) == 0 {
		ctx.HostingAutoscalingPolicy.Spec.PolicyName = &ctx.PolicyName
	}

	// For each resourceID, register the scalableTarget
	registerScalableTargetInputList = sdkutil.CreateRegisterScalableTargetInputFromSpec(ctx.HostingAutoscalingPolicy.Spec)
	for _, registerScalableTargetInput := range registerScalableTargetInputList {
		if _, err := ctx.ApplicationAutoscalingClient.RegisterScalableTarget(ctx, &registerScalableTargetInput); err != nil {
			return scalableTargetDescriptionList, scalingPolicyDescriptionList, errors.Wrap(err, "Unable to Register Target")
		}
	}

	// For each resourceID, apply the scalingPolicy
	putScalingPolicyInputList = sdkutil.CreatePutScalingPolicyInputFromSpec(ctx.HostingAutoscalingPolicy.Spec)
	for _, putScalingPolicyInput := range putScalingPolicyInputList {
		if _, err := ctx.ApplicationAutoscalingClient.PutScalingPolicy(ctx, &putScalingPolicyInput); err != nil {
			return scalableTargetDescriptionList, scalingPolicyDescriptionList, errors.Wrap(err, "Unable to Put Scaling Policy")
		}
	}

	var err error
	if scalableTargetDescriptionList, scalingPolicyDescriptionList, err = r.describeAutoscalingPolicy(ctx); err != nil {
		return scalableTargetDescriptionList, scalingPolicyDescriptionList, r.updateStatusAndReturnError(ctx, errors.Wrap(err, "Unable to describe HostingAutoscalingPolicy."))
	}

	// TODO mbaijal: This check is not needed since the first describe is handled differently
	if len(scalableTargetDescriptionList) == 0 || len(scalingPolicyDescriptionList) == 0 {
		return nil, nil, fmt.Errorf("hosting autoscaling policy was not applied, description is empty")
	}

	return scalableTargetDescriptionList, scalingPolicyDescriptionList, nil
}

// If this function returns an error, the status update has failed, and the reconciler should always requeue.
// This prevents the case where a terminal status fails to persist to the Kubernetes datastore yet we stop
// reconciling and thus leave the job in an unfinished state.
func (r *Reconciler) updateStatus(ctx reconcileRequestContext, hostingAutoscalingPolicyStatus string) error {
	return r.updateStatusWithAdditional(ctx, hostingAutoscalingPolicyStatus, "")
}

// Check the error code and update to error/reconciling status accordingly
func (r *Reconciler) updateStatusAndReturnError(ctx reconcileRequestContext, reconcileErr error) error {

	status := FailedAutoscalingJobStatus
	if clientwrapper.IsHAPInternalServiceExceptionError(reconcileErr) || clientwrapper.IsHAPConcurrentUpdateExceptionError(reconcileErr) || clientwrapper.IsHDPendingError(reconcileErr) {
		status = ReconcilingAutoscalingJobStatus
	}

	if err := r.updateStatusWithAdditional(ctx, status, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

func (r *Reconciler) updateStatusWithAdditional(ctx reconcileRequestContext, hostingAutoscalingPolicyStatus, additional string) error {

	jobStatus := &ctx.HostingAutoscalingPolicy.Status

	// When you call this function, update/refresh all the fields since we overwrite.
	jobStatus.HostingAutoscalingPolicyStatus = hostingAutoscalingPolicyStatus
	jobStatus.Additional = additional
	jobStatus.PolicyName = ctx.PolicyName

	if err := r.Status().Update(ctx, ctx.HostingAutoscalingPolicy); err != nil {
		err = errors.Wrap(err, "Unable to update status")
		ctx.Log.Info("Error while updating status.", "err", err)
		return err
	}

	return nil
}

// SetupWithManager configures the manager to recognise the controller.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hostingautoscalingpolicyv1.HostingAutoscalingPolicy{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
