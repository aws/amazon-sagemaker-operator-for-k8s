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
	"time"

	. "container/list"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	endpointconfigv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/endpointconfig"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("Reconciling an EndpointConfig while failing to get the Kubernetes job", func() {

	var (
		sageMakerClient sagemakeriface.ClientAPI
	)

	BeforeEach(func() {
		sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).Build()
	})

	It("should not requeue if the EndpointConfig does not exist", func() {
		controller := createReconciler(k8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should requeue if there was an error", func() {
		mockK8sClient := FailToGetK8sClient{}
		controller := createReconciler(mockK8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
	})
})

var _ = Describe("Reconciling a EndpointConfig that does not exist in SageMaker", func() {

	var (
		receivedRequests           List
		mockSageMakerClientBuilder *MockSageMakerClientBuilder
		endpointConfig             *endpointconfigv1.EndpointConfig
	)

	BeforeEach(func() {
		receivedRequests = List{}
		mockSageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		endpointConfig = createEndpointConfigWithGeneratedNames()
		err := k8sClient.Create(context.Background(), endpointConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should not return an error and requeue after interval", func() {
		endpointConfigName := "test-endpoint-config-name"
		endpointConfigArn := "endpoint-config-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeEndpointConfigErrorResponse(clientwrapper.DescribeEndpointConfig404Code, clientwrapper.DescribeEndpointConfig404MessagePrefix+" xyz", 400, "request id").
			AddCreateEndpointConfigResponse(sagemaker.CreateEndpointConfigOutput{
				EndpointConfigArn: &endpointConfigArn,
			}).
			AddDescribeEndpointConfigResponse(sagemaker.DescribeEndpointConfigOutput{
				EndpointConfigName: &endpointConfigName,
				EndpointConfigArn:  &endpointConfigArn,
			}).
			Build()

		pollIntervalStr := "1s"
		controller := createReconciler(k8sClient, sageMakerClient, pollIntervalStr)
		request := CreateReconciliationRequest(endpointConfig.ObjectMeta.Name, endpointConfig.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(ParseDurationOrFail(pollIntervalStr)))
	})

	It("should create the endpointconfig", func() {
		endpointConfigName := "test-endpointconfig-name"
		endpointConfigArn := "endpointconfig-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeEndpointConfigErrorResponse(clientwrapper.DescribeEndpointConfig404Code, clientwrapper.DescribeEndpointConfig404MessagePrefix+" xyz", 400, "request id").
			AddCreateEndpointConfigResponse(sagemaker.CreateEndpointConfigOutput{
				EndpointConfigArn: &endpointConfigArn,
			}).
			AddDescribeEndpointConfigResponse(sagemaker.DescribeEndpointConfigOutput{
				EndpointConfigName: &endpointConfigName,
				EndpointConfigArn:  &endpointConfigArn,
			}).
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(endpointConfig.ObjectMeta.Name, endpointConfig.ObjectMeta.Namespace)

		controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(3))
	})

	It("should add the finalizer and update the status", func() {
		endpointConfigName := "test-endpointconfig-name"
		endpointConfigArn := "endpointconfig-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeEndpointConfigErrorResponse(clientwrapper.DescribeEndpointConfig404Code, clientwrapper.DescribeEndpointConfig404MessagePrefix+" xyz", 400, "request id").
			AddCreateEndpointConfigResponse(sagemaker.CreateEndpointConfigOutput{
				EndpointConfigArn: &endpointConfigArn,
			}).
			AddDescribeEndpointConfigResponse(sagemaker.DescribeEndpointConfigOutput{
				EndpointConfigName: &endpointConfigName,
				EndpointConfigArn:  &endpointConfigArn,
			}).
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(endpointConfig.ObjectMeta.Name, endpointConfig.ObjectMeta.Namespace)

		controller.Reconcile(request)

		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: endpointConfig.ObjectMeta.Namespace,
			Name:      endpointConfig.ObjectMeta.Name,
		}, endpointConfig)
		Expect(err).ToNot(HaveOccurred())

		// Verify a finalizer has been added
		Expect(endpointConfig.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))

		Expect(receivedRequests.Len()).To(Equal(3))
		Expect(endpointConfig.Status.Status).To(Equal(CreatedStatus))
		Expect(endpointConfig.Status.SageMakerEndpointConfigName).To(Equal(endpointConfigName))
		Expect(endpointConfig.Status.EndpointConfigArn).To(Equal(endpointConfigArn))
	})
})

