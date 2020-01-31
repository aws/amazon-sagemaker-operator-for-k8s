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

package hosting

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"

	hostingv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingdeployment"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
)

const (
	ReconcilingEndpointStatus = "ReconcilingEndpoint"
)

// HostingDeploymentReconciler reconciles a HostingDeployment object
type HostingDeploymentReconciler struct {
	client.Client
	Log          logr.Logger
	PollInterval time.Duration

	awsConfigLoader                AwsConfigLoader
	createModelReconciler          ModelReconcilerProvider
	createEndpointConfigReconciler EndpointConfigReconcilerProvider
	createSageMakerClient          SageMakerClientProvider
}

func NewHostingDeploymentReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *HostingDeploymentReconciler {
	return &HostingDeploymentReconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createSageMakerClient: func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			return sagemaker.New(awsConfig)
		},
		awsConfigLoader:                NewAwsConfigLoader(),
		createModelReconciler:          NewModelReconciler,
		createEndpointConfigReconciler: NewEndpointConfigReconciler,
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hostingdeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hostingdeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=models/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=endpointconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=endpointconfigs/status,verbs=get;update;patch

func (r *HostingDeploymentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context:    context.Background(),
		Log:        r.Log.WithValues("hostingdeployment", req.NamespacedName),
		Deployment: new(hostingv1.HostingDeployment),
	}

	// Get state from etcd
	if err := r.Get(ctx, req.NamespacedName, ctx.Deployment); err != nil {
		ctx.Log.Info("Unable to fetch HostingDeployment", "reason", err)

		if apierrs.IsNotFound(err) {
			return NoRequeue()
		} else {
			return RequeueImmediately()
		}
	}

	if err := r.reconcileHostingDeployment(ctx); err != nil {
		// TODO stack traces are not printed well.
		ctx.Log.Info("Got error while reconciling, will retry", "err", err)
		return RequeueImmediately()
	} else {
		return RequeueAfterInterval(r.PollInterval, nil)
	}

}

type reconcileRequestContext struct {
	context.Context

	Log                      logr.Logger
	SageMakerClient          clientwrapper.SageMakerClientWrapper
	ModelReconciler          ModelReconciler
	EndpointConfigReconciler EndpointConfigReconciler

	// The desired state of the HostingDeployment
	Deployment *hostingv1.HostingDeployment

	// The SageMaker Endpoint description.
	EndpointDescription *sagemaker.DescribeEndpointOutput

	// The name of the SageMaker Endpoint.
	EndpointName string

	// A map of k8s model names to their SageMaker model names.
	ModelNames map[string]string

	// The name of the SageMaker EndpointConfig
	EndpointConfigName string
}

