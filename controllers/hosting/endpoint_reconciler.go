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
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/sdkutil"
	"go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/sdkutil/clientwrapper"

	endpointconfigv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/endpointconfig"
	hostingv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/hostingdeployment"
)

// Type that returns an instantiated EndpointReconciler given parameters.
type EndpointReconcilerProvider func(client.Client, logr.Logger, clientwrapper.SageMakerClientWrapper) EndpointReconciler

// Helper method to create a EndpointReconciler.
func NewEndpointReconciler(client client.Client, log logr.Logger, sageMakerClient clientwrapper.SageMakerClientWrapper) EndpointReconciler {
	return &endpointReconciler{
		k8sClient:       client,
		log:             log.WithName("EndpointReconciler"),
		sageMakerClient: sageMakerClient,
	}
}

// Helper type that is responsible for reconciling Endpoints of an endpoint.
type EndpointReconciler interface {
	Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, actualDeployment *sagemaker.DescribeEndpointOutput) error
}

// Concrete implementation of EndpointReconciler.
type endpointReconciler struct {
	EndpointReconciler

	k8sClient       client.Client
	log             logr.Logger
	sageMakerClient clientwrapper.SageMakerClientWrapper
}

// Reconcile actual state with desired state.
// If the SageMaker Endpoint does not exist, create it. This obtains the necessary EndpointConfigName from
// the Kubernetes EndpointConfig.
func (r *endpointReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, actualDeployment *sagemaker.DescribeEndpointOutput) error {

	r.log.Info("Reconciling Endpoints")

	var err error
	var desiredEndpoint *sagemaker.CreateEndpointInput
	if desiredEndpoint, err = r.getDesiredEndpoint(ctx, desiredDeployment); err != nil {
		return errors.Wrap(err, "Unable to determine desired endpoint")
	}

	r.log.Info("Desired endpoint", "endpoint", desiredEndpoint)

	var needsCreate bool
	if needsCreate, err = r.determineIfNeedsCreate(desiredEndpoint, actualDeployment); err != nil {
		return errors.Wrap(err, "Unable to determine if the Endpoint needs to be created.")
	}

	r.log.Info("Endpoint needs create", "needsCreate", needsCreate)

	var needsDelete bool
	needsDelete = !desiredDeployment.ObjectMeta.GetDeletionTimestamp().IsZero()

	if needsCreate && !needsDelete {
		if _, err := r.sageMakerClient.CreateEndpoint(ctx, desiredEndpoint); err != nil {
			return errors.Wrap(err, "Unable to create Endpoint")
		}
	}

	// Try to delete only if its marked for deletion and not already being deleted
	if needsDelete { //&& (actualDeployment.EndpointStatus != DeletingEndpointStatus) { TODO(cade) fix this when doing endpoint update.
		// Client returns err=nil in case of 404, however returns error if update-in-progress
		if _, err := r.sageMakerClient.DeleteEndpoint(ctx, desiredEndpoint.EndpointName); err != nil {
			return errors.Wrap(err, "Unable to delete Endpoint")
		}
	}

	// TODO Need creation verification strategy that doesn't use exponential backoff.

	return nil
}

// Get the desired Endpoint.
func (r *endpointReconciler) getDesiredEndpoint(ctx context.Context, deployment *hostingv1.HostingDeployment) (*sagemaker.CreateEndpointInput, error) {

	var err error
	var endpointConfigName *string
	if endpointConfigName, err = r.resolveSageMakerEndpointConfigName(ctx, deployment); err != nil {
		// If we are on create path and failed to resolve endpointconfig
		// then we need to raise the error.
		// If we are on delete path then error does not matter.
		if deployment.ObjectMeta.GetDeletionTimestamp().IsZero() {
			return nil, err
		}
	}

	endpointName := GetSageMakerEndpointName(*deployment)

	desiredEndpoint := sagemaker.CreateEndpointInput{
		EndpointConfigName: endpointConfigName,
		EndpointName:       &endpointName,
		Tags:               sdkutil.ConvertTagSliceToSageMakerTagSlice(deployment.Spec.Tags),
	}

	return &desiredEndpoint, nil
}

// Get the SageMaker Endpoint name given a HostingDeployment.
// TODO Make sure this fits SageMaker validation https://docs.aws.amazon.com/sagemaker/latest/dg/API_CreateEndpoint.html#SageMaker-CreateEndpoint-request-EndpointName
func GetSageMakerEndpointName(desiredDeployment hostingv1.HostingDeployment) string {
	SMMaxLen := 63
	name := desiredDeployment.ObjectMeta.GetName()
	uid := strings.Replace(string(desiredDeployment.ObjectMeta.GetUID()), "-", "", -1)
	smEndpointName := name + "-" + uid
	if len(smEndpointName) > SMMaxLen {
		smEndpointName = name[:SMMaxLen-len(uid)-1] + "-" + uid
	}
	return smEndpointName
}

// For a given HostingDeployment, get the SageMaker EndpointConfig name.
// This works by reading the status of the EndpointConfig Kubernetes object.
func (r *endpointReconciler) resolveSageMakerEndpointConfigName(ctx context.Context, deployment *hostingv1.HostingDeployment) (*string, error) {

	if deployment == nil {
		return nil, fmt.Errorf("Unable to resolve SageMaker EndpointConfig name for nil deployment")
	}

	namespacedName := GetKubernetesEndpointConfigNamespacedName(*deployment)

	var endpointConfig endpointconfigv1.EndpointConfig
	if err := r.k8sClient.Get(ctx, namespacedName, &endpointConfig); err != nil {
		return nil, errors.Wrapf(err, "Unable to resolve SageMaker EndpointConfig name for EndpointConfig '%s'", namespacedName)
	}

	if endpointConfig.Status.SageMakerEndpointConfigName == "" {
		return nil, fmt.Errorf("EndpointConfig '%s' does not have a SageMakerEndpointConfigName", namespacedName)
	}

	return &endpointConfig.Status.SageMakerEndpointConfigName, nil
}

// Determine if the Endpoint needs to be created.
func (r *endpointReconciler) determineIfNeedsCreate(desiredEndpoint *sagemaker.CreateEndpointInput, actualState *sagemaker.DescribeEndpointOutput) (bool, error) {

	if actualState == nil {
		return true, nil
	}

	// TODO deep equality comparison to catch when the actual state drifted away from desired state.
	return false, nil
}