var _ = Describe("Reconciling a endpointconfig with finalizer that is being deleted", func() {

	var (
		receivedRequests           List
		mockSageMakerClientBuilder *MockSageMakerClientBuilder
		endpointconfig             *endpointconfigv1.EndpointConfig
	)

	BeforeEach(func() {
		receivedRequests = List{}
		mockSageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		endpointconfig = createEndpointConfigWithFinalizer()
		err := k8sClient.Create(context.Background(), endpointconfig)
		Expect(err).ToNot(HaveOccurred())

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), endpointconfig)
		Expect(err).ToNot(HaveOccurred())

	})

	It("should do nothing if the endpointconfig is not in sagemaker", func() {
		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeEndpointConfigErrorResponse(clientwrapper.DescribeEndpointConfig404Code, clientwrapper.DescribeEndpointConfig404MessagePrefix+" xyz", 400, "request id").
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(endpointconfig.ObjectMeta.Name, endpointconfig.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		// Should requeue
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(ParseDurationOrFail("1s")))
		Expect(receivedRequests.Len()).To(Equal(1))
	})

	It("should delete the endpointconfig in sagemaker", func() {
		endpointconfigName := "test-endpointconfig-name"
		endpointconfigArn := "endpointconfig-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeEndpointConfigResponse(sagemaker.DescribeEndpointConfigOutput{
				EndpointConfigName: &endpointconfigName,
				EndpointConfigArn:  &endpointconfigArn,
			}).
			AddDeleteEndpointConfigResponse(sagemaker.DeleteEndpointConfigOutput{}).
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(endpointconfig.ObjectMeta.Name, endpointconfig.ObjectMeta.Namespace)

		Expect(endpointconfig.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))
		result, err := controller.Reconcile(request)

		// Should not requeue
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).ToNot(Equal(time.Duration(0)))
		Expect(receivedRequests.Len()).To(Equal(2))

		// entry should be deleted from k8s
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: endpointconfig.ObjectMeta.Namespace,
			Name:      endpointconfig.ObjectMeta.Name,
		}, endpointconfig)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("Verify that finalizer is removed (or object is deleted) when endpointconfig does not exist", func() {
		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeEndpointConfigErrorResponse(clientwrapper.DescribeEndpointConfig404Code, clientwrapper.DescribeEndpointConfig404MessagePrefix+" xyz", 400, "request id").
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(endpointconfig.ObjectMeta.Name, endpointconfig.ObjectMeta.Namespace)

		_, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())

		// We can't verify about finalizer but can make sure that object has been deleted from k8s
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: endpointconfig.ObjectMeta.Namespace,
			Name:      endpointconfig.ObjectMeta.Name,
		}, endpointconfig)

		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
		Expect(receivedRequests.Len()).To(Equal(1))
	})

	It("Verify that finalizer is not removed  when SageMaker fails to delete", func() {
		endpointconfigName := "test-endpointconfig-name"
		endpointconfigArn := "endpointconfig-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeEndpointConfigResponse(sagemaker.DescribeEndpointConfigOutput{
				EndpointConfigName: &endpointconfigName,
				EndpointConfigArn:  &endpointconfigArn,
			}).
			AddDeleteEndpointConfigErrorResponse("ValidationException", "Server error", 500, "request id").
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(endpointconfig.ObjectMeta.Name, endpointconfig.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		// Should requeue
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(endpointconfig.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))
		Expect(receivedRequests.Len()).To(Equal(2))
	})
})

func createReconciler(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalStr string) EndpointConfigReconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return EndpointConfigReconciler{
		Client:                k8sClient,
		Log:                   ctrl.Log,
		PollInterval:          pollInterval,
		createSageMakerClient: CreateMockSageMakerClientProvider(sageMakerClient),
		awsConfigLoader:       CreateMockAwsConfigLoader(),
	}
}

func createEndpointConfigWithGeneratedNames() *endpointconfigv1.EndpointConfig {
	k8sName := "endpointconfig-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()
	return createEndpointConfig(false, k8sName, k8sNamespace)
}

func createEndpointConfigWithFinalizer() *endpointconfigv1.EndpointConfig {
	k8sName := "endpointconfig-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()
	return createEndpointConfig(true, k8sName, k8sNamespace)
}

