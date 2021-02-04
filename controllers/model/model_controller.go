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

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/aws/aws-sdk-go/service/sagemaker/sagemakeriface"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	modelv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/model"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
)

const (
	// Status to indicate that the desired model has been created in SageMaker.
	CreatedStatus = "Created"

	// Status to indicate that the desired model has been deleted in SageMaker.
	DeletedStatus = "Deleted"

	// Defines the maximum number of characters in a SageMaker Model SubResource name
	MaxModelNameLength = 63

	// Defines the default value for ContainerDefinition Mode
	DefaultContainerDefinitionMode = "SingleModel"
)

// ModelReconciler reconciles a Model object
type ModelReconciler struct {
	client.Client
	Log          logr.Logger
	PollInterval time.Duration

	awsConfigLoader       AwsConfigLoader
	createSageMakerClient SageMakerClientProvider
}

func NewModelReconciler(client client.Client, log logr.Logger, pollInterval time.Duration) *ModelReconciler {
	return &ModelReconciler{
		Client:       client,
		Log:          log,
		PollInterval: pollInterval,
		createSageMakerClient: func(awsConfig aws.Config) sagemakeriface.SageMakerAPI {
			return sagemaker.New(awsConfig)
		},
		awsConfigLoader: NewAwsConfigLoader(),
	}
}

// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sagemaker.aws.amazon.com,resources=models/status,verbs=get;update;patch

