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
	"time"

	. "container/list"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/controllertest"
	"go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/sdkutil/clientwrapper"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	commonv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/common"
	hostingv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/hostingdeployment"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("Reconciling a HostingDeployment while failing to get the Kubernetes job", func() {

	var (
		sageMakerClient sagemakeriface.ClientAPI
	)

	BeforeEach(func() {
		sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).Build()
	})

	It("should not requeue if the HostingDeployment does not exist", func() {
		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &mockEndpointConfigReconciler{}, &mockEndpointReconciler{}, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should requeue if there was an error", func() {
		mockK8sClient := FailToGetK8sClient{}
		controller := createReconciler(mockK8sClient, sageMakerClient, &mockModelReconciler{}, &mockEndpointConfigReconciler{}, &mockEndpointReconciler{}, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Reconciling a HostingDeployment when the endpoint does not exist", func() {

	var (
		receivedRequests List
		sageMakerClient  sagemakeriface.ClientAPI
		deployment       *hostingv1.HostingDeployment
	)

	BeforeEach(func() {
		receivedRequests = List{}
		mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
		sageMakerClient = mockSageMakerClientBuilder.
			AddDescribeEndpointErrorResponse("ValidationException", "Could not find endpoint xyz", 400, "request id").
			Build()

		deployment = createDeploymentWithGeneratedNames()
		err := k8sClient.Create(context.Background(), deployment)
		Expect(err).ToNot(HaveOccurred())

	})

	It("should call ModelReconciler.Reconcile with correct parameters", func() {
		Skip("Fix me later")
		modelReconciler := mockModelReconciler{}
		modelReconciler.DesiredDeployments = &List{}

		controller := createReconciler(k8sClient, sageMakerClient, &modelReconciler, &mockEndpointConfigReconciler{}, &mockEndpointReconciler{}, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		controller.Reconcile(request)

		Expect(modelReconciler.DesiredDeployments.Len()).To(Equal(1))
		desiredDeployment := modelReconciler.DesiredDeployments.Front().Value.(*hostingv1.HostingDeployment)
		Expect(desiredDeployment.Spec).To(Equal(deployment.Spec))
	})

	It("should requeue if ModelReconciler.Reconcile failed", func() {
		Skip("Fix me later")
		modelReconciler := mockModelReconciler{}
		modelReconciler.ReconcileReturnValues = &List{}
		modelReconciler.ReconcileReturnValues.PushBack(fmt.Errorf("mock error"))

		controller := createReconciler(k8sClient, sageMakerClient, &modelReconciler, &mockEndpointConfigReconciler{}, &mockEndpointReconciler{}, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(modelReconciler.ReconcileReturnValues.Len()).To(Equal(0))
	})

	It("should correctly update the status if Model.Reconcile failed", func() {
		Skip("Fix me later")
		errorMessage := "mock error"
		modelReconciler := mockModelReconciler{}
		modelReconciler.ReconcileReturnValues = &List{}
		modelReconciler.ReconcileReturnValues.PushBack(fmt.Errorf(errorMessage))

		controller := createReconciler(k8sClient, sageMakerClient, &modelReconciler, &mockEndpointConfigReconciler{}, &mockEndpointReconciler{}, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		_, err := controller.Reconcile(request)
		Expect(err).ToNot(HaveOccurred())

		var updatedDeployment hostingv1.HostingDeployment
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: deployment.ObjectMeta.Namespace,
			Name:      deployment.ObjectMeta.Name,
		}, &updatedDeployment)
		Expect(err).ToNot(HaveOccurred())

		Expect(updatedDeployment.Status.Additional).To(ContainSubstring(errorMessage))
	})

	It("should call EndpointConfigReconciler.Reconcile with correct parameters", func() {
		Skip("Fix me later")
		endpointConfigReconciler := mockEndpointConfigReconciler{}
		endpointConfigReconciler.DesiredDeployments = &List{}

		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &endpointConfigReconciler, &mockEndpointReconciler{}, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		controller.Reconcile(request)

		Expect(endpointConfigReconciler.DesiredDeployments.Len()).To(Equal(1))
		desiredDeployment := endpointConfigReconciler.DesiredDeployments.Front().Value.(*hostingv1.HostingDeployment)
		Expect(desiredDeployment.Spec).To(Equal(deployment.Spec))
	})

	It("should requeue if EndpointConfigReconciler.Reconcile failed", func() {
		Skip("Fix me later")
		endpointConfigReconciler := mockEndpointConfigReconciler{}
		endpointConfigReconciler.ReconcileReturnValues = &List{}
		endpointConfigReconciler.ReconcileReturnValues.PushBack(fmt.Errorf("mock error"))

		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &endpointConfigReconciler, &mockEndpointReconciler{}, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(endpointConfigReconciler.ReconcileReturnValues.Len()).To(Equal(0))
	})

	It("should correctly update the status if EndpointConfigReconciler.Reconcile failed", func() {
		Skip("Fix me later")
		errorMessage := "mock error"
		endpointConfigReconciler := mockEndpointConfigReconciler{}
		endpointConfigReconciler.ReconcileReturnValues = &List{}
		endpointConfigReconciler.ReconcileReturnValues.PushBack(fmt.Errorf(errorMessage))

		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &endpointConfigReconciler, &mockEndpointReconciler{}, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		_, err := controller.Reconcile(request)
		Expect(err).ToNot(HaveOccurred())

		var updatedDeployment hostingv1.HostingDeployment
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: deployment.ObjectMeta.Namespace,
			Name:      deployment.ObjectMeta.Name,
		}, &updatedDeployment)
		Expect(err).ToNot(HaveOccurred())

		Expect(updatedDeployment.Status.Additional).To(ContainSubstring(errorMessage))
	})

	It("should call EndpointReconciler.Reconcile with correct parameters", func() {
		Skip("Fix me later")
		endpointReconciler := mockEndpointReconciler{}
		endpointReconciler.DesiredDeployments = &List{}
		endpointReconciler.ActualDeployments = &List{}

		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &mockEndpointConfigReconciler{}, &endpointReconciler, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		controller.Reconcile(request)

		Expect(endpointReconciler.DesiredDeployments.Len()).To(Equal(1))
		desiredDeployment := endpointReconciler.DesiredDeployments.Front().Value.(*hostingv1.HostingDeployment)
		Expect(desiredDeployment.Spec).To(Equal(deployment.Spec))

		// Since the endpoint does not exist, we check that the reconciler received an empty actual state.
		Expect(endpointReconciler.ActualDeployments.Len()).To(Equal(1))
		actualDeployment := endpointReconciler.ActualDeployments.Front().Value.(*sagemaker.DescribeEndpointOutput)
		Expect(actualDeployment).To(BeNil())
	})

	It("should requeue if EndpointReconciler.Reconcile failed", func() {
		Skip("Fix me later")
		endpointReconciler := mockEndpointReconciler{}
		endpointReconciler.ReconcileReturnValues = &List{}
		endpointReconciler.ReconcileReturnValues.PushBack(fmt.Errorf("mock error"))

		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &mockEndpointConfigReconciler{}, &endpointReconciler, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(endpointReconciler.ReconcileReturnValues.Len()).To(Equal(0))
	})

	It("should correctly update the status if EndpointReconciler.Reconcile failed", func() {
		Skip("Fix me later")
		errorMessage := "mock error"
		endpointReconciler := mockEndpointReconciler{}
		endpointReconciler.ReconcileReturnValues = &List{}
		endpointReconciler.ReconcileReturnValues.PushBack(fmt.Errorf(errorMessage))

		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &mockEndpointConfigReconciler{}, &endpointReconciler, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		_, err := controller.Reconcile(request)
		Expect(err).ToNot(HaveOccurred())

		var updatedDeployment hostingv1.HostingDeployment
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: deployment.ObjectMeta.Namespace,
			Name:      deployment.ObjectMeta.Name,
		}, &updatedDeployment)
		Expect(err).ToNot(HaveOccurred())

		Expect(updatedDeployment.Status.Additional).To(ContainSubstring(errorMessage))
	})

	It("should update the status", func() {
		Skip("Fix me later")
		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &mockEndpointConfigReconciler{}, &mockEndpointReconciler{}, "1s")
		request := CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

		_, err := controller.Reconcile(request)
		Expect(err).ToNot(HaveOccurred())

		var updatedDeployment hostingv1.HostingDeployment
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: deployment.ObjectMeta.Namespace,
			Name:      deployment.ObjectMeta.Name,
		}, &updatedDeployment)
		Expect(err).ToNot(HaveOccurred())

		Expect(updatedDeployment.Status.EndpointStatus).To(Equal(PreparingEndpointStatus))
	})
})

