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
	"time"

	. "container/list"

	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/aws/aws-sdk-go/service/sagemaker/sagemakeriface"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	modelv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/model"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("Reconciling an model while failing to get the Kubernetes job", func() {

	var (
		sageMakerClient sagemakeriface.ClientAPI
	)

	BeforeEach(func() {
		sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).Build()
	})

	It("should not requeue if the model does not exist", func() {
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

var _ = Describe("Reconciling a model that does not exist in SageMaker", func() {

	var (
		receivedRequests           List
		mockSageMakerClientBuilder *MockSageMakerClientBuilder
		model                      *modelv1.Model
	)

	BeforeEach(func() {
		receivedRequests = List{}
		mockSageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		model = createModelWithGeneratedNames()
		err := k8sClient.Create(context.Background(), model)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should not return an error and not requeue immediately", func() {
		modelName := "test-model-name"
		modelArn := "model-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeModelErrorResponse("ValidationException", "Could not find model xyz", 400, "request id").
			AddCreateModelResponse(sagemaker.CreateModelOutput{
				ModelArn: &modelArn,
			}).
			AddDescribeModelResponse(sagemaker.DescribeModelOutput{
				ModelName: &modelName,
				ModelArn:  &modelArn,
			}).
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(model.ObjectMeta.Name, model.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).ToNot(Equal(time.Duration(0)))
	})

	It("should create the model", func() {
		modelName := "test-model-name"
		modelArn := "model-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeModelErrorResponse("ValidationException", "Could not find model xyz", 400, "request id").
			AddCreateModelResponse(sagemaker.CreateModelOutput{
				ModelArn: &modelArn,
			}).
			AddDescribeModelResponse(sagemaker.DescribeModelOutput{
				ModelName: &modelName,
				ModelArn:  &modelArn,
			}).
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(model.ObjectMeta.Name, model.ObjectMeta.Namespace)

		controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(3))
	})

	It("should add the finalizer and update the status", func() {
		modelName := "test-model-name"
		modelArn := "model-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeModelErrorResponse("ValidationException", "Could not find model xyz", 400, "request id").
			AddCreateModelResponse(sagemaker.CreateModelOutput{
				ModelArn: &modelArn,
			}).
			AddDescribeModelResponse(sagemaker.DescribeModelOutput{
				ModelName: &modelName,
				ModelArn:  &modelArn,
			}).
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(model.ObjectMeta.Name, model.ObjectMeta.Namespace)

		controller.Reconcile(request)

		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: model.ObjectMeta.Namespace,
			Name:      model.ObjectMeta.Name,
		}, model)
		Expect(err).ToNot(HaveOccurred())

		// Verify a finalizer has been added
		Expect(model.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))

		Expect(receivedRequests.Len()).To(Equal(3))
		Expect(model.Status.Status).To(Equal(CreatedStatus))
		Expect(model.Status.SageMakerModelName).To(Equal(modelName))
		Expect(model.Status.ModelArn).To(Equal(modelArn))
	})
})

