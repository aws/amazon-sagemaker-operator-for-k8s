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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"

	hostingv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/hostingdeployment"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers"
	"go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/sdkutil"
	"go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/sdkutil/clientwrapper"
)

const (
	PreparingEndpointStatus = "PreparingEndpoint"
	DeletingEndpointStatus  = "DeletingEndpoint"
	DeletedEndpointStatus   = "DeletedEndpoint"
)

// HostingDeploymentReconciler reconciles a HostingDeployment object
type HostingDeploymentReconciler struct {
	client.Client
	Log          logr.Logger
	PollInterval time.Duration

	awsConfigLoader                AwsConfigLoader
	createModelReconciler          ModelReconcilerProvider
	createEndpointConfigReconciler EndpointConfigReconcilerProvider
	createEndpointReconciler       EndpointReconcilerProvider
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
		createEndpointReconciler:       NewEndpointReconciler,
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hostingdeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=hostingdeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=models/status,verbs=get;update;patch

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
	EndpointReconciler       EndpointReconciler

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

func (r *HostingDeploymentReconciler) reconcileHostingDeployment(ctx reconcileRequestContext) error {

	ctx.EndpointName = GetSageMakerEndpointName(*ctx.Deployment)
	r.Log.Info("SageMaker EndpointName", "name", ctx.EndpointName)

	// TODO add SageMaker endpoint to spec.
	sageMakerEndpoint := ""
	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.Deployment.Spec.Region, &sageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.SageMakerClient = clientwrapper.NewSageMakerClientWrapper(r.createSageMakerClient(awsConfig))
	ctx.Log.Info("Loaded AWS config")
	ctx.ModelReconciler = r.createModelReconciler(r, ctx.Log)
	ctx.EndpointConfigReconciler = r.createEndpointConfigReconciler(r, ctx.Log)
	ctx.EndpointReconciler = r.createEndpointReconciler(r, ctx.Log, ctx.SageMakerClient)

	return r.reconcile(ctx)

}

// Reconcile HostingDeployments
func (r *HostingDeploymentReconciler) reconcile(ctx reconcileRequestContext) error {

	var err error
	if ctx.Deployment.Status.EndpointStatus == "" {
		if err = r.updateStatus(ctx, InitializingJobStatus); err != nil {
			return err
		}
	}

	// Add finalizer if it's not marked for deletion.
	if ctx.Deployment.ObjectMeta.GetDeletionTimestamp().IsZero() {
		if !ContainsString(ctx.Deployment.ObjectMeta.GetFinalizers(), SageMakerResourceFinalizerName) {
			ctx.Deployment.ObjectMeta.Finalizers = append(ctx.Deployment.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.Deployment); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer added")
		}
	}
	// Get the EndpointDescription from SageMaker.
	if ctx.EndpointDescription, err = ctx.SageMakerClient.DescribeEndpoint(ctx, ctx.EndpointName); err != nil {
		reconcileErr := errors.Wrap(err, "Unable to describe SageMaker endpoint")
		if err = r.updateStatusWithAdditional(ctx, PreparingEndpointStatus, reconcileErr.Error()); err != nil {
			return err
		}

		return reconcileErr
	}

	// Reconcile Models.
	if err = ctx.ModelReconciler.Reconcile(ctx, ctx.Deployment); err != nil {

		reconcileErr := errors.Wrap(err, "Unable to reconcile models")
		if err = r.updateStatusWithAdditional(ctx, PreparingEndpointStatus, reconcileErr.Error()); err != nil {
			return err
		}

		return reconcileErr
	}

	if ctx.ModelNames, err = ctx.ModelReconciler.GetSageMakerModelNames(ctx, ctx.Deployment); err != nil {
		ctx.Log.Info("Unable to get model names", "err", err)
	}

	// Reconcile EndpointConfigs.
	if err = ctx.EndpointConfigReconciler.Reconcile(ctx, ctx.Deployment); err != nil {

		reconcileErr := errors.Wrap(err, "Unable to reconcile EndpointConfig")
		if err = r.updateStatusWithAdditional(ctx, PreparingEndpointStatus, reconcileErr.Error()); err != nil {
			return err
		}

		return reconcileErr
	}

	if ctx.EndpointConfigName, err = ctx.EndpointConfigReconciler.GetSageMakerEndpointConfigName(ctx, ctx.Deployment); err != nil {
		ctx.Log.Info("Unable to get endpoint config name", "err", err)
	}

	// Reconcile Endpoints.
	if err = ctx.EndpointReconciler.Reconcile(ctx, ctx.Deployment, ctx.EndpointDescription); err != nil {

		reconcileErr := errors.Wrap(err, "Unable to reconcile Endpoint")
		if err = r.updateStatusWithAdditional(ctx, PreparingEndpointStatus, reconcileErr.Error()); err != nil {
			return err
		}

		return reconcileErr
	}

	var status string
	// If not mark for deletion
	if ctx.Deployment.ObjectMeta.GetDeletionTimestamp().IsZero() {
		// If the SageMaker endpoint has been created, update the status with the new information.
		if ctx.EndpointDescription != nil {
			status = string(ctx.EndpointDescription.EndpointStatus)
		} else {
			status = PreparingEndpointStatus
		}
	} else {
		// If SageMaker endpoint exists
		if ctx.EndpointDescription != nil {
			status = string(ctx.EndpointDescription.EndpointStatus)
		} else {
			status = DeletedEndpointStatus
		}
	}

	// Update the status for endpoint
	if err = r.updateStatus(ctx, status); err != nil {
		return err
	}

	// Remove finalizer if it's marked for deletion
	if !ctx.Deployment.ObjectMeta.GetDeletionTimestamp().IsZero() {
		ctx.Deployment.ObjectMeta.Finalizers = RemoveString(ctx.Deployment.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
		if err := r.Update(ctx, ctx.Deployment); err != nil {
			return errors.Wrap(err, "Failed to remove finalizer")
		}
		ctx.Log.Info("Finalizer has been removed")
	}
	return nil
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

// TODO add code that ignores status, metadata updates.
func (r *HostingDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hostingv1.HostingDeployment{}).
		Complete(r)
}
