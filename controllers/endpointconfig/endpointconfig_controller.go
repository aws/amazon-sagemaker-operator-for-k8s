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

package endpointconfig

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	endpointconfigv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/endpointconfig"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
)

// This file is almost identical to controllers/model/model_controller.go. We should create a reconciler
// that implements common logic and use it here and in model controller. (cadedaniel: I tried but did not have
// enough time to think how to do it using composition instead of inheritance for release).
// TODO refactor common code with model_controller.
const (
	// Status to indicate that the desired endpointConfig has been created in SageMaker.
	CreatedStatus = "Created"

	// Status to indicate that the desired endpointConfig has been deleted in SageMaker.
	DeletedStatus = "Deleted"

	// Status to indicate that an error occurred during reconciliation.
	ErrorStatus = "Error"
)

// EndpointConfigReconciler reconciles a EndpointConfig object
type EndpointConfigReconciler struct {
	client.Client
	Log          logr.Logger
	PollInterval time.Duration

	awsConfigLoader       AwsConfigLoader
	createSageMakerClient SageMakerClientProvider
}

func NewEndpointConfigReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *EndpointConfigReconciler {
	return &EndpointConfigReconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createSageMakerClient: func(awsConfig aws.Config) sagemakeriface.ClientAPI {
			return sagemaker.New(awsConfig)
		},
		awsConfigLoader: NewAwsConfigLoader(),
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=endpointconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=endpointconfigs/status,verbs=get;update;patch

func (r *EndpointConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context:        context.Background(),
		Log:            r.Log.WithValues("endpointConfig", req.NamespacedName),
		EndpointConfig: new(endpointconfigv1.EndpointConfig),
	}

	// Get state from etcd
	if err := r.Get(ctx, req.NamespacedName, ctx.EndpointConfig); err != nil {
		ctx.Log.Info("Unable to fetch EndpointConfig", "reason", err)
		if apierrs.IsNotFound(err) {
			return NoRequeue()
		} else {
			return RequeueImmediately()
		}
	}

	// TODO: Sometime reconcileEndpointConfig would like to have noRequeue.
	//       for e.g. on completion of delete we don't need to do requeu.
	//       That feature should be supported in following code block
	if err := r.reconcileEndpointConfig(ctx); err != nil {
		// TODO stack traces are not printed well.
		// TODO if an endpointConfig fails to be created due to bad spec and the controller enters
		// a long backoff retry period before the user fixes the bad spec, will it create
		// two controller loops? How to fix this?
		ctx.Log.Info("Got error while reconciling, will retry", "err", err)
		return RequeueImmediately()
	} else {
		return RequeueAfterInterval(r.PollInterval, nil)
	}

}

type reconcileRequestContext struct {
	context.Context

	Log             logr.Logger
	SageMakerClient clientwrapper.SageMakerClientWrapper

	EndpointConfigDescription *sagemaker.DescribeEndpointConfigOutput
	EndpointConfig            *endpointconfigv1.EndpointConfig
}