var _ = Describe("Reconciling a HostingDeployment when the endpoint exists", func() {

	var (
		receivedRequests           List
		mockSageMakerClientBuilder *MockSageMakerClientBuilder
		deployment                 *hostingv1.HostingDeployment
	)

	BeforeEach(func() {
		receivedRequests = List{}
		mockSageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		deployment = createDeploymentWithGeneratedNames()
		err := k8sClient.Create(context.Background(), deployment)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should correctly populate the status", func() {
		Skip("Fix me later")
		endpointArn := "endpoint-arn"
		endpointStatus := "InService"
		variantName := "variant-A"
		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeEndpointResponse(sagemaker.DescribeEndpointOutput{
				EndpointArn:    &endpointArn,
				EndpointStatus: sagemaker.EndpointStatus(endpointStatus),
				ProductionVariants: []sagemaker.ProductionVariantSummary{
					{
						VariantName: &variantName,
					},
				},
			}).
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, &mockModelReconciler{}, &mockEndpointConfigReconciler{}, &mockEndpointReconciler{}, "1s")
		controller.Reconcile(CreateReconciliationRequest(deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace))

		var updatedDeployment hostingv1.HostingDeployment
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: deployment.ObjectMeta.Namespace,
			Name:      deployment.ObjectMeta.Name,
		}, &updatedDeployment)
		Expect(err).ToNot(HaveOccurred())

		Expect(updatedDeployment.Status.EndpointStatus).To(Equal(endpointStatus))
		Expect(updatedDeployment.Status.EndpointArn).To(Equal(endpointArn))
		Expect(len(updatedDeployment.Status.ProductionVariants)).To(Equal(1))
		Expect(updatedDeployment.Status.ProductionVariants[0]).ToNot(BeNil())
		Expect(updatedDeployment.Status.ProductionVariants[0].VariantName).ToNot(BeNil())
		Expect(*updatedDeployment.Status.ProductionVariants[0].VariantName).To(Equal(variantName))
	})

})

