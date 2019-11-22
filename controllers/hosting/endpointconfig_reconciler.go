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
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/common"
	endpointconfigv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/endpointconfig"
	hostingv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/hostingdeployment"
	modelv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/model"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Type that returns an instantiated EndpointConfigReconciler given parameters.
type EndpointConfigReconcilerProvider func(client.Client, logr.Logger) EndpointConfigReconciler

// Helper method to create a EndpointConfigReconciler.
func NewEndpointConfigReconciler(client client.Client, log logr.Logger) EndpointConfigReconciler {
	return &endpointConfigReconciler{
		k8sClient: client,
		log:       log.WithName("EndpointConfigReconciler"),
	}
}

// Helper type that is responsible for reconciling EndpointConfigs of an endpoint.
type EndpointConfigReconciler interface {
	Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, shouldDeleteUnusedResources bool) error
	GetSageMakerEndpointConfigName(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (string, error)
}

// Concrete implementation of EndpointConfigReconciler.
type endpointConfigReconciler struct {
	k8sClient client.Client
	log       logr.Logger
}

// Make sure at compile time endpointConfigReconciler implements EndpointConfigReconciler.
var _ EndpointConfigReconciler = (*endpointConfigReconciler)(nil)

// Reconcile desired EndpointConfig with actual EndpointConfig.
// This will create, delete and update an EndpointConfig in Kubernetes.
// The created EndpointConfig will point to actual SageMaker models. This function will
// obtain the SageMaker model names from the Kubernetes model statuses.
//
// The parameter shouldDeleteUnusedResources controls whether unnecessary endpoint configs are deleted. This is useful when updating Endpoints.
func (r *endpointConfigReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, shouldDeleteUnusedResources bool) error {

	r.log.Info("Reconciling EndpointConfigs")

	var err error
	var desiredEndpointConfig *endpointconfigv1.EndpointConfig

	if desiredEndpointConfig, err = r.extractDesiredEndpointConfigFromHostingDeployment(ctx, desiredDeployment); err != nil {
		return errors.Wrap(err, "Unable to interpret HostingDeployment endpoint config")
	}

	r.log.Info("Desired endpoint config", "desired", desiredEndpointConfig)

	var actualEndpointConfigs map[string]*endpointconfigv1.EndpointConfig
	if actualEndpointConfigs, err = r.getActualEndpointConfigsForHostingDeployment(ctx, desiredDeployment); err != nil {
		return errors.Wrap(err, "Unable to get actual endpoint config")
	}

	r.log.Info("Actual endpoint config", "actual", actualEndpointConfigs)

	actions := r.determineActionForEndpointConfig(desiredEndpointConfig, actualEndpointConfigs)

	for action, endpointConfigs := range actions {

		r.log.Info("action", "action", action, "ecs", getEndpointConfigNamesFromMap(endpointConfigs))
		switch action {
		case NeedsNoop:
			// Do nothing.
		case NeedsCreate:
			for _, endpointConfig := range endpointConfigs {
				if err := r.k8sClient.Create(ctx, endpointConfig); err != nil {
					return errors.Wrapf(err, "Unable to create Kubernetes EndpointConfig '%s'", types.NamespacedName{
						Name:      endpointConfig.ObjectMeta.Name,
						Namespace: endpointConfig.ObjectMeta.Namespace,
					})
				}
			}
		case NeedsDelete:
			if !shouldDeleteUnusedResources {
				r.log.Info("Not deleting unused resources", "shouldDeleteUnusedResources", shouldDeleteUnusedResources)
				break
			}
			for _, endpointConfig := range endpointConfigs {
				if err := r.k8sClient.Delete(ctx, endpointConfig); err != nil {
					if !apierrs.IsNotFound(err) {
						return errors.Wrapf(err, "Unable to delete Kubernetes EndpointConfig '%s'", types.NamespacedName{
							Name:      endpointConfig.ObjectMeta.Name,
							Namespace: endpointConfig.ObjectMeta.Namespace,
						})
					}
				}
			}
		case NeedsUpdate:
			for _, endpointConfig := range endpointConfigs {
				if err = r.k8sClient.Update(ctx, endpointConfig); err != nil {
					return errors.Wrapf(err, "Unable to update Kubernetes EndpointConfig '%s'", types.NamespacedName{
						Name:      endpointConfig.ObjectMeta.Name,
						Namespace: endpointConfig.ObjectMeta.Namespace,
					})
				}
			}
		}

	}
	return nil
}