// Reconcile a HostingDeployment, creating, deleting or updating a SageMaker Endpoint as necessary.
// SageMaker softly requires that corresponding EndpointConfig and Models exist during the lifetime of
// an Endpoint. During Endpoint updates, both the existing set of EndpointConfig/Models and the new,
// desired set of Endpoint/Models must exist for the entire duration of the update (~10 minutes x2).
// This function thus creates necessary resources (EndpointConfig+Models) before creating the Endpoint,
// and deletes the corresponding resources after the Endpoint is deleted.
// Updates are a special case. Updates are only supported when the Endpoint is InService. When an update
// occurs, this creates a new set of EndpointConfig/Model that lives alongside the old set of
// EndpointConfig/Model for the duration of the endpoint update. Once the update is complete, and the
// status is InService again, the old resources are deleted.
//
// See also:
// UpdateEndpoint https://docs.aws.amazon.com/sagemaker/latest/dg/API_UpdateEndpoint.html
// DescribeEndpoint https://docs.aws.amazon.com/sagemaker/latest/dg/API_DescribeEndpoint.html
func (r *HostingDeploymentReconciler) reconcileHostingDeployment(ctx reconcileRequestContext) error {

	var err error
	// Set first-touch status.
	if ctx.Deployment.Status.EndpointStatus == "" {
		if err = r.updateStatus(ctx, InitializingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, string(sagemaker.EndpointStatusFailed), errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if it's not marked for deletion.
	if !HasDeletionTimestamp(ctx.Deployment.ObjectMeta) {
		if !ContainsString(ctx.Deployment.ObjectMeta.GetFinalizers(), SageMakerResourceFinalizerName) {
			ctx.Deployment.ObjectMeta.Finalizers = append(ctx.Deployment.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.Deployment); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer added")
		}
	}

	// Get the EndpointConfigName so that status updates can show it.
	// If there is an error, ignore it.
	if ctx.EndpointConfigName, err = ctx.EndpointConfigReconciler.GetSageMakerEndpointConfigName(ctx, ctx.Deployment); err != nil {
		r.Log.Info("Unable to get EndpointConfigName", "err", err)
	}

	// Get the SageMaker model names so that status updates can show them.
	// If there is an error, ignore it.
	if ctx.ModelNames, err = ctx.ModelReconciler.GetSageMakerModelNames(ctx, ctx.Deployment); err != nil {
		r.Log.Info("Unable to get model names", "err", err)
	}

	// Get the EndpointDescription from SageMaker.
	if ctx.EndpointDescription, err = ctx.SageMakerClient.DescribeEndpoint(ctx, ctx.EndpointName); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingEndpointStatus, errors.Wrap(err, "Unable to describe SageMaker endpoint"))
	}

	// The Endpoint does not exist. If the HostingDeployment needs to be deleted, then delete.
	// Otherwise, create the Endpoint.
	if ctx.EndpointDescription == nil {

		if HasDeletionTimestamp(ctx.Deployment.ObjectMeta) {
			return r.cleanupAndRemoveFinalizer(ctx)
		}

		if err = r.createEndpoint(ctx); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingEndpointStatus, errors.Wrap(err, "Unable to create Endpoint"))
		}

		// Get description and continue
		if ctx.EndpointDescription, err = ctx.SageMakerClient.DescribeEndpoint(ctx, ctx.EndpointName); err != nil {
			return r.updateStatusAndReturnError(ctx, ReconcilingEndpointStatus, errors.Wrap(err, "Unable to describe SageMaker endpoint"))
		}
	}

	// Updates and deletions are only supported in SageMaker when the Endpoint is "InService" (update or deletion) or "Failed" (only deletion).
	// Thus, gate the updates/deletes according to status.
	switch ctx.EndpointDescription.EndpointStatus {
	case sagemaker.EndpointStatusInService:

		// Only do updates if the object is not marked as deleted.
		if !HasDeletionTimestamp(ctx.Deployment.ObjectMeta) {
			if err = r.handleUpdates(ctx); err != nil {
				return r.updateStatusAndReturnError(ctx, ReconcilingEndpointStatus, errors.Wrap(err, "Unable to update SageMaker endpoint"))
			}
		}

		// Handle deletion by falling through.
		fallthrough
	case sagemaker.EndpointStatusFailed:
		if HasDeletionTimestamp(ctx.Deployment.ObjectMeta) {
			if _, err := ctx.SageMakerClient.DeleteEndpoint(ctx, &ctx.EndpointName); err != nil && !clientwrapper.IsDeleteEndpoint404Error(err) {
				return r.updateStatusAndReturnError(ctx, ReconcilingEndpointStatus, errors.Wrap(err, "Unable to delete Endpoint"))
			}
		}
		break
	case sagemaker.EndpointStatusCreating:
		fallthrough
	case sagemaker.EndpointStatusDeleting:
		fallthrough
	case sagemaker.EndpointStatusOutOfService:
		fallthrough
	case sagemaker.EndpointStatusRollingBack:
		fallthrough
	case sagemaker.EndpointStatusSystemUpdating:
		fallthrough
	case sagemaker.EndpointStatusUpdating:
		// The status will be updated after the switch statement.
		r.Log.Info("Noop action, endpoint status does not allow modifications", "status", ctx.EndpointDescription.EndpointStatus)
	}

	status := string(ctx.EndpointDescription.EndpointStatus)

	// Present to the user that the endpoint is being deleted after they delete the hosting deployment.
	if HasDeletionTimestamp(ctx.Deployment.ObjectMeta) && ctx.EndpointDescription.EndpointStatus != sagemaker.EndpointStatusDeleting {
		status = string(sagemaker.EndpointStatusDeleting)
	}

	if err = r.updateStatus(ctx, status); err != nil {
		return err
	}

	return nil
}