func createReconciler(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.ClientAPI, modelReconciler ModelReconciler, endpointConfigReconciler EndpointConfigReconciler, endpointReconciler EndpointReconciler, pollIntervalStr string) HostingDeploymentReconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return HostingDeploymentReconciler{
		Client:                         k8sClient,
		Log:                            ctrl.Log,
		PollInterval:                   pollInterval,
		createSageMakerClient:          CreateMockSageMakerClientProvider(sageMakerClient),
		awsConfigLoader:                CreateMockAwsConfigLoader(),
		createModelReconciler:          createModelReconcilerProvider(modelReconciler),
		createEndpointConfigReconciler: createEndpointConfigReconcilerProvider(endpointConfigReconciler),
		createEndpointReconciler:       createEndpointReconcilerProvider(endpointReconciler),
	}
}

func createModelReconcilerProvider(modelReconciler ModelReconciler) ModelReconcilerProvider {
	return func(_ client.Client, _ logr.Logger) ModelReconciler {
		return modelReconciler
	}
}

func createEndpointConfigReconcilerProvider(endpointConfigReconciler EndpointConfigReconciler) EndpointConfigReconcilerProvider {
	return func(_ client.Client, _ logr.Logger) EndpointConfigReconciler {
		return endpointConfigReconciler
	}
}

func createEndpointReconcilerProvider(endpointReconciler EndpointReconciler) EndpointReconcilerProvider {
	return func(_ client.Client, _ logr.Logger, _ clientwrapper.SageMakerClientWrapper) EndpointReconciler {
		return endpointReconciler
	}
}

// Mock implementation of EndpointReconciler.
// This simply tracks invocations of Reconcile and the parameters it was called with.
// Return values are configurable.
type mockEndpointReconciler struct {
	EndpointReconciler
	subreconcilerCallTracker
}

// Mock implementation of Reconcile. This stores the parameters it was called with in the mock. It also will return a ReturnValue
// in each invocation.
func (r *mockEndpointReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, actualDeployment *sagemaker.DescribeEndpointOutput) error {
	return r.TrackAll(desiredDeployment, actualDeployment)
}