// Reconcile creation, updates, and deletion of endpointConfigs. If the spec changed, then
// delete the existing one and create a new one that matches the spec.
func (r *EndpointConfigReconciler) reconcileEndpointConfig(ctx reconcileRequestContext) error {

	var err error

	// Update initial status.
	if ctx.EndpointConfig.Status.Status == "" {
		if err = r.updateStatus(ctx, InitializingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if the EndpointConfig is not marked for deletion.
	if !HasDeletionTimestamp(ctx.EndpointConfig.ObjectMeta) {
		if !ContainsString(ctx.EndpointConfig.ObjectMeta.GetFinalizers(), SageMakerResourceFinalizerName) {
			ctx.EndpointConfig.ObjectMeta.Finalizers = append(ctx.EndpointConfig.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.EndpointConfig); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer has been added")
		}
	}

	// Get SageMaker endpointConfig description.
	if ctx.EndpointConfigDescription, err = ctx.SageMakerClient.DescribeEndpointConfig(ctx, generateEndpointConfigName(ctx.EndpointConfig)); err != nil {
		return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to get SageMaker EndpointConfig description"))
	}

	// Determine action needed for endpointConfig and endpointConfig description.
	var action ReconcileAction
	if action, err = r.determineActionForEndpointConfig(ctx.EndpointConfig, ctx.EndpointConfigDescription); err != nil {
		return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to determine action for SageMaker EndpointConfig."))
	}
	ctx.Log.Info("Determined action for endpointConfig", "action", action)

	// If update or delete, delete the existing endpointConfig.
	if action == NeedsDelete || action == NeedsUpdate {
		if err = r.reconcileDeletion(ctx, ctx.SageMakerClient, generateEndpointConfigName(ctx.EndpointConfig)); err != nil {
			return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to delete SageMaker endpointConfig"))
		}

		// Delete succeeded, set EndpointConfigDescription to nil.
		ctx.EndpointConfigDescription = nil
	}

	// If update or create, create the desired endpointConfig.
	if action == NeedsCreate || action == NeedsUpdate {
		if ctx.EndpointConfigDescription, err = r.reconcileCreation(ctx, ctx.SageMakerClient, ctx.EndpointConfig.Spec, generateEndpointConfigName(ctx.EndpointConfig)); err != nil {
			return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to create SageMaker endpointConfig"))
		}
	}

	// Update the status accordingly.
	status := CreatedStatus
	if ctx.EndpointConfigDescription == nil {
		status = DeletedStatus
	}

	if err = r.updateStatus(ctx, status); err != nil {
		return err
	}

	// Remove finalizer on deletion.
	if HasDeletionTimestamp(ctx.EndpointConfig.ObjectMeta) {
		ctx.EndpointConfig.ObjectMeta.Finalizers = RemoveString(ctx.EndpointConfig.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
		if err := r.Update(ctx, ctx.EndpointConfig); err != nil {
			return errors.Wrap(err, "Failed to remove finalizer")
		}
		ctx.Log.Info("Finalizer has been removed")
	}

	return nil
}

// Initialize config on context object.
func (r *EndpointConfigReconciler) initializeContext(ctx *reconcileRequestContext) error {

	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.EndpointConfig.Spec.Region, ctx.EndpointConfig.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.SageMakerClient = clientwrapper.NewSageMakerClientWrapper(r.createSageMakerClient(awsConfig))
	ctx.Log.Info("Loaded AWS config")

	return nil
}

// For a desired endpointConfig and an actual endpointConfig, determine the action needed to reconcile the two.
func (r *EndpointConfigReconciler) determineActionForEndpointConfig(desiredEndpointConfig *endpointconfigv1.EndpointConfig, actualEndpointConfig *sagemaker.DescribeEndpointConfigOutput) (ReconcileAction, error) {
	if HasDeletionTimestamp(desiredEndpointConfig.ObjectMeta) {
		if actualEndpointConfig != nil {
			return NeedsDelete, nil
		}
		return NeedsNoop, nil
	}

	if actualEndpointConfig == nil {
		return NeedsCreate, nil
	}

	var comparison sdkutil.Comparison
	var err error
	if comparison, err = sdkutil.EndpointConfigSpecMatchesDescription(*actualEndpointConfig, desiredEndpointConfig.Spec); err != nil {
		return NeedsNoop, err
	}

	r.Log.Info("Compared existing endpointConfig to actual endpointConfig to determine if endpointConfig needs to be updated.", "differences", comparison.Differences, "equal", comparison.Equal)

	if comparison.Equal {
		return NeedsNoop, nil
	}

	return NeedsUpdate, nil
}

// Given that the endpointConfig should be deleted, delete the endpointConfig.
func (r *EndpointConfigReconciler) reconcileDeletion(ctx context.Context, sageMakerClient clientwrapper.SageMakerClientWrapper, endpointConfigName string) error {
	var err error
	var input *sagemaker.DeleteEndpointConfigInput
	if input, err = sdkutil.CreateDeleteEndpointConfigInput(&endpointConfigName); err != nil {
		return errors.Wrap(err, "Unable to create SageMaker DeleteEndpointConfig request")
	}

	if _, err := sageMakerClient.DeleteEndpointConfig(ctx, input); err != nil {
		return errors.Wrap(err, "Unable to delete SageMaker EndpointConfig")
	}

	return nil
}

// Given that the endpointConfig should be created, create the endpointConfig.
func (r *EndpointConfigReconciler) reconcileCreation(ctx context.Context, sageMakerClient clientwrapper.SageMakerClientWrapper, spec endpointconfigv1.EndpointConfigSpec, endpointConfigName string) (*sagemaker.DescribeEndpointConfigOutput, error) {
	var err error
	var input *sagemaker.CreateEndpointConfigInput
	if input, err = sdkutil.CreateCreateEndpointConfigInputFromSpec(&spec, endpointConfigName); err != nil {
		return nil, errors.Wrap(err, "Unable to create SageMaker CreateEndpointConfig request")
	}

	if _, err := sageMakerClient.CreateEndpointConfig(ctx, input); err != nil {
		return nil, errors.Wrap(err, "Unable to create SageMaker EndpointConfig")
	}

	var output *sagemaker.DescribeEndpointConfigOutput
	if output, err = sageMakerClient.DescribeEndpointConfig(ctx, endpointConfigName); err != nil {
		return nil, errors.Wrap(err, "Unable to get SageMaker endpointConfig description")
	}

	// The endpointConfig does not exist.
	if output == nil {
		return nil, fmt.Errorf("Creation failed, endpointConfig does not exist after creation")
	}

	return output, err
}

// Helper method to update the status with the error message and status. If there was an error updating the status, return
// that error instead.
func (r *EndpointConfigReconciler) updateStatusAndReturnError(ctx reconcileRequestContext, status string, reconcileErr error) error {
	if err := r.updateStatusWithAdditional(ctx, status, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

// Update the status and LastCheckTime. This also updates the status fields obtained from
// the SageMaker description, if they are present.
func (r *EndpointConfigReconciler) updateStatus(ctx reconcileRequestContext, status string) error {
	return r.updateStatusWithAdditional(ctx, status, "")
}

// Update the status and LastCheckTime. This also updates the status fields obtained from
// the SageMaker description, if they are present.
func (r *EndpointConfigReconciler) updateStatusWithAdditional(ctx reconcileRequestContext, status, additional string) error {
	ctx.EndpointConfig.Status.Status = status
	ctx.EndpointConfig.Status.Additional = additional

	ctx.EndpointConfig.Status.EndpointConfigArn = ""
	ctx.EndpointConfig.Status.SageMakerEndpointConfigName = ""

	if ctx.EndpointConfigDescription != nil {
		if ctx.EndpointConfigDescription.EndpointConfigName != nil {
			ctx.EndpointConfig.Status.SageMakerEndpointConfigName = *ctx.EndpointConfigDescription.EndpointConfigName
		}

		if ctx.EndpointConfigDescription.EndpointConfigArn != nil {
			ctx.EndpointConfig.Status.EndpointConfigArn = *ctx.EndpointConfigDescription.EndpointConfigArn
		}
	}

	ctx.EndpointConfig.Status.LastCheckTime = Now()

	ctx.Log.Info("Updating status", "status", status, "additional", additional, "statusObject", ctx.EndpointConfig.Status)

	if err := r.Status().Update(ctx, ctx.EndpointConfig); err != nil {
		return errors.Wrap(err, "Unable to update status")
	}

	return nil
}

func generateEndpointConfigName(endpointConfig *endpointconfigv1.EndpointConfig) string {
	sageMakerMaxNameLen := 63
	name := endpointConfig.ObjectMeta.GetName()
	requiredPostfix := "-" + strings.Replace(string(endpointConfig.ObjectMeta.GetUID()), "-", "", -1)

	sageMakerEndpointConfigName := name + requiredPostfix
	if len(sageMakerEndpointConfigName) > sageMakerMaxNameLen {
		sageMakerEndpointConfigName = name[:sageMakerMaxNameLen-len(requiredPostfix)] + requiredPostfix
	}

	return sageMakerEndpointConfigName
}

// TODO add code that ignores status, metadata updates.
func (r *EndpointConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&endpointconfigv1.EndpointConfig{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