func createEndpointConfig(withFinalizer bool, k8sName, k8sNamespace string) *endpointconfigv1.EndpointConfig {
	finalizers := []string{}
	if withFinalizer {
		finalizers = append(finalizers, SageMakerResourceFinalizerName)
	}
	return &endpointconfigv1.EndpointConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       k8sName,
			Namespace:  k8sNamespace,
			Finalizers: finalizers,
		},
		Spec: endpointconfigv1.EndpointConfigSpec{
			Region: ToStringPtr("us-east-1"),
			ProductionVariants: []commonv1.ProductionVariant{
				{
					VariantName:          ToStringPtr("variant-name"),
					InitialInstanceCount: ToInt64Ptr(5),
					InitialVariantWeight: ToInt64Ptr(1),
					InstanceType:         "instance-type",
					ModelName:            ToStringPtr("model-name"),
				},
			},
		},
	}
}

var _ = Describe("Reconciling a endpointConfig that is different than the spec", func() {

	var (
		receivedRequests     List
		endpointConfig       *endpointconfigv1.EndpointConfig
		outOfDateDescription sagemaker.DescribeEndpointConfigOutput
		sageMakerClient      sagemakeriface.ClientAPI
		controller           EndpointConfigReconciler
		request              ctrl.Request
	)

	BeforeEach(func() {
		endpointConfig = createEndpointConfigWithFinalizer()
		err := k8sClient.Create(context.Background(), endpointConfig)
		Expect(err).ToNot(HaveOccurred())

		outOfDateDescription = sagemaker.DescribeEndpointConfigOutput{
			EndpointConfigName: ToStringPtr("endpointConfig name"),
			EndpointConfigArn:  ToStringPtr("endpointConfig arn"),
			KmsKeyId:           ToStringPtr(endpointConfig.Spec.KmsKeyId),
			ProductionVariants: []sagemaker.ProductionVariant{},
		}

		for _, pv := range endpointConfig.Spec.ProductionVariants {
			outOfDateDescription.ProductionVariants = append(outOfDateDescription.ProductionVariants, sagemaker.ProductionVariant{
				AcceleratorType:      sagemaker.ProductionVariantAcceleratorType(pv.AcceleratorType),
				InitialInstanceCount: ToInt64Ptr(*pv.InitialInstanceCount + 1),
				InitialVariantWeight: ToFloat64Ptr(float64(*pv.InitialVariantWeight)),
				ModelName:            ToStringPtr(*pv.ModelName),
				VariantName:          ToStringPtr(*pv.VariantName),
			})
		}

		request = CreateReconciliationRequest(endpointConfig.ObjectMeta.Name, endpointConfig.ObjectMeta.Namespace)

	})

	AfterEach(func() {
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      endpointConfig.ObjectMeta.Name,
			Namespace: endpointConfig.ObjectMeta.Namespace,
		}, endpointConfig)
		Expect(err).ToNot(HaveOccurred())

		endpointConfig.ObjectMeta.Finalizers = []string{}
		err = k8sClient.Update(context.Background(), endpointConfig)
		Expect(err).ToNot(HaveOccurred())

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), endpointConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When the delete and creation succeeds", func() {

		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeEndpointConfigResponse(outOfDateDescription).
				AddDeleteEndpointConfigResponse(sagemaker.DeleteEndpointConfigOutput{}).
				AddCreateEndpointConfigResponse(sagemaker.CreateEndpointConfigOutput{}).
				AddDescribeEndpointConfigResponse(sagemaker.DescribeEndpointConfigOutput{}).
				Build()

			controller = createReconciler(k8sClient, sageMakerClient, "1s")
		})
		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(4))
		})
		It("should delete and create the endpointConfig in SageMaker", func() {

			result, err := controller.Reconcile(request)

			// Should requeue after interval.
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(false))
			Expect(result.RequeueAfter).ToNot(Equal(time.Duration(0)))

		})

		It("should not delete or remove the finalizer from the Kubernetes object", func() {
			controller.Reconcile(request)

			// entry should not deleted from k8s, and still have a finalizer.
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: endpointConfig.ObjectMeta.Namespace,
				Name:      endpointConfig.ObjectMeta.Name,
			}, endpointConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpointConfig.ObjectMeta.Finalizers).To(ContainElement(SageMakerResourceFinalizerName))
		})
	})

	Context("When the delete fails", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeEndpointConfigResponse(outOfDateDescription).
				AddDeleteEndpointConfigErrorResponse("ValidationException", "server error", 500, "request id").
				Build()

			controller = createReconciler(k8sClient, sageMakerClient, "1s")

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(2))
		})

		It("should requeue immediately", func() {
			result, err := controller.Reconcile(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(true))
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})

		It("should update the status with the error message", func() {
			controller.Reconcile(request)

			var a endpointconfigv1.EndpointConfig
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: endpointConfig.ObjectMeta.Namespace,
				Name:      endpointConfig.ObjectMeta.Name,
			}, &a)
			Expect(err).ToNot(HaveOccurred())

			Expect(a.Status.Status).To(Equal(ErrorStatus))
			Expect(a.Status.Additional).To(ContainSubstring("server error"))
		})
	})

	Context("When the delete returns 404", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeEndpointConfigResponse(outOfDateDescription).
				AddDeleteEndpointConfigErrorResponse("ValidationException", clientwrapper.DeleteEndpointConfig404MessagePrefix, 400, "request id").
				AddCreateEndpointConfigResponse(sagemaker.CreateEndpointConfigOutput{}).
				AddDescribeEndpointConfigResponse(sagemaker.DescribeEndpointConfigOutput{}).
				Build()

			controller = createReconciler(k8sClient, sageMakerClient, "1s")

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(4))
		})

		// This implicitly checks that the Create happened as expected.
		It("should requeue after interval and not return error", func() {

			result, err := controller.Reconcile(request)

			// Should requeue after interval.
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(false))
			Expect(result.RequeueAfter).ToNot(Equal(time.Duration(0)))

		})

		It("should not delete or remove the finalizer from the Kubernetes object", func() {
			controller.Reconcile(request)

			// entry should not deleted from k8s, and still have a finalizer.
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: endpointConfig.ObjectMeta.Namespace,
				Name:      endpointConfig.ObjectMeta.Name,
			}, endpointConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpointConfig.ObjectMeta.Finalizers).To(ContainElement(SageMakerResourceFinalizerName))
		})

	})

	Context("When the create fails", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeEndpointConfigResponse(outOfDateDescription).
				AddDeleteEndpointConfigResponse(sagemaker.DeleteEndpointConfigOutput{}).
				AddCreateEndpointConfigErrorResponse("ValidationException", "server error", 500, "request id").
				Build()

			controller = createReconciler(k8sClient, sageMakerClient, "1s")

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(3))
		})

		It("should requeue immediately", func() {
			result, err := controller.Reconcile(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(true))
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})
	})

	Context("When the second describe fails", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeEndpointConfigResponse(outOfDateDescription).
				AddDeleteEndpointConfigResponse(sagemaker.DeleteEndpointConfigOutput{}).
				AddCreateEndpointConfigResponse(sagemaker.CreateEndpointConfigOutput{}).
				AddDescribeEndpointConfigErrorResponse("ValidationException", "server error", 500, "request id").
				Build()

			controller = createReconciler(k8sClient, sageMakerClient, "1s")

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(4))
		})

		It("should requeue immediately", func() {
			result, err := controller.Reconcile(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(true))
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})

		It("should update the status with the error message", func() {
			controller.Reconcile(request)

			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: endpointConfig.ObjectMeta.Namespace,
				Name:      endpointConfig.ObjectMeta.Name,
			}, endpointConfig)
			Expect(err).ToNot(HaveOccurred())

			Expect(endpointConfig.Status.Status).To(Equal(ErrorStatus))
			Expect(endpointConfig.Status.Additional).To(ContainSubstring("server error"))
		})
	})

	Context("When the second describe returns 404", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeEndpointConfigResponse(outOfDateDescription).
				AddDeleteEndpointConfigResponse(sagemaker.DeleteEndpointConfigOutput{}).
				AddCreateEndpointConfigResponse(sagemaker.CreateEndpointConfigOutput{}).
				AddDescribeEndpointConfigErrorResponse("ValidationException", clientwrapper.DeleteEndpointConfig404MessagePrefix, 400, "request id").
				Build()

			controller = createReconciler(k8sClient, sageMakerClient, "1s")

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(4))
		})

		It("should requeue immediately", func() {
			result, err := controller.Reconcile(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(true))
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})

		It("should update the status with the error message", func() {
			controller.Reconcile(request)

			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: endpointConfig.ObjectMeta.Namespace,
				Name:      endpointConfig.ObjectMeta.Name,
			}, endpointConfig)
			Expect(err).ToNot(HaveOccurred())

			Expect(endpointConfig.Status.Status).To(Equal(ErrorStatus))
			Expect(endpointConfig.Status.Additional).To(ContainSubstring("endpointConfig does not exist after creation"))
		})
	})
})