func (r *ModelReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := reconcileRequestContext{
		Context: context.Background(),
		Log:     r.Log.WithValues("model", req.NamespacedName),
		Model:   new(modelv1.Model),
	}

	// Get state from etcd
	if err := r.Get(ctx, req.NamespacedName, ctx.Model); err != nil {
		ctx.Log.Info("Unable to fetch Model", "reason", err)
		if apierrs.IsNotFound(err) {
			return NoRequeue()
		} else {
			return RequeueImmediately()
		}
	}

	// TODO: Sometime reconcileModel would like to have noRequeue.
	//       for e.g. on completion of delete we don't need to do requeu.
	//       That feature should be supported in following code block
	if err := r.reconcileModel(ctx); err != nil {
		// TODO stack traces are not printed well.
		// TODO if a model fails to be created due to bad spec and the controller enters
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

	ModelDescription *sagemaker.DescribeModelOutput
	Model            *modelv1.Model
}

// Reconcile creation, updates, and deletion of models. If the spec changed, then
// delete the existing one and create a new one that matches the spec.
func (r *ModelReconciler) reconcileModel(ctx reconcileRequestContext) error {

	var err error

	// Update initial status.
	if ctx.Model.Status.Status == "" {
		if err = r.updateStatus(ctx, InitializingJobStatus); err != nil {
			return err
		}
	}

	if err = r.initializeContext(&ctx); err != nil {
		return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to initialize operator"))
	}

	// Add finalizer if the Model is not marked for deletion.
	if !HasDeletionTimestamp(ctx.Model.ObjectMeta) {
		if !ContainsString(ctx.Model.ObjectMeta.GetFinalizers(), SageMakerResourceFinalizerName) {
			ctx.Model.ObjectMeta.Finalizers = append(ctx.Model.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
			if err := r.Update(ctx, ctx.Model); err != nil {
				return errors.Wrap(err, "Failed to add finalizer")
			}
			ctx.Log.Info("Finalizer has been added")
		}
	}

	// Get SageMaker model description.
	if ctx.ModelDescription, err = ctx.SageMakerClient.DescribeModel(ctx, generateModelName(ctx.Model)); err != nil {
		return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to get SageMaker Model description"))
	}

	// Determine action needed for model and model description.
	var action ReconcileAction
	if action, err = r.determineActionForModel(ctx.Model, ctx.ModelDescription); err != nil {
		return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to determine action for SageMaker Model."))
	}
	ctx.Log.Info("Determined action for model", "action", action)

	// If update or delete, delete the existing model.
	if action == NeedsDelete || action == NeedsUpdate {
		if err = r.reconcileDeletion(ctx, ctx.SageMakerClient, generateModelName(ctx.Model)); err != nil {
			return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to delete SageMaker model"))
		}

		// Delete succeeded, set ModelDescription to nil.
		ctx.ModelDescription = nil
	}

	// If update or create, create the desired model.
	if action == NeedsCreate || action == NeedsUpdate {
		if ctx.ModelDescription, err = r.reconcileCreation(ctx, ctx.SageMakerClient, ctx.Model.Spec, generateModelName(ctx.Model)); err != nil {
			return r.updateStatusAndReturnError(ctx, ErrorStatus, errors.Wrap(err, "Unable to create SageMaker model"))
		}
	}

	// Update the status accordingly.
	status := CreatedStatus
	if ctx.ModelDescription == nil {
		status = DeletedStatus
	}

	if err = r.updateStatus(ctx, status); err != nil {
		return err
	}

	// Remove finalizer on deletion.
	if HasDeletionTimestamp(ctx.Model.ObjectMeta) {
		ctx.Model.ObjectMeta.Finalizers = RemoveString(ctx.Model.ObjectMeta.Finalizers, SageMakerResourceFinalizerName)
		if err := r.Update(ctx, ctx.Model); err != nil {
			return errors.Wrap(err, "Failed to remove finalizer")
		}
		ctx.Log.Info("Finalizer has been removed")
	}

	return nil
}

// Initialize config on context object.
func (r *ModelReconciler) initializeContext(ctx *reconcileRequestContext) error {

	awsConfig, err := r.awsConfigLoader.LoadAwsConfigWithOverrides(*ctx.Model.Spec.Region, ctx.Model.Spec.SageMakerEndpoint)
	if err != nil {
		ctx.Log.Error(err, "Error loading AWS config")
		return err
	}

	ctx.SageMakerClient = clientwrapper.NewSageMakerClientWrapper(r.createSageMakerClient(awsConfig))
	ctx.Log.Info("Loaded AWS config")

	if ctx.Model.Spec.PrimaryContainer != nil && ctx.Model.Spec.PrimaryContainer.Mode == nil {
		ctx.Model.Spec.PrimaryContainer.Mode = controllertest.ToStringPtr(DefaultContainerDefinitionMode)
	}
	for _, container := range ctx.Model.Spec.Containers {
		if container.Mode == nil {
			container.Mode = controllertest.ToStringPtr(DefaultContainerDefinitionMode)
		}
	}

	return nil
}

// For a desired model and an actual model, determine the action needed to reconcile the two.
func (r *ModelReconciler) determineActionForModel(desiredModel *modelv1.Model, actualModel *sagemaker.DescribeModelOutput) (ReconcileAction, error) {
	if HasDeletionTimestamp(desiredModel.ObjectMeta) {
		if actualModel != nil {
			return NeedsDelete, nil
		}
		return NeedsNoop, nil
	}

	if actualModel == nil {
		return NeedsCreate, nil
	}

	var comparison sdkutil.Comparison
	var err error
	if comparison, err = sdkutil.ModelSpecMatchesDescription(*actualModel, desiredModel.Spec); err != nil {
		return NeedsNoop, err
	}

	r.Log.Info("Compared existing model to actual model to determine if model needs to be updated.", "differences", comparison.Differences, "equal", comparison.Equal)

	if comparison.Equal {
		return NeedsNoop, nil
	}

	return NeedsUpdate, nil
}

// Given that the model should be deleted, delete the model.
func (r *ModelReconciler) reconcileDeletion(ctx context.Context, sageMakerClient clientwrapper.SageMakerClientWrapper, modelName string) error {
	var err error
	var input *sagemaker.DeleteModelInput
	if input, err = sdkutil.CreateDeleteModelInput(&modelName); err != nil {
		return errors.Wrap(err, "Unable to create SageMaker DeleteModel request")
	}

	if _, err := sageMakerClient.DeleteModel(ctx, input); err != nil && !clientwrapper.IsDeleteModel404Error(err) {
		return errors.Wrap(err, "Unable to delete SageMaker Model")
	}

	return nil
}

// Given that the model should be created, create the model.
func (r *ModelReconciler) reconcileCreation(ctx context.Context, sageMakerClient clientwrapper.SageMakerClientWrapper, spec modelv1.ModelSpec, modelName string) (*sagemaker.DescribeModelOutput, error) {
	var err error
	var input *sagemaker.CreateModelInput
	if input, err = sdkutil.CreateCreateModelInputFromSpec(&spec, modelName); err != nil {
		return nil, errors.Wrap(err, "Unable to create SageMaker CreateModel request")
	}

	if _, err := sageMakerClient.CreateModel(ctx, input); err != nil {
		return nil, errors.Wrap(err, "Unable to create SageMaker Model")
	}

	var output *sagemaker.DescribeModelOutput
	if output, err = sageMakerClient.DescribeModel(ctx, modelName); err != nil {
		return nil, errors.Wrap(err, "Unable to get SageMaker model description")
	}

	// The model does not exist.
	if output == nil {
		return nil, fmt.Errorf("Creation failed, model does not exist after creation")
	}

	return output, err
}

// Helper method to update the status with the error message and status. If there was an error updating the status, return
// that error instead.
func (r *ModelReconciler) updateStatusAndReturnError(ctx reconcileRequestContext, status string, reconcileErr error) error {
	if err := r.updateStatusWithAdditional(ctx, status, reconcileErr.Error()); err != nil {
		return errors.Wrapf(reconcileErr, "Unable to update status with error. Status failure was caused by: '%s'", err.Error())
	}
	return reconcileErr
}

// Update the status and LastCheckTime. This also updates the status fields obtained from
// the SageMaker description, if they are present.
func (r *ModelReconciler) updateStatus(ctx reconcileRequestContext, status string) error {
	return r.updateStatusWithAdditional(ctx, status, "")
}

// Update the status and LastCheckTime. This also updates the status fields obtained from
// the SageMaker description, if they are present.
func (r *ModelReconciler) updateStatusWithAdditional(ctx reconcileRequestContext, status, additional string) error {
	ctx.Model.Status.Status = status
	ctx.Model.Status.Additional = additional

	ctx.Model.Status.ModelArn = ""
	ctx.Model.Status.SageMakerModelName = ""

	if ctx.ModelDescription != nil {
		if ctx.ModelDescription.ModelName != nil {
			ctx.Model.Status.SageMakerModelName = *ctx.ModelDescription.ModelName
		}

		if ctx.ModelDescription.ModelArn != nil {
			ctx.Model.Status.ModelArn = *ctx.ModelDescription.ModelArn
		}
	}

	ctx.Model.Status.LastCheckTime = Now()

	ctx.Log.Info("Updating status", "status", status, "additional", additional, "statusObject", ctx.Model.Status)

	if err := r.Status().Update(ctx, ctx.Model); err != nil {
		return errors.Wrap(err, "Unable to update status")
	}

	return nil
}

// TODO add code that ignores status, metadata updates.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&modelv1.Model{}).
		// Ignore status-only and metadata-only updates
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// Given a Model object, generate a SageMaker name of valid length
func generateModelName(model *modelv1.Model) string {
	return GetGeneratedJobName(model.ObjectMeta.GetUID(), model.ObjectMeta.GetName(), MaxModelNameLength)
}