var _ = Describe("Reconciling a model that is different than the spec", func() {

	var (
		receivedRequests     List
		model                *modelv1.Model
		outOfDateDescription sagemaker.DescribeModelOutput
		sageMakerClient      sagemakeriface.ClientAPI
		controller           ModelReconciler
		request              ctrl.Request
	)

	BeforeEach(func() {
		model = createModelWithFinalizer()
		err := k8sClient.Create(context.Background(), model)
		Expect(err).ToNot(HaveOccurred())

		outOfDateDescription = sagemaker.DescribeModelOutput{
			ModelName:        ToStringPtr("model name"),
			ModelArn:         ToStringPtr("model arn"),
			ExecutionRoleArn: model.Spec.ExecutionRoleArn,
			PrimaryContainer: &sagemaker.ContainerDefinition{
				ContainerHostname: model.Spec.PrimaryContainer.ContainerHostname,
				ModelDataUrl:      model.Spec.PrimaryContainer.ModelDataUrl,
				Image:             ToStringPtr("modified.image.amazon.com/container:tag"),
			},
		}
		request = CreateReconciliationRequest(model.ObjectMeta.Name, model.ObjectMeta.Namespace)

	})

	AfterEach(func() {
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      model.ObjectMeta.Name,
			Namespace: model.ObjectMeta.Namespace,
		}, model)
		Expect(err).ToNot(HaveOccurred())

		model.ObjectMeta.Finalizers = []string{}
		err = k8sClient.Update(context.Background(), model)
		Expect(err).ToNot(HaveOccurred())

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), model)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When the delete and creation succeeds", func() {

		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeModelResponse(outOfDateDescription).
				AddDeleteModelResponse(sagemaker.DeleteModelOutput{}).
				AddCreateModelResponse(sagemaker.CreateModelOutput{}).
				AddDescribeModelResponse(sagemaker.DescribeModelOutput{}).
				Build()

			controller = createReconciler(k8sClient, sageMakerClient, "1s")
		})
		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(4))
		})
		It("should delete and create the model in SageMaker", func() {

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
				Namespace: model.ObjectMeta.Namespace,
				Name:      model.ObjectMeta.Name,
			}, model)
			Expect(err).ToNot(HaveOccurred())
			Expect(model.ObjectMeta.Finalizers).To(ContainElement(SageMakerResourceFinalizerName))
		})
	})

	Context("When the delete fails", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeModelResponse(outOfDateDescription).
				AddDeleteModelErrorResponse("ValidationException", "server error", 500, "request id").
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

			var a modelv1.Model
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: model.ObjectMeta.Namespace,
				Name:      model.ObjectMeta.Name,
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
				AddDescribeModelResponse(outOfDateDescription).
				AddDeleteModelErrorResponse("ValidationException", "Could not find model", 400, "request id").
				AddCreateModelResponse(sagemaker.CreateModelOutput{}).
				AddDescribeModelResponse(sagemaker.DescribeModelOutput{}).
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
				Namespace: model.ObjectMeta.Namespace,
				Name:      model.ObjectMeta.Name,
			}, model)
			Expect(err).ToNot(HaveOccurred())
			Expect(model.ObjectMeta.Finalizers).To(ContainElement(SageMakerResourceFinalizerName))
		})

	})

	Context("When the create fails", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeModelResponse(outOfDateDescription).
				AddDeleteModelResponse(sagemaker.DeleteModelOutput{}).
				AddCreateModelErrorResponse("ValidationException", "server error", 500, "request id").
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
				AddDescribeModelResponse(outOfDateDescription).
				AddDeleteModelResponse(sagemaker.DeleteModelOutput{}).
				AddCreateModelResponse(sagemaker.CreateModelOutput{}).
				AddDescribeModelErrorResponse("ValidationException", "server error", 500, "request id").
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
				Namespace: model.ObjectMeta.Namespace,
				Name:      model.ObjectMeta.Name,
			}, model)
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Status.Status).To(Equal(ErrorStatus))
			Expect(model.Status.Additional).To(ContainSubstring("server error"))
		})
	})

	Context("When the second describe returns 404", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockSageMakerClientBuilder := NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			sageMakerClient = mockSageMakerClientBuilder.
				AddDescribeModelResponse(outOfDateDescription).
				AddDeleteModelResponse(sagemaker.DeleteModelOutput{}).
				AddCreateModelResponse(sagemaker.CreateModelOutput{}).
				AddDescribeModelErrorResponse("ValidationException", "Could not find model", 400, "request id").
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
				Namespace: model.ObjectMeta.Namespace,
				Name:      model.ObjectMeta.Name,
			}, model)
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Status.Status).To(Equal(ErrorStatus))
			Expect(model.Status.Additional).To(ContainSubstring("model does not exist after creation"))
		})
	})
})

