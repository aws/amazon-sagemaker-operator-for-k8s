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
	Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) error
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
// After creation, it will verify that the EndpointConfig is created in SageMaker.
// The same process will follow for delete and update except verification.
func (r *endpointConfigReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) error {

	r.log.Info("Reconciling EndpointConfigs")

	var err error
	var desiredEndpointConfig *endpointconfigv1.EndpointConfig

	if desiredEndpointConfig, err = r.extractDesiredEndpointConfigFromHostingDeployment(ctx, desiredDeployment); err != nil {
		return errors.Wrap(err, "Unable to interpret HostingDeployment endpoint config")
	}

	r.log.Info("Desired endpoint config", "desired", desiredEndpointConfig)

	var actualEndpointConfig *endpointconfigv1.EndpointConfig
	if actualEndpointConfig, err = r.getActualEndpointConfigForHostingDeployment(ctx, desiredDeployment); err != nil {
		return errors.Wrap(err, "Unable to get actual endpoint config")
	}

	r.log.Info("Actual endpoint config", "actual", actualEndpointConfig)

	action := r.determineActionForEndpointConfig(desiredEndpointConfig, actualEndpointConfig)

	r.log.Info("Action for endpoint config", "action", action)

	if action == NeedsDelete {

		if err := r.k8sClient.Delete(ctx, actualEndpointConfig); err != nil {
			if !apierrs.IsNotFound(err) {
				return errors.Wrapf(err, "Unable to delete Kubernetes EndpointConfig '%s'", types.NamespacedName{
					Name:      actualEndpointConfig.ObjectMeta.Name,
					Namespace: actualEndpointConfig.ObjectMeta.Namespace,
				})
			}
		}

	} else if action == NeedsCreate {
		if err := r.k8sClient.Create(ctx, desiredEndpointConfig); err != nil {
			return errors.Wrapf(err, "Unable to create Kubernetes EndpointConfig '%s'", types.NamespacedName{
				Name:      desiredEndpointConfig.ObjectMeta.Name,
				Namespace: desiredEndpointConfig.ObjectMeta.Namespace,
			})
		}
	} else if action == NeedsUpdate {

		toUpdate := actualEndpointConfig.DeepCopy()
		toUpdate.Spec = desiredEndpointConfig.Spec

		if err = r.k8sClient.Update(ctx, toUpdate); err != nil {
			return errors.Wrapf(err, "Unable to update Kubernetes EndpointConfig '%s'", types.NamespacedName{
				Name:      toUpdate.ObjectMeta.Name,
				Namespace: toUpdate.ObjectMeta.Namespace,
			})
		}
	}

	return nil
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

	desiredEndpointConfig := endpointconfigv1.EndpointConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
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
	endpointConfigName := name + "-" + uid

	if len(endpointConfigName) > k8sMaxLen {
		endpointConfigName = name[:k8sMaxLen-len(uid)-1] + "-" + uid
	}

	return types.NamespacedName{
		Name:      endpointConfigName,
		Namespace: deployment.ObjectMeta.GetNamespace(),
	}
}

// Get the existing Kubernetes EndpointConfig. If it does not exist, return nil.
func (r *endpointConfigReconciler) getActualEndpointConfigForHostingDeployment(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (*endpointconfigv1.EndpointConfig, error) {

	key := GetKubernetesEndpointConfigNamespacedName(*desiredDeployment)
	var actualEndpointConfig endpointconfigv1.EndpointConfig

	if err := r.k8sClient.Get(ctx, key, &actualEndpointConfig); err != nil {
		if apierrs.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, errors.Wrapf(err, "Unable to get existing endpoint config '%s'", key)
		}
	}

	return &actualEndpointConfig, nil
}

// Determine the action necessary to bring actual state to desired state.
func (r *endpointConfigReconciler) determineActionForEndpointConfig(desired, actual *endpointconfigv1.EndpointConfig) ReconcileAction {

	if desired == nil {
		if actual != nil {
			return NeedsDelete
		}
		return NeedsNoop
	}

	if actual == nil {
		return NeedsCreate
	}

	if reflect.DeepEqual(desired.Spec, actual.Spec) {
		return NeedsNoop
	}

	return NeedsUpdate
}

// Get the SageMaker EndpointConfig name from the status of the Kubernetes EndpointConfig.
// Returns error if any failure occurred.
func (r *endpointConfigReconciler) GetSageMakerEndpointConfigName(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (string, error) {

	var err error
	var actualEndpointConfig *endpointconfigv1.EndpointConfig
	if actualEndpointConfig, err = r.getActualEndpointConfigForHostingDeployment(ctx, desiredDeployment); err != nil {
		return "", errors.Wrap(err, "Unable to get actual endpoint config")
	}

	if actualEndpointConfig == nil {
		return "", nil
	}

	if name, err := r.getSageMakerEndpointConfigName(ctx, actualEndpointConfig); err != nil {
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
