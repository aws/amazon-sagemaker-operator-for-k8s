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
	"reflect"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	hostingv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingdeployment"
	modelv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Type that returns an instantiated ModelReconciler given parameters.
type ModelReconcilerProvider func(client.Client, logr.Logger) ModelReconciler

// Helper method to create a ModelReconciler.
func NewModelReconciler(client client.Client, log logr.Logger) ModelReconciler {
	return &modelReconciler{
		k8sClient: client,
		log:       log.WithName("ModelReconciler"),
	}
}

// Helper type that is responsible for reconciling models of an endpoint.
type ModelReconciler interface {
	Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, shouldDeleteUnusedModels bool) error
	GetSageMakerModelNames(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (map[string]string, error)
}

// Concrete implementation of ModelReconciler.
type modelReconciler struct {
	k8sClient client.Client
	log       logr.Logger
}

// Make sure at compile time modelReconciler implements ModelReconciler
var _ ModelReconciler = (*modelReconciler)(nil)

// Reconcile desired deployment models with actual models.
// If there are models that are desired but do not exist, create them.
// If there are models that are not desired but exist, delete them.
// This creates a modelv1.Model in Kubernetes before creating the model in SageMaker for idempotency.
// The parameter shouldDeleteUnusedResources controls whether unnecessary endpoint configs are deleted. This is useful when updating Endpoints.
//
// Returns an error if any operation fails. The reconciliation should be retried if err is non-nil.
func (r *modelReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, shouldDeleteUnusedModels bool) error {
	r.log.Info("Reconciling models")

	var err error
	// Get the set of desired models. If this is empty, then no models are desired.
	var desiredModels map[string]*modelv1.Model
	if desiredModels, _, err = r.extractDesiredModelsFromHostingDeployment(desiredDeployment); err != nil {
		return errors.Wrap(err, "Unable to interpret models")
	}
	r.log.Info("Desired models", "models", desiredModels)

	// Get actual (existing) models.
	var actualModels map[string]*modelv1.Model
	if actualModels, err = r.getActualModelsForHostingDeployment(ctx, desiredDeployment); err != nil {
		return errors.Wrap(err, "Unable to get actual models")
	}
	r.log.Info("Actual models", "models", actualModels)

	modelsToCreate, modelsToDelete, modelsToUpdate, modelsToNoop := r.determineActionForModels(desiredModels, actualModels)

	r.log.Info("Determined action for each model", "modelsToCreate", getModelNamesFromMap(modelsToCreate), "modelsToDelete", getModelNamesFromMap(modelsToDelete), "modelsToUpdate", getModelNamesFromMap(modelsToUpdate), "modelsToNoop", getModelNamesFromMap(modelsToNoop))

	if err = r.reconcileModelsToCreate(ctx, modelsToCreate); err != nil {
		return errors.Wrap(err, "Unable to create model(s)")
	}

	if shouldDeleteUnusedModels {
		if err = r.reconcileModelsToDelete(ctx, modelsToDelete); err != nil {
			return errors.Wrap(err, "Unable to delete model(s)")
		}
	} else {
		r.log.Info("Ignoring modelsToDelete because shouldDeleteUnusedModels=false", "shouldDeleteUnusedModels", shouldDeleteUnusedModels)
	}

	if err = r.reconcileModelsToUpdate(ctx, modelsToUpdate); err != nil {
		return errors.Wrap(err, "Unable to update model(s)")
	}

	return nil
}