// Start an Endpoint update. If the HostingDeployment spec was updated, this will create a new
// set of models and endpointconfigs in SageMaker. If the endpoint's config name is different than
// the new config name, then invoke UpdateEndpoint.
// If no update is necessary, this cleans up unused resources (like those from a previously finished update).
// TODO Currently if a user edits the HostingDeployment (which causes an error), then reverts their bad edit, an update will be triggered.
// This is because the updates are determined by a change in generation number. We should ignore updates when the endpoint config is deep equal.
func (r *HostingDeploymentReconciler) handleUpdates(ctx reconcileRequestContext) error {
	var err error

	// If the specs changed, then this will create new models and endpoint configs.
	if err = r.reconcileEndpointResources(ctx, false); err != nil {
		return errors.Wrap(err, "Unable to reconcile endpoint resources")
	}

	// Get the desired endpoint config name.
	if ctx.EndpointConfigName, err = ctx.EndpointConfigReconciler.GetSageMakerEndpointConfigName(ctx, ctx.Deployment); err != nil {
		return errors.Wrap(err, "Unable to get SageMaker EndpointConfigName for endpoint")
	}

	if ctx.EndpointConfigName == "" {
		return fmt.Errorf("Unable to get SageMaker EndpointConfigName for endpoint")
	}

	// If the desired endpoint config name does not equal the actual endpoint config name, we need to call UpdateEndpoint.
	if *ctx.EndpointDescription.EndpointConfigName != ctx.EndpointConfigName {
		r.Log.Info("Endpoint needs update", "endpoint name", ctx.EndpointName, "actual config name", ctx.EndpointDescription.EndpointConfigName, "desired config name", ctx.EndpointConfigName)

		var output *sagemaker.UpdateEndpointOutput
		if output, err = ctx.SageMakerClient.UpdateEndpoint(ctx, ctx.EndpointName, ctx.EndpointConfigName); err != nil && !clientwrapper.IsUpdateEndpoint404Error(err) {
			return errors.Wrap(err, "Unable to update Endpoint")
		}

		// If the endpoint doesn't exist.
		if output == nil {
			return fmt.Errorf("Unable to update Endpoint: Endpoint not found")
		}
	} else {
		r.Log.Info("Endpoint up to date")

		// Clean up from previous update.
		if err = r.reconcileEndpointResources(ctx, true); err != nil {
			return errors.Wrap(err, "Unable to reconcile endpoint resources")
		}
	}

	return nil
}

// Create models and endpoint configs in SageMaker, then create the Endpoint.
func (r *HostingDeploymentReconciler) createEndpoint(ctx reconcileRequestContext) error {
	var err error
	if err = r.reconcileEndpointResources(ctx, false); err != nil {
		return errors.Wrap(err, "Unable to reconcile Endpoint resources")
	}

	var createEndpointInput *sagemaker.CreateEndpointInput
	if createEndpointInput, err = r.createCreateEndpointInput(ctx); err != nil {
		return errors.Wrap(err, "Unable to create CreateEndpointInput")
	}

	r.Log.Info("Create endpoint", "input", createEndpointInput)

	if _, err := ctx.SageMakerClient.CreateEndpoint(ctx, createEndpointInput); err != nil {
		return errors.Wrap(err, "Unable to create Endpoint")
	}

	return nil
}

// Clean up all models and endpoints, then remove the HostingDeployment finalizer.
func (r *HostingDeploymentReconciler) cleanupAndRemoveFinalizer(ctx reconcileRequestContext) error {
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

// Initialize fields on the context object which will be used later.
func (r *HostingDeploymentReconciler) initializeContext(ctx *reconcileRequestContext) error {
	ctx.EndpointName = GetGeneratedResourceName(ctx.Deployment.ObjectMeta.GetUID(), ctx.Deployment.ObjectMeta.GetName(), 63)
	r.Log.Info("SageMaker EndpointName", "name", ctx.EndpointName)

	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.Deployment.Spec.Region, ctx.Deployment.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.SageMakerClient = clientwrapper.NewSageMakerClientWrapper(r.createSageMakerClient(awsConfig))
	ctx.Log.Info("Loaded AWS config")
	ctx.ModelReconciler = r.createModelReconciler(r, ctx.Log)
	ctx.EndpointConfigReconciler = r.createEndpointConfigReconciler(r, ctx.Log)

	return nil
}

// Reconcile resources necessary to create an Endpoint. This includes Models and EndpointConfigs. If shouldDeleteUnusedResources is true,
// then clean up all resources that are unused. This should be false when an update is in-progress, as SageMaker requires both sets of
// models/endpoint configs exist until the update is finished.
func (r *HostingDeploymentReconciler) reconcileEndpointResources(ctx reconcileRequestContext, shouldDeleteUnusedResources bool) error {
	r.Log.Info("Reconcile endpoint resources", "shouldDeleteUnusedResources", shouldDeleteUnusedResources)

	// TODO This should create models, then endpoint configs when creating, and reverse the order when deleting.
	if err := ctx.ModelReconciler.Reconcile(ctx, ctx.Deployment, shouldDeleteUnusedResources); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingEndpointStatus, errors.Wrap(err, "Unable to reconcile models"))
	}

	if err := ctx.EndpointConfigReconciler.Reconcile(ctx, ctx.Deployment, shouldDeleteUnusedResources); err != nil {
		return r.updateStatusAndReturnError(ctx, ReconcilingEndpointStatus, errors.Wrap(err, "Unable to reconcile EndpointConfigs"))
	}
	return nil
}

