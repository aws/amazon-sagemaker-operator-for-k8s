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

package controllertest

import (
	"context"
	"fmt"

	"github.com/adammck/venv"
	. "github.com/onsi/ginkgo"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// +kubebuilder:scaffold:imports
)

func CreateMockAwsConfigLoader() AwsConfigLoader {
	return NewAwsConfigLoaderForEnv(venv.Mock())
}

// Create a provider that creates a mock SageMaker client.
func CreateMockSageMakerClientProvider(sageMakerClient sagemakeriface.ClientAPI) SageMakerClientProvider {
	return func(_ aws.Config) sagemakeriface.ClientAPI {
		return sageMakerClient
	}
}

// Helper function to create a ctrl.Request.
func CreateReconciliationRequest(name string, namespace string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// Mock of Kubernetes client.Client that always returns error when Get is invoked.
// TODO Should merge all k8s client mocks to a single, flexible mock.
type FailToGetK8sClient struct {
	client.Writer
	client.Reader
	client.StatusClient
}

// Always return error.
func (m FailToGetK8sClient) Get(_ context.Context, _ client.ObjectKey, _ runtime.Object) error {
	return fmt.Errorf("unable to Get")
}

// Mock of Kubernetes client.Client that always returns error when List is invoked.
type FailToListK8sClient struct {
	client.Writer
	client.Reader
	client.StatusClient
}

// Always return error.
func (m FailToListK8sClient) List(_ context.Context, _ runtime.Object, _ ...client.ListOption) error {
	return fmt.Errorf("unable to List")
}

// Mock of Kubernetes client.Client that always returns error when Update is invoked.
type FailToUpdateK8sClient struct {
	client.Writer
	client.Reader
	client.StatusClient

	ActualClient client.Client
}

// Get should work normally.
func (m FailToUpdateK8sClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return m.ActualClient.Get(ctx, key, obj)
}

// Always return error on Update.
func (m FailToUpdateK8sClient) Update(_ context.Context, _ runtime.Object, _ ...client.UpdateOption) error {
	return fmt.Errorf("unable to Update")
}

// Mock of Kubernetes client.Client that always returns error when Create is invoked.
type FailToCreateK8sClient struct {
	client.Writer
	client.Reader
	client.StatusClient

	ActualClient client.Client
}

// Get should work normally.
func (m FailToCreateK8sClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return m.ActualClient.Get(ctx, key, obj)
}

// List should work normally.
func (m FailToCreateK8sClient) List(ctx context.Context, obj runtime.Object, opts ...client.ListOption) error {
	return m.ActualClient.List(ctx, obj, opts...)
}

// Always return error on Create.
func (m FailToCreateK8sClient) Create(_ context.Context, _ runtime.Object, _ ...client.CreateOption) error {
	return fmt.Errorf("unable to Create")
}

// Make test fail on Get.
type FailTestOnGetK8sClient struct {
	client.Writer
	client.Reader
	client.StatusClient
}

// Make test fail.
func (m FailTestOnGetK8sClient) Get(_ context.Context, _ client.ObjectKey, _ runtime.Object) error {
	Fail("FailTestOnGetK8sClient.Get should never be called")
	return nil
}

// Make test fail on Create.
type FailTestOnCreateK8sClient struct {
	client.Writer
	client.Reader
	client.StatusClient

	ActualClient client.Client
}

// Get should work normally.
func (m FailTestOnCreateK8sClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return m.ActualClient.Get(ctx, key, obj)
}

// Update should work normally.
func (m FailTestOnCreateK8sClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return m.ActualClient.Update(ctx, obj, opts...)
}

// Make test fail.
func (m FailTestOnCreateK8sClient) Create(_ context.Context, _ runtime.Object, _ ...client.CreateOption) error {
	Fail("FailTestOnCreateK8sClient.Createshould never be called")
	return nil
}