// For a set of desired models and actual models owned by the hosting deployment, categorize each model as
// a model that needs to be created, deleted, updated, or noop (no-operation).
// Return the list of models that fit in each category.
// This is done by determining if the desired model exists and is deep equal to the actual model.
func (r *modelReconciler) determineActionForModels(desiredModels, actualModels map[string]*modelv1.Model) (modelsToCreate, modelsToDelete, modelsToUpdate, modelsToNoop map[string]*modelv1.Model) {
	// This can be refactored to use common.ReconcileAction instead of separate maps. A single list
	// of model-action pairs would enable execution to run concurrently.
	modelsToCreate = map[string]*modelv1.Model{}
	modelsToDelete = map[string]*modelv1.Model{}
	modelsToUpdate = map[string]*modelv1.Model{}
	modelsToNoop = map[string]*modelv1.Model{}
	visitedModels := map[string]*modelv1.Model{}

	for name, desiredModel := range desiredModels {
		if actualModel, exists := actualModels[name]; exists {
			// If a desired model deep equals an existing model by the same name,
			// no action should be performed.
			// Otherwise, add it to the toUpdate list.
			if reflect.DeepEqual(desiredModel.Spec, actualModel.Spec) {
				modelsToNoop[name] = desiredModel
			} else {
				// We only want to update the spec, so merge the actual metadata with the desired
				// spec.
				targetModel := actualModels[name].DeepCopy()
				targetModel.Spec = desiredModel.Spec
				modelsToUpdate[name] = targetModel
			}
		} else {
			// If the actual model does not exist, we need to create it.
			modelsToCreate[name] = desiredModel
		}

		// Mark that we have processed this model.
		visitedModels[name] = desiredModel
	}

	// For every actual model that has not yet been visited,
	// we know that it is not a desired model.
	// So add it to toDelete.
	for name, actualModel := range actualModels {
		if _, visited := visitedModels[name]; visited {
			continue
		}

		modelsToDelete[name] = actualModel
	}

	return
}

// Helper method to get a slice of model names from a map.
func getModelNamesFromMap(m map[string]*modelv1.Model) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper method to get Kubernetes labels that are applied to all created models.
// These labels contain the namespaced-name of the HostingDeployment, allowing it to retrieve all
// owned models.
// This is necessary when the spec is updated and a model is completely deleted from the spec. In order
// to delete the model in Kubernetes, the HostingDeployment controller needs to be able to list all
// models that it created.
func GetResourceOwnershipLabelsForHostingDeployment(deployment hostingv1.HostingDeployment) map[string]string {
	typeName := reflect.TypeOf(deployment).Name()

	return map[string]string{
		("owner_" + typeName + "_namespace"): deployment.ObjectMeta.GetNamespace(),
		("owner_" + typeName + "_name"):      deployment.ObjectMeta.GetName(),
	}
}