// Helper method to get a slice of EndpointConfig names from a map.
func getEndpointConfigNamesFromMap(m map[string]*endpointconfigv1.EndpointConfig) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Return a Kubernetes object representing the desired EndpointConfig. This performs some validation
// on the ProductionVariants, and will return any error.
func (r *endpointConfigReconciler) extractDesiredEndpointConfigFromHostingDeployment(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (*endpointconfigv1.EndpointConfig, error) {

	// If deletion is requested, there is no desired endpoint config.
	if !desiredDeployment.ObjectMeta.GetDeletionTimestamp().IsZero() {
		return nil, nil
	}

	productionVariants := []commonv1.ProductionVariant{}

	for _, variant := range desiredDeployment.Spec.ProductionVariants {

		if variant.VariantName == nil {
			return nil, fmt.Errorf("ProductionVariant has nil VariantName")
		}

		if variant.ModelName == nil {
			return nil, fmt.Errorf("ProductionVariant '%s' has nil ModelName", *variant.VariantName)
		}

		if resolved, err := r.resolveSageMakerModelName(ctx, *variant.ModelName, desiredDeployment); err != nil {
			return nil, err
		} else {
			variant.ModelName = resolved
		}

		productionVariants = append(productionVariants, variant)

	}

	// TODO need to rename production variant names to remove "initial"

	namespacedName := GetKubernetesEndpointConfigNamespacedName(*desiredDeployment)

	// Add labels to endpointconfig that indicate which particular HostingDeployment
	// owns it.
	ownershipLabels := GetResourceOwnershipLabelsForHostingDeployment(*desiredDeployment)

	desiredEndpointConfig := endpointconfigv1.EndpointConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    ownershipLabels,
		},
		Spec: endpointconfigv1.EndpointConfigSpec{
			ProductionVariants: productionVariants,
			KmsKeyId:           GetOrDefault(desiredDeployment.Spec.KmsKeyId, ""),
			Tags:               commonv1.DeepCopyTagSlice(desiredDeployment.Spec.Tags),
			Region:             desiredDeployment.Spec.Region,
		},
	}

	return &desiredEndpointConfig, nil
}

// Get the SageMaker model name for a given model.
// This will fetch the Kubernetes model to obtain the SageMaker model name from the status.
func (r *endpointConfigReconciler) resolveSageMakerModelName(ctx context.Context, k8sModelName string, desiredDeployment *hostingv1.HostingDeployment) (*string, error) {

	if desiredDeployment == nil {
		return nil, fmt.Errorf("Unable to resolve SageMaker model name for nil deployment")
	}

	namespacedName := GetKubernetesModelNamespacedName(k8sModelName, *desiredDeployment)

	var model modelv1.Model
	if err := r.k8sClient.Get(ctx, namespacedName, &model); err != nil {
		return nil, errors.Wrapf(err, "Unable to resolve SageMaker model name for model '%s'", namespacedName)
	}

	if model.Status.SageMakerModelName == "" {
		return nil, fmt.Errorf("Model '%s' does not have a SageMakerModelName", namespacedName)
	}

	return &model.Status.SageMakerModelName, nil
}

// Get the idempotent name of the EndpointConfig in Kubernetes.
// Kubernetes resources can have names up to 253 characters long.
// The characters allowed in names are: digits (0-9), lower case letters (a-z), -, and .
func GetKubernetesEndpointConfigNamespacedName(deployment hostingv1.HostingDeployment) types.NamespacedName {
	k8sMaxLen := 253
	name := deployment.ObjectMeta.GetName()
	uid := strings.Replace(string(deployment.ObjectMeta.GetUID()), "-", "", -1)
	generation := strconv.FormatInt(deployment.ObjectMeta.GetGeneration(), 10)

	requiredPostfix := "-" + generation + "-" + uid
	endpointConfigName := name + requiredPostfix

	if len(endpointConfigName) > k8sMaxLen {
		endpointConfigName = name[:k8sMaxLen-len(requiredPostfix)] + requiredPostfix
	}

	return types.NamespacedName{
		Name:      endpointConfigName,
		Namespace: deployment.ObjectMeta.GetNamespace(),
	}
}