// Create CreateEndpointInput.
func (r *HostingDeploymentReconciler) createCreateEndpointInput(ctx reconcileRequestContext) (*sagemaker.CreateEndpointInput, error) {
	r.Log.Info("Create CreateEndpointInput")

	var endpointConfigName string
	var err error
	if endpointConfigName, err = ctx.EndpointConfigReconciler.GetSageMakerEndpointConfigName(ctx, ctx.Deployment); err != nil {
		return nil, err
	}

	endpointName := GetGeneratedResourceName(ctx.Deployment.ObjectMeta.GetUID(), ctx.Deployment.ObjectMeta.GetName(), 63)

	createInput := &sagemaker.CreateEndpointInput{
		EndpointConfigName: &endpointConfigName,
		EndpointName:       &endpointName,
		Tags:               sdkutil.ConvertTagSliceToSageMakerTagSlice(ctx.Deployment.Spec.Tags),
	}

	return createInput, nil
}

// Helper method to update the status with the error message and status. If there was an error updating the status, return
// that error instead.
func (r *HostingDeploymentReconciler) updateStatusAndReturnError(ctx reconcileRequestContext, status string, reconcileErr error) error {
	if err := r.updateStatusWithAdditional(ctx, status, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

// Update the status and other informational fields.
// Returns an error if there was a failure to update.
func (r *HostingDeploymentReconciler) updateStatus(ctx reconcileRequestContext, endpointStatus string) error {
	return r.updateStatusWithAdditional(ctx, endpointStatus, "")
}

// Update the status and other informational fields. The "additional" parameter should be used to convey additional error information. Leave empty to omit.
// Returns an error if there was a failure to update.
func (r *HostingDeploymentReconciler) updateStatusWithAdditional(ctx reconcileRequestContext, endpointStatus, additional string) error {
	ctx.Log.Info("updateStatusWithAdditional", "endpointStatus", endpointStatus, "additional", additional)

	deploymentStatus := &ctx.Deployment.Status
	endpoint := ctx.EndpointDescription

	deploymentStatus.Additional = additional
	deploymentStatus.EndpointStatus = endpointStatus
	deploymentStatus.LastCheckTime = Now()

	if endpoint != nil {
		deploymentStatus.EndpointName = GetOrDefault(endpoint.EndpointName, "")
		deploymentStatus.EndpointArn = GetOrDefault(endpoint.EndpointArn, "")
		deploymentStatus.FailureReason = GetOrDefault(endpoint.FailureReason, "")
		deploymentStatus.LastModifiedTime = convertTimeOrDefault(endpoint.LastModifiedTime, nil)
		deploymentStatus.CreationTime = convertTimeOrDefault(endpoint.CreationTime, nil)

		// TODO The ProductionVariant retrieved from SageMaker does not reflect updates (desiredInstanceCount does not change, even though it should).
		// This might be a bug in how we're using SageMaker or a bug in SageMaker.
		if converted, err := sdkutil.ConvertProductionVariantSummarySlice(endpoint.ProductionVariants); err != nil {
			return errors.Wrap(err, "Unable to interpret ProductionVariant for status")
		} else {
			deploymentStatus.ProductionVariants = converted
		}
	}

	deploymentStatus.ModelNames = sdkutil.ConvertMapToKeyValuePairSlice(ctx.ModelNames)
	deploymentStatus.EndpointConfigName = ctx.EndpointConfigName

	if ctx.Deployment.Spec.Region != nil && ctx.EndpointName != "" {
		deploymentStatus.EndpointUrl = "https://runtime.sagemaker." + *ctx.Deployment.Spec.Region + ".amazonaws.com/endpoints/" + ctx.EndpointName + "/invocations"
	} else {
		deploymentStatus.EndpointUrl = ""
	}

	if err := r.Status().Update(ctx, ctx.Deployment); err != nil {
		err = errors.Wrap(err, "Unable to update status")
		ctx.Log.Info("Error while updating status.", "err", err)
		return err
	}

	return nil
}

// Convert a *time.Time to a *metav1.Time, or default value if *time.Time is nil.
func convertTimeOrDefault(time *time.Time, defaultValue *metav1.Time) *metav1.Time {
	if time == nil {
		return defaultValue
	}

	converted := metav1.NewTime(*time)
	return &converted
}

func (r *HostingDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hostingv1.HostingDeployment{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// Get the Kubernetes name of the Model/EndpointConfig. This must be idempotent so that future reconciler invocations
// are able to find the object.
// Kubernetes resources can have names up to 253 characters long.
// The characters allowed in names are: digits (0-9), lower case letters (a-z), -, and .
func GetKubernetesNamespacedName(objectName string, hostingDeployment hostingv1.HostingDeployment) types.NamespacedName {
	k8sMaxLen := 253
	uid := hostingDeployment.ObjectMeta.GetUID()
	generation := strconv.FormatInt(hostingDeployment.ObjectMeta.GetGeneration(), 10)
	generatedPostfix := generation + "-"

	name := GetGeneratedResourceName(uid, objectName, k8sMaxLen, generatedPostfix)

	return types.NamespacedName{
		Name:      name,
		Namespace: hostingDeployment.ObjectMeta.GetNamespace(),
	}
}