// Get models created for the desired deployment. This looks up existing models by HostingDeployment ownership labels.
func (r *modelReconciler) getActualModelsForHostingDeployment(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (map[string]*modelv1.Model, error) {

	ownershipLabelSelector := client.MatchingLabels(GetResourceOwnershipLabelsForHostingDeployment(*desiredDeployment))

	// TODO need to support pagination.
	// See https://github.com/kubernetes-sigs/controller-runtime/blob/6b91e8e65756b561525314771a913145d161aa14/pkg/client/options.go#L457-L461 for how to use limits and test.
	// We are still on v0.2.0, this limit is added later.
	// The continuation token in models.ListMeta will be an option that is passed to List().
	models := &modelv1.ModelList{}
	if err := r.k8sClient.List(ctx, models, ownershipLabelSelector); err != nil {
		return nil, err
	}

	actualModels := map[string]*modelv1.Model{}
	for i, model := range models.Items {
		actualModels[model.ObjectMeta.Name] = &models.Items[i]
	}

	return actualModels, nil

}

// Generate a map of Kubernetes model names (defined by user in spec) and the corresponding auto-generated SageMaker names.
// Returns error if unable to get the Kubernetes Models, or if the Models do not yet have a SageMaker name.
func (r *modelReconciler) GetSageMakerModelNames(ctx context.Context, deployment *hostingv1.HostingDeployment) (map[string]string, error) {

	var err error
	var desiredModels map[string]*modelv1.Model
	var specModelNameToK8sNameMap map[string]string
	if desiredModels, specModelNameToK8sNameMap, err = r.extractDesiredModelsFromHostingDeployment(deployment); err != nil {
		return nil, errors.Wrap(err, "Unable to interpret models")
	}

	return r.getSageMakerModelNames(ctx, desiredModels, specModelNameToK8sNameMap)
}

// For a map of desiredModels, return a map of their names to the SageMaker model name.
func (r *modelReconciler) getSageMakerModelNames(ctx context.Context, desiredModels map[string]*modelv1.Model, specModelNameToK8sNameMap map[string]string) (map[string]string, error) {

	sageMakerModelNames := map[string]string{}

	for modelSpecName, modelK8sName := range specModelNameToK8sNameMap {

		desiredModel := desiredModels[modelK8sName]

		key := types.NamespacedName{
			Name:      desiredModel.ObjectMeta.GetName(),
			Namespace: desiredModel.ObjectMeta.GetNamespace(),
		}

		r.log.Info("Obtaining model SageMakerModelName from Kubernetes Model", "model", key)

		var actualModel modelv1.Model
		if err := r.k8sClient.Get(ctx, key, &actualModel); err != nil {
			if apierrs.IsNotFound(err) {
				return nil, errors.Wrapf(err, "Awaiting model creation: '%s' ", key)
			} else {
				return nil, errors.Wrapf(err, "Unable to determine if model '%s' was created", key)
			}
		}

		if actualModel.Status.SageMakerModelName == "" {
			causedBy := ""
			if actualModel.Status.Additional != "" {
				causedBy = fmt.Sprintf("Caused by: %s", actualModel.Status.Additional)
			}
			return nil, fmt.Errorf("Awaiting model name for '%s' to not be empty. %s", key, causedBy)
		}

		sageMakerModelNames[modelSpecName] = actualModel.Status.SageMakerModelName
	}

	return sageMakerModelNames, nil
}

// Convert the desired HostingDeployment models to modelv1.Models.
// This returns a map of k8sModelName->modelv1.Model and a map of modelName->k8sModelName.
// This also validates the model definitions; any error should be presented to the user.
func (r *modelReconciler) extractDesiredModelsFromHostingDeployment(deployment *hostingv1.HostingDeployment) (map[string]*modelv1.Model, map[string]string, error) {

	// If the desired deployment is to be deleted, then there are no desired models.
	if !deployment.ObjectMeta.GetDeletionTimestamp().IsZero() {
		return map[string]*modelv1.Model{}, map[string]string{}, nil
	}

	var err error
	var models map[string]*commonv1.Model
	if models, err = r.getAndValidateModelMap(deployment.Spec.Models); err != nil {
		return nil, nil, err
	}

	desiredModels := map[string]*modelv1.Model{}
	modelNameMap := map[string]string{}

	// For each desired model, create a Kubernetes spec for it.
	for name, model := range models {

		var containers map[string]*commonv1.ContainerDefinition
		if containers, err = r.getAndValidateContainerMap(model); err != nil {
			return nil, nil, err
		}

		var primaryContainer *commonv1.ContainerDefinition
		var modelContainers []*commonv1.ContainerDefinition

		// For multi-container, pass containers list directly - ignore primary container definition
		if len(model.Containers) > 1 {
			// Provide a useful log to inform users that primary container isn't being used
			if model.PrimaryContainer != nil {
				r.log.Info("The primary container field is ignored if more than one containers is specified")
			}

			primaryContainer = nil
			modelContainers = model.Containers
		} else {
			// Determine which container is marked as primary
			if primaryContainer, err = r.getPrimaryContainerDefinition(model, containers); err != nil {
				return nil, nil, err
			}

			modelContainers = nil
		}
		namespacedName := GetSubresourceNamespacedName(name, *deployment)

		// Add labels to model that indicate which particular HostingDeployment
		// owns it.
		ownershipLabels := GetResourceOwnershipLabelsForHostingDeployment(*deployment)

		k8sModel := &modelv1.Model{
			ObjectMeta: metav1.ObjectMeta{
				Name:      namespacedName.Name,
				Namespace: namespacedName.Namespace,
				Labels:    ownershipLabels,
			},
			Spec: modelv1.ModelSpec{
				Containers:             modelContainers,
				EnableNetworkIsolation: model.EnableNetworkIsolation,
				ExecutionRoleArn:       model.ExecutionRoleArn,
				PrimaryContainer:       primaryContainer,
				VpcConfig:              model.VpcConfig,
				Tags:                   commonv1.DeepCopyTagSlice(deployment.Spec.Tags),
				Region:                 deployment.Spec.Region,
				SageMakerEndpoint:      deployment.Spec.SageMakerEndpoint,
			},
		}

		desiredModels[k8sModel.ObjectMeta.Name] = k8sModel
		modelNameMap[name] = k8sModel.ObjectMeta.Name
	}

	return desiredModels, modelNameMap, nil
}

// Get a map of model name to model definition. This also validates that all models have unique names.
func (r *modelReconciler) getAndValidateModelMap(models []commonv1.Model) (map[string]*commonv1.Model, error) {
	modelMap := map[string]*commonv1.Model{}
	for _, model := range models {

		if model.Name == nil {
			return nil, fmt.Errorf("All models must have names")
		}
		name := *model.Name

		if _, ok := modelMap[name]; ok {
			return nil, fmt.Errorf("Model names must be unique. Found multiple models with name '%s'", name)
		}

		modelMap[name] = model.DeepCopy()
	}
	return modelMap, nil
}

// Get a map of container name to container definition. This also validates that all containers have unique hostnames.
func (r *modelReconciler) getAndValidateContainerMap(model *commonv1.Model) (map[string]*commonv1.ContainerDefinition, error) {
	containerMap := map[string]*commonv1.ContainerDefinition{}
	for _, container := range model.Containers {

		if container.ContainerHostname == nil {
			return nil, fmt.Errorf("All containers must have hostnames")
		}
		containerHostname := *container.ContainerHostname

		if _, ok := containerMap[containerHostname]; ok {
			return nil, fmt.Errorf("Model '%s' container hostnames must be unique. Found multiple containers with hostname '%s'", *model.Name, containerHostname)
		}

		containerMap[containerHostname] = container.DeepCopy()
	}
	return containerMap, nil
}

// Get the primary container definition for a model.
// If no primary container is specified, and the list of required containers has only one element, promote that container to the
// primary container.
// Else, return an error explaining that a primary container must be specified.
// This also returns an error if the primary container definition cannot be found.
func (r *modelReconciler) getPrimaryContainerDefinition(model *commonv1.Model, containers map[string]*commonv1.ContainerDefinition) (*commonv1.ContainerDefinition, error) {

	// Get primary container name.
	// If none is specified and the list of required containers has only one element, use that container as the primary.
	var primaryContainerName string
	if model.PrimaryContainer != nil {
		primaryContainerName = *model.PrimaryContainer
	} else if len(model.Containers) == 1 {
		primaryContainerName = *model.Containers[0].ContainerHostname
	} else {
		return nil, fmt.Errorf("Unable to determine primary container for model '%s'. Either specify explicitly or provide only one container.", *model.Name)
	}

	var primaryContainer *commonv1.ContainerDefinition
	if found, ok := containers[primaryContainerName]; ok {
		primaryContainer = found.DeepCopy()
	} else {
		return nil, fmt.Errorf("Unknown primary container '%s'", primaryContainerName)
	}

	return primaryContainer, nil
}

// Given a list of models to create, this creates each model in Kubernetes.
func (r *modelReconciler) reconcileModelsToCreate(ctx context.Context, modelsToCreate map[string]*modelv1.Model) error {

	r.log.Info("reconcileModelsToCreate")
	for _, model := range modelsToCreate {
		if err := r.k8sClient.Create(ctx, model); err != nil {
			return errors.Wrapf(err, "Unable to create Kubernetes model '%s'", types.NamespacedName{
				Name:      model.ObjectMeta.GetName(),
				Namespace: model.ObjectMeta.GetNamespace(),
			})
		}
	}

	return nil
}

// Given a list of models to delete, this deletes each model in Kubernetes.
func (r *modelReconciler) reconcileModelsToDelete(ctx context.Context, modelsToDelete map[string]*modelv1.Model) error {

	r.log.Info("reconcileModelsToDelete")
	for _, model := range modelsToDelete {

		if err := r.k8sClient.Delete(ctx, model); err != nil {
			if !apierrs.IsNotFound(err) {
				return errors.Wrapf(err, "Unable to delete Kubernetes model '%s'", types.NamespacedName{
					Name:      model.ObjectMeta.GetName(),
					Namespace: model.ObjectMeta.GetNamespace(),
				})
			}
		}
	}
	return nil
}

// Given a list of models to update, this updates each model in Kubernetes.
func (r *modelReconciler) reconcileModelsToUpdate(ctx context.Context, modelsToUpdate map[string]*modelv1.Model) error {
	r.log.Info("reconcileModelsToUpdate")
	for _, model := range modelsToUpdate {

		if err := r.k8sClient.Update(ctx, model); err != nil {
			return errors.Wrapf(err, "Unable to update Kubernetes model '%s'", types.NamespacedName{
				Name:      model.ObjectMeta.GetName(),
				Namespace: model.ObjectMeta.GetNamespace(),
			})
		}
	}
	return nil
}