var _ = Describe("Reconciling a model with finalizer that is being deleted", func() {

	var (
		receivedRequests           List
		mockSageMakerClientBuilder *MockSageMakerClientBuilder
		model                      *modelv1.Model
	)

	BeforeEach(func() {
		receivedRequests = List{}
		mockSageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		model = createModelWithFinalizer()
		err := k8sClient.Create(context.Background(), model)
		Expect(err).ToNot(HaveOccurred())

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), model)
		Expect(err).ToNot(HaveOccurred())

	})

	It("should do nothing if the model is not in sagemaker", func() {
		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeModelErrorResponse("ValidationException", "Could not find model xyz", 400, "request id").
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(model.ObjectMeta.Name, model.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		// Should requeue
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(controller.PollInterval)))
		Expect(receivedRequests.Len()).To(Equal(1))
	})

	It("should delete the model in sagemaker", func() {
		modelName := "test-model-name"
		modelArn := "model-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeModelResponse(sagemaker.DescribeModelOutput{
				ModelName: &modelName,
				ModelArn:  &modelArn,
			}).
			AddDeleteModelResponse(sagemaker.DeleteModelOutput{}).
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(model.ObjectMeta.Name, model.ObjectMeta.Namespace)

		Expect(model.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))
		result, err := controller.Reconcile(request)

		// Should not requeue
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).ToNot(Equal(time.Duration(0)))
		Expect(receivedRequests.Len()).To(Equal(2))

		// entry should be deleted from k8s
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: model.ObjectMeta.Namespace,
			Name:      model.ObjectMeta.Name,
		}, model)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("Verify that finalizer is removed (or object is deleted) when model does not exist", func() {
		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeModelErrorResponse("ValidationException", "Could not find model xyz", 400, "request id").
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(model.ObjectMeta.Name, model.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		// Should requeue
		// TODO: Ideally in this case we should do no requeue but we are doing extra requeu
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(controller.PollInterval)))

		// We can't verify about finalizer but can make sure that object has been deleted from k8s
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: model.ObjectMeta.Namespace,
			Name:      model.ObjectMeta.Name,
		}, model)

		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
		Expect(receivedRequests.Len()).To(Equal(1))
	})

	It("Verify that finalizer is not removed and error returned when SageMaker fails to delete", func() {
		modelName := "test-model-name"
		modelArn := "model-arn"

		sageMakerClient := mockSageMakerClientBuilder.
			AddDescribeModelResponse(sagemaker.DescribeModelOutput{
				ModelName: &modelName,
				ModelArn:  &modelArn,
			}).
			AddDeleteModelErrorResponse("ValidationException", "server error", 500, "request id").
			Build()

		controller := createReconciler(k8sClient, sageMakerClient, "1s")
		request := CreateReconciliationRequest(model.ObjectMeta.Name, model.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		// Should requeue
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(model.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))
		Expect(receivedRequests.Len()).To(Equal(2))
	})
})

func createReconciler(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalStr string) ModelReconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return ModelReconciler{
		Client:                k8sClient,
		Log:                   ctrl.Log,
		PollInterval:          pollInterval,
		createSageMakerClient: CreateMockSageMakerClientProvider(sageMakerClient),
		awsConfigLoader:       CreateMockAwsConfigLoader(),
	}
}

func createModelWithGeneratedNames() *modelv1.Model {
	k8sName := "model-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()
	CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)
	return createModel(false, k8sName, k8sNamespace)
}

func createModelWithFinalizer() *modelv1.Model {
	k8sName := "model-" + uuid.New().String()
	k8sNamespace := "namespace-" + uuid.New().String()
	CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)
	return createModel(true, k8sName, k8sNamespace)
}

func createModel(withFinalizer bool, k8sName, k8sNamespace string) *modelv1.Model {
	finalizers := []string{}
	if withFinalizer {
		finalizers = append(finalizers, SageMakerResourceFinalizerName)
	}
	return &modelv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:       k8sName,
			Namespace:  k8sNamespace,
			Finalizers: finalizers,
		},
		Spec: modelv1.ModelSpec{
			Region:           ToStringPtr("us-east-1"),
			ExecutionRoleArn: ToStringPtr("xxx"),
			PrimaryContainer: &commonv1.ContainerDefinition{
				ContainerHostname: ToStringPtr("container-hostname"),
				ModelDataUrl:      ToStringPtr("s3://bucket-name/model.tar.gz"),
				Image:             ToStringPtr("123.amazon.com/xgboost:latest"),
			},
		},
	}
}