// Get the existing Kubernetes EndpointConfig. If it does not exist, return nil.
func (r *endpointConfigReconciler) getActualEndpointConfigsForHostingDeployment(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (map[string]*endpointconfigv1.EndpointConfig, error) {
	ownershipLabelSelector := client.MatchingLabels(GetResourceOwnershipLabelsForHostingDeployment(*desiredDeployment))

	// TODO need to support pagination. See modelReconciler.getActualModelsForHostingDeployment
	endpointConfigs := &endpointconfigv1.EndpointConfigList{}
	if err := r.k8sClient.List(ctx, endpointConfigs, ownershipLabelSelector); err != nil {
		return nil, errors.Wrap(err, "Unable to get existing EndpointConfigs")
	}

	actualEndpointConfigs := map[string]*endpointconfigv1.EndpointConfig{}
	for i, endpointConfig := range endpointConfigs.Items {
		actualEndpointConfigs[endpointConfig.ObjectMeta.Name] = &endpointConfigs.Items[i]
	}

	return actualEndpointConfigs, nil
}

// Determine the action necessary to bring actual state to desired state.
func (r *endpointConfigReconciler) determineActionForEndpointConfig(desired *endpointconfigv1.EndpointConfig, actualEndpointConfigs map[string]*endpointconfigv1.EndpointConfig) map[ReconcileAction]map[string]*endpointconfigv1.EndpointConfig {

	// Put desired into a map even though it is singular.
	// This makes the following logic closer to models and easier to follow.
	desiredEndpointConfigs := map[string]*endpointconfigv1.EndpointConfig{}
	if desired != nil {
		desiredEndpointConfigs[desired.ObjectMeta.GetName()] = desired
	}

	actions := map[ReconcileAction]map[string]*endpointconfigv1.EndpointConfig{
		NeedsCreate: map[string]*endpointconfigv1.EndpointConfig{},
		NeedsDelete: map[string]*endpointconfigv1.EndpointConfig{},
		NeedsNoop:   map[string]*endpointconfigv1.EndpointConfig{},
		NeedsUpdate: map[string]*endpointconfigv1.EndpointConfig{},
	}
	visited := map[string]*endpointconfigv1.EndpointConfig{}

	for name, desiredEndpointConfig := range desiredEndpointConfigs {
		if actualEndpointConfig, exists := actualEndpointConfigs[name]; exists {
			if reflect.DeepEqual(desiredEndpointConfig.Spec, actualEndpointConfig.Spec) {
				actions[NeedsNoop][name] = desiredEndpointConfig
			} else {
				targetEndpointConfig := actualEndpointConfigs[name].DeepCopy()
				targetEndpointConfig.Spec = desiredEndpointConfig.Spec
				actions[NeedsUpdate][name] = targetEndpointConfig
			}
		} else {
			actions[NeedsCreate][name] = desiredEndpointConfig
		}

		visited[name] = desiredEndpointConfig
	}

	for name, actualEndpointConfig := range actualEndpointConfigs {
		if _, visited := visited[name]; visited {
			continue
		}
		actions[NeedsDelete][name] = actualEndpointConfig
	}

	return actions
}

// Get the SageMaker EndpointConfig name from the status of the Kubernetes EndpointConfig.
// Returns error if any failure occurred.
func (r *endpointConfigReconciler) GetSageMakerEndpointConfigName(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (string, error) {

	var err error
	var desiredEndpointConfig *endpointconfigv1.EndpointConfig
	if desiredEndpointConfig, err = r.extractDesiredEndpointConfigFromHostingDeployment(ctx, desiredDeployment); err != nil {
		return "", errors.Wrap(err, "Unable to interpret HostingDeployment endpoint config")
	}

	if desiredEndpointConfig == nil {
		return "", nil
	}

	if name, err := r.getSageMakerEndpointConfigName(ctx, desiredEndpointConfig); err != nil {
		return "", errors.Wrap(err, "Unable to get SageMaker EndpointConfig name")
	} else {
		return name, nil
	}
}

func (r *endpointConfigReconciler) getSageMakerEndpointConfigName(ctx context.Context, desiredEndpointConfig *endpointconfigv1.EndpointConfig) (string, error) {

	key := types.NamespacedName{
		Name:      desiredEndpointConfig.ObjectMeta.GetName(),
		Namespace: desiredEndpointConfig.ObjectMeta.GetNamespace(),
	}

	var existingEndpointConfig endpointconfigv1.EndpointConfig
	if err := r.k8sClient.Get(ctx, key, &existingEndpointConfig); err != nil {
		if apierrs.IsNotFound(err) {
			return "", errors.Wrapf(err, "Awaiting endpoint config creation: '%s'", key)
		} else {
			return "", errors.Wrapf(err, "Unable to determine if endpoint config '%s' was created", key)
		}
	}

	name := existingEndpointConfig.Status.SageMakerEndpointConfigName
	if name == "" {
		return "", fmt.Errorf("Awaiting EndpointConfig name for '%s' to not be empty", key)
	} else {
		return name, nil
	}
}