// Mock implementation of EndpointConfigReconciler.
// This simply tracks invocations of Reconcile and the parameters it was called with.
// Return values are configurable.
type mockEndpointConfigReconciler struct {
	EndpointConfigReconciler
	subreconcilerCallTracker
}

// Mock implementation of Reconcile. This stores the parameters it was called with in the mock. It also will return a ReturnValue
// in each invocation.
func (r *mockEndpointConfigReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) error {
	return r.TrackOnlyDesiredDeployment(desiredDeployment)
}

// Mock implementation of GetSageMakerEndpointConfigName
func (r *mockEndpointConfigReconciler) GetSageMakerEndpointConfigName(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (string, error) {
	return "", nil
}

// Mock implementation of ModelReconciler.
// This simply tracks invocations of Reconcile and the parameters it was called with.
// Return values are configurable.
type mockModelReconciler struct {
	ModelReconciler
	subreconcilerCallTracker
}

// Mock implementation of Reconcile. This stores the parameters it was called with in the mock. It also will return a ReturnValue
// in each invocation.
func (r *mockModelReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) error {
	return r.TrackOnlyDesiredDeployment(desiredDeployment)
}

// Mock implementation of GetSageMakerModelNames.
func (r *mockModelReconciler) GetSageMakerModelNames(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (map[string]string, error) {
	return map[string]string{}, nil
}

// Call tracker for sub reconcilers (ModelReconciler/EndpointConfigReconciler/EndpointReconciler).
// Common logic and variables are refactored into this common struct.
// This simply tracks invocations of Reconcile and the parameters it was called with.
// Return values are configurable.
type subreconcilerCallTracker struct {

	// A list of HostingDeployments that are passed to Reconcile. This is useful if a test wants
	// to verify that parameters were correctly passed.
	// This must be non-nil in order for HostingDeployments to be stored here.
	DesiredDeployments *List

	// A list of *DescribeEndpointOutputs that are passed to Reconcile. This is useful if a test wants
	// to verify that parameters were correctly passed.
	// This must be non-nil in order for *DescribeEndpointOutputs to be stored here.
	ActualDeployments *List

	// A list of errors that are returned from the mock Reconcile.
	// If this is nil, or if the number of calls to Reconcile is greater than the number of elements
	// originally in this list, Reconcile will return nil.
	ReconcileReturnValues *List
}

// Store the DesiredDeployment and return a ReturnValue
func (r *subreconcilerCallTracker) TrackOnlyDesiredDeployment(desiredDeployment *hostingv1.HostingDeployment) error {

	if r.DesiredDeployments != nil {
		r.DesiredDeployments.PushBack(desiredDeployment)
	}

	if r.ReconcileReturnValues != nil && r.ReconcileReturnValues.Len() > 0 {
		front := r.ReconcileReturnValues.Front()
		r.ReconcileReturnValues.Remove(front)
		return front.Value.(error)
	} else {
		return nil
	}
}

// Store the DesiredDeployment and ActualDeployment. Return a ReturnValue.
func (r *subreconcilerCallTracker) TrackAll(desiredDeployment *hostingv1.HostingDeployment, actualDeployment *sagemaker.DescribeEndpointOutput) error {

	if r.ActualDeployments != nil {
		r.ActualDeployments.PushBack(actualDeployment)
	}

	return r.TrackOnlyDesiredDeployment(desiredDeployment)
}

func createDeploymentWithGeneratedNames() *hostingv1.HostingDeployment {
	k8sName := "endpoint-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()
	return createDeployment(k8sName, k8sNamespace)
}

func createDeployment(k8sName, k8sNamespace string) *hostingv1.HostingDeployment {
	return &hostingv1.HostingDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: k8sNamespace,
		},
		Spec: hostingv1.HostingDeploymentSpec{
			Region: ToStringPtr("us-east-1"),
			ProductionVariants: []commonv1.ProductionVariant{
				{
					InitialInstanceCount: ToInt64Ptr(5),
					InstanceType:         "instance-type",
					ModelName:            ToStringPtr("model-name"),
					VariantName:          ToStringPtr("variant-name"),
				},
			},
			Models:     []commonv1.Model{},
			Containers: []commonv1.ContainerDefinition{},
		},
	}
}
