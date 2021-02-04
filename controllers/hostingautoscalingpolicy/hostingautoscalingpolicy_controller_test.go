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

package hostingautoscalingpolicy

import (
	. "container/list"
	"context"
	"time"

	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	hostingautoscalingpolicyv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingautoscalingpolicy"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling/applicationautoscalingiface"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("Reconciling HAP while failing to get the Kubernetes object", func() {

	var (
		applicationAutoscalingClient applicationautoscalingiface.ApplicationAutoScalingAPI
	)

	BeforeEach(func() {
		applicationAutoscalingClient = NewMockAutoscalingClientBuilder(GinkgoT()).Build()
	})

	It("should not requeue if the hap does not exist", func() {
		controller := createReconciler(k8sClient, applicationAutoscalingClient)

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should requeue if there was an error", func() {
		mockK8sClient := FailToGetK8sClient{}
		controller := createReconciler(mockK8sClient, applicationAutoscalingClient)

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
	})
})

var _ = Describe("Reconciling HAP that does not exist", func() {

	var (
		receivedRequests             List
		mockAutoscalingClientBuilder *MockAutoscalingClientBuilder
		hostingautoscalingpolicy     *hostingautoscalingpolicyv1.HostingAutoscalingPolicy
	)

	BeforeEach(func() {
		receivedRequests = List{}
		mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		hostingautoscalingpolicy = createHAPWithGeneratedNames()
		err := k8sClient.Create(context.Background(), hostingautoscalingpolicy)
		hostingautoscalingpolicy.Status.ResourceIDList = []string{"endpoint/endpoint-xyz/variant/variant-xyz"}
		err = k8sClient.Status().Update(context.Background(), hostingautoscalingpolicy)

		Expect(err).ToNot(HaveOccurred())
	})

	It("should not return an error and not requeue immediately", func() {
		policyName := "test-policy-name"
		policyArn := "policy-arn"
		resourceId := "endpoint/endpoint-xyz/variant/variant-xyz"
		scalableTarget := applicationautoscaling.ScalableTarget{
			ResourceId: &resourceId,
		}
		scalableTargets := []applicationautoscaling.ScalableTarget{scalableTarget}
		scalingPolicy := applicationautoscaling.ScalingPolicy{
			PolicyName: &policyName,
			PolicyARN:  &policyArn,
			ResourceId: &resourceId,
		}
		scalingPolicies := []applicationautoscaling.ScalingPolicy{scalingPolicy}

		applicationAutoscalingClient := mockAutoscalingClientBuilder.
			AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{}).
			AddDescribeScalingPoliciesResponse(applicationautoscaling.DescribeScalingPoliciesOutput{
				ScalingPolicies: []applicationautoscaling.ScalingPolicy{},
			}).
			AddRegisterScalableTargetsResponse(applicationautoscaling.RegisterScalableTargetOutput{}).
			AddPutScalingPolicyResponse(applicationautoscaling.PutScalingPolicyOutput{
				PolicyARN: &policyArn,
			}).
			AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{
				ScalableTargets: scalableTargets,
			}).
			AddDescribeScalingPoliciesResponse(applicationautoscaling.DescribeScalingPoliciesOutput{
				ScalingPolicies: scalingPolicies,
			}).
			Build()

		controller := createReconciler(k8sClient, applicationAutoscalingClient)
		request := CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
	})

	It("should put the scalingPolicy", func() {

		policyName := "test-policy-name"
		policyArn := "policy-arn"
		resourceId := "endpoint/endpoint-xyz/variant/variant-xyz"
		scalableTarget := &applicationautoscaling.ScalableTarget{
			ResourceId: &resourceId,
		}
		scalableTargets := []*applicationautoscaling.ScalableTarget{scalableTarget}
		scalingPolicy := applicationautoscaling.ScalingPolicy{
			PolicyName: &policyName,
			PolicyARN:  &policyArn,
			ResourceId: &resourceId,
		}
		scalingPolicies := []applicationautoscaling.ScalingPolicy{scalingPolicy}

		applicationAutoscalingClient := mockAutoscalingClientBuilder.
			AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{}).
			AddDescribeScalingPoliciesResponse(applicationautoscaling.DescribeScalingPoliciesOutput{
				ScalingPolicies: []*applicationautoscaling.ScalingPolicy{},
			}).
			AddRegisterScalableTargetsResponse(applicationautoscaling.RegisterScalableTargetOutput{}).
			AddPutScalingPolicyResponse(applicationautoscaling.PutScalingPolicyOutput{
				PolicyARN: &policyArn,
			}).
			AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{
				ScalableTargets: scalableTargets,
			}).
			AddDescribeScalingPoliciesResponse(applicationautoscaling.DescribeScalingPoliciesOutput{
				ScalingPolicies: scalingPolicies,
			}).
			Build()

		controller := createReconciler(k8sClient, applicationAutoscalingClient)
		request := CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		controller.Reconcile(request)

		Expect(receivedRequests.Len()).To(Equal(6))
	})

	It("should add the finalizer and update the status", func() {
		policyName := "test-policy-name"
		policyArn := "policy-arn"
		resourceId := "endpoint/endpoint-xyz/variant/variant-xyz"
		scalableTarget := applicationautoscaling.ScalableTarget{
			ResourceId: &resourceId,
		}
		scalableTargets := []applicationautoscaling.ScalableTarget{scalableTarget}
		scalingPolicy := &applicationautoscaling.ScalingPolicy{
			PolicyName: &policyName,
			PolicyARN:  &policyArn,
			ResourceId: &resourceId,
		}
		scalingPolicies := []*applicationautoscaling.ScalingPolicy{scalingPolicy}

		applicationAutoscalingClient := mockAutoscalingClientBuilder.
			AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{}).
			AddDescribeScalingPoliciesResponse(applicationautoscaling.DescribeScalingPoliciesOutput{
				ScalingPolicies: []*applicationautoscaling.ScalingPolicy{},
			}).
			AddRegisterScalableTargetsResponse(applicationautoscaling.RegisterScalableTargetOutput{}).
			AddPutScalingPolicyResponse(applicationautoscaling.PutScalingPolicyOutput{
				PolicyARN: &policyArn,
			}).
			AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{
				ScalableTargets: scalableTargets,
			}).
			AddDescribeScalingPoliciesResponse(applicationautoscaling.DescribeScalingPoliciesOutput{
				ScalingPolicies: scalingPolicies,
			}).
			Build()

		controller := createReconciler(k8sClient, applicationAutoscalingClient)
		request := CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		controller.Reconcile(request)

		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
			Name:      hostingautoscalingpolicy.ObjectMeta.Name,
		}, hostingautoscalingpolicy)
		Expect(err).ToNot(HaveOccurred())

		// Verify a finalizer has been added
		Expect(hostingautoscalingpolicy.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))

		Expect(receivedRequests.Len()).To(Equal(6))
		Expect(hostingautoscalingpolicy.Status.HostingAutoscalingPolicyStatus).To(Equal(CreatedAutoscalingJobStatus))
		Expect(hostingautoscalingpolicy.Status.PolicyName).To(Equal(policyName))
	})

})

var _ = Describe("Reconciling an HAP that is different from the spec", func() {

	var (
		receivedRequests                                     List
		mockAutoscalingClientBuilder                         *MockAutoscalingClientBuilder
		hostingautoscalingpolicy                             *hostingautoscalingpolicyv1.HostingAutoscalingPolicy
		outOfDateTargetDescription                           applicationautoscaling.DescribeScalableTargetsOutput
		outOfDatePolicyDescription, updatedPolicyDescription applicationautoscaling.DescribeScalingPoliciesOutput
		controller                                           Reconciler
		request                                              ctrl.Request
		policyName, policyArn                                string
	)

	BeforeEach(func() {
		hostingautoscalingpolicy = createHAPWithFinalizer()
		err := k8sClient.Create(context.Background(), hostingautoscalingpolicy)
		hostingautoscalingpolicy.Status.ResourceIDList = []string{"endpoint/endpoint-xyz/variant/variant-xyz"}
		err = k8sClient.Status().Update(context.Background(), hostingautoscalingpolicy)

		Expect(err).ToNot(HaveOccurred())

		policyName = "test-policy-name"
		policyArn = "policy-arn"
		resourceID := "endpoint/endpoint-xyz/variant/variant-xyz"
		scalableTarget := applicationautoscaling.ScalableTarget{
			ResourceId: &resourceID,
		}
		scalableTargets := []applicationautoscaling.ScalableTarget{scalableTarget}
		scalingPolicy := &applicationautoscaling.ScalingPolicy{
			PolicyName: &policyName,
			PolicyARN:  &policyArn,
			ResourceId: &resourceID,
		}
		scalingPolicies := []*applicationautoscaling.ScalingPolicy{scalingPolicy}

		outOfDateTargetDescription = applicationautoscaling.DescribeScalableTargetsOutput{
			ScalableTargets: scalableTargets,
		}
		outOfDatePolicyDescription = applicationautoscaling.DescribeScalingPoliciesOutput{
			ScalingPolicies: scalingPolicies,
		}
		updatedPolicyDescription = applicationautoscaling.DescribeScalingPoliciesOutput{
			ScalingPolicies: scalingPolicies,
		}
	})

	AfterEach(func() {
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      hostingautoscalingpolicy.ObjectMeta.Name,
			Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
		}, hostingautoscalingpolicy)
		Expect(err).ToNot(HaveOccurred())

		hostingautoscalingpolicy.ObjectMeta.Finalizers = []string{}
		err = k8sClient.Update(context.Background(), hostingautoscalingpolicy)
		Expect(err).ToNot(HaveOccurred())

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), hostingautoscalingpolicy)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When the delete and creation succeeds", func() {

		BeforeEach(func() {
			receivedRequests = List{}
			mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			applicationAutoscalingClient := mockAutoscalingClientBuilder.
				AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
				AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
				AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
				AddDeregisterScalableTargetsResponse(applicationautoscaling.DeregisterScalableTargetOutput{}).
				AddRegisterScalableTargetsResponse(applicationautoscaling.RegisterScalableTargetOutput{}).
				AddPutScalingPolicyResponse(applicationautoscaling.PutScalingPolicyOutput{}).
				AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{}).
				AddDescribeScalingPoliciesResponse(updatedPolicyDescription).
				Build()
			controller = createReconciler(k8sClient, applicationAutoscalingClient)
			request = CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		})
		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(8))
		})
		It("should delete and create the HAP", func() {

			result, err := controller.Reconcile(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(false))
		})

		It("should not delete or remove the finalizer from the Kubernetes object", func() {
			controller.Reconcile(request)

			// entry should not deleted from k8s, and still have a finalizer.
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
				Name:      hostingautoscalingpolicy.ObjectMeta.Name,
			}, hostingautoscalingpolicy)
			Expect(err).ToNot(HaveOccurred())
			Expect(hostingautoscalingpolicy.ObjectMeta.Finalizers).To(ContainElement(SageMakerResourceFinalizerName))
		})
	})

	Context("When the delete fails with a server error", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			applicationAutoscalingClient := mockAutoscalingClientBuilder.
				AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
				AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
				AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
				AddDeregisterScalableTargetsErrorResponse("InternalServiceException", "Server Error", 500, "request-id").
				Build()
			controller = createReconciler(k8sClient, applicationAutoscalingClient)
			request = CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

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
				Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
				Name:      hostingautoscalingpolicy.ObjectMeta.Name,
			}, hostingautoscalingpolicy)
			Expect(err).ToNot(HaveOccurred())
			Expect(hostingautoscalingpolicy.Status.HostingAutoscalingPolicyStatus).To(Equal(ReconcilingAutoscalingJobStatus))
			Expect(hostingautoscalingpolicy.Status.Additional).To(ContainSubstring("Unable to DeregisterScalableTarget"))
		})
	})

	Context("When the delete returns 404", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			applicationAutoscalingClient := mockAutoscalingClientBuilder.
				AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
				AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
				AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
				AddDeregisterScalableTargetsErrorResponse("ObjectNotFoundException", "Could not find HAP", 404, "request-id").
				AddRegisterScalableTargetsResponse(applicationautoscaling.RegisterScalableTargetOutput{}).
				AddPutScalingPolicyResponse(applicationautoscaling.PutScalingPolicyOutput{}).
				AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{}).
				AddDescribeScalingPoliciesResponse(updatedPolicyDescription).
				Build()
			controller = createReconciler(k8sClient, applicationAutoscalingClient)
			request = CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)
		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(8))
		})

		It("should not requeue and return error", func() {

			result, err := controller.Reconcile(request)

			Expect(err).ToNot(HaveOccurred())
			// If object is not found on delete, it is not an error. Only errors are requeued
			Expect(result.Requeue).To(Equal(false))
		})

		It("should not delete or remove the finalizer from the Kubernetes object", func() {
			controller.Reconcile(request)

			// entry should not deleted from k8s, and still have a finalizer.
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
				Name:      hostingautoscalingpolicy.ObjectMeta.Name,
			}, hostingautoscalingpolicy)
			Expect(err).ToNot(HaveOccurred())
			Expect(hostingautoscalingpolicy.ObjectMeta.Finalizers).To(ContainElement(SageMakerResourceFinalizerName))
			Expect(hostingautoscalingpolicy.Status.HostingAutoscalingPolicyStatus).To(Equal(CreatedAutoscalingJobStatus))
		})

	})

	Context("When the registerScalableTarget fails with server error", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			applicationAutoscalingClient := mockAutoscalingClientBuilder.
				AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
				AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
				AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
				AddDeregisterScalableTargetsResponse(applicationautoscaling.DeregisterScalableTargetOutput{}).
				AddRegisterScalableTargetsErrorResponse("InternalServiceException", "Server Error", 500, "request-id").
				Build()
			controller = createReconciler(k8sClient, applicationAutoscalingClient)
			request = CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(5))
		})

		It("should requeue immediately", func() {
			result, err := controller.Reconcile(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(true))
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})
	})

	Context("When the putScalingPolicy fails with server error", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			applicationAutoscalingClient := mockAutoscalingClientBuilder.
				AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
				AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
				AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
				AddDeregisterScalableTargetsResponse(applicationautoscaling.DeregisterScalableTargetOutput{}).
				AddRegisterScalableTargetsResponse(applicationautoscaling.RegisterScalableTargetOutput{}).
				AddPutScalingPolicyErrorResponse("InternalServiceException", "Server Error", 500, "request id").
				Build()
			controller = createReconciler(k8sClient, applicationAutoscalingClient)
			request = CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(6))
		})

		It("should requeue immediately", func() {
			result, err := controller.Reconcile(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(Equal(true))
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})
	})

	Context("When the second describe fails with a server error", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			applicationAutoscalingClient := mockAutoscalingClientBuilder.
				AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
				AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
				AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
				AddDeregisterScalableTargetsResponse(applicationautoscaling.DeregisterScalableTargetOutput{}).
				AddRegisterScalableTargetsResponse(applicationautoscaling.RegisterScalableTargetOutput{}).
				AddPutScalingPolicyResponse(applicationautoscaling.PutScalingPolicyOutput{}).
				AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{}).
				AddDescribeScalingPoliciesErrorResponse("InternalServiceException", "Server Error", 500, "request-id").
				Build()
			controller = createReconciler(k8sClient, applicationAutoscalingClient)
			request = CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(8))
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
				Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
				Name:      hostingautoscalingpolicy.ObjectMeta.Name,
			}, hostingautoscalingpolicy)
			Expect(err).ToNot(HaveOccurred())

			Expect(hostingautoscalingpolicy.Status.HostingAutoscalingPolicyStatus).To(Equal(ReconcilingAutoscalingJobStatus))
			Expect(hostingautoscalingpolicy.Status.Additional).To(ContainSubstring("Unable to describe ScalingPolicy"))
		})
	})

	Context("When the second describe returns 404", func() {
		BeforeEach(func() {
			receivedRequests = List{}
			mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)
			applicationAutoscalingClient := mockAutoscalingClientBuilder.
				AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
				AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
				AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
				AddDeregisterScalableTargetsResponse(applicationautoscaling.DeregisterScalableTargetOutput{}).
				AddRegisterScalableTargetsResponse(applicationautoscaling.RegisterScalableTargetOutput{}).
				AddPutScalingPolicyResponse(applicationautoscaling.PutScalingPolicyOutput{}).
				AddDescribeScalableTargetsResponse(applicationautoscaling.DescribeScalableTargetsOutput{}).
				AddDescribeScalingPoliciesErrorResponse("ValidationException", "Could not find HAP", 400, "request-id").
				Build()
			controller = createReconciler(k8sClient, applicationAutoscalingClient)
			request = CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		})

		AfterEach(func() {
			Expect(receivedRequests.Len()).To(Equal(8))
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
				Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
				Name:      hostingautoscalingpolicy.ObjectMeta.Name,
			}, hostingautoscalingpolicy)
			Expect(err).ToNot(HaveOccurred())

			Expect(hostingautoscalingpolicy.Status.HostingAutoscalingPolicyStatus).To(Equal(FailedAutoscalingJobStatus))
			Expect(hostingautoscalingpolicy.Status.Additional).To(ContainSubstring("Unable to describe ScalingPolicy"))
		})
	})
})

var _ = Describe("Reconciling an HAP with finalizer that is being deleted", func() {

	var (
		receivedRequests             List
		mockAutoscalingClientBuilder *MockAutoscalingClientBuilder
		hostingautoscalingpolicy     *hostingautoscalingpolicyv1.HostingAutoscalingPolicy
		outOfDateTargetDescription   applicationautoscaling.DescribeScalableTargetsOutput
		outOfDatePolicyDescription   applicationautoscaling.DescribeScalingPoliciesOutput
		policyName, policyArn        string
	)

	BeforeEach(func() {
		policyName = "test-policy-name"
		policyArn = "policy-arn"
		resourceID := "endpoint/endpoint-xyz/variant/variant-xyz"
		scalableTarget := applicationautoscaling.ScalableTarget{
			ResourceId: &resourceID,
		}
		scalableTargets := []applicationautoscaling.ScalableTarget{scalableTarget}
		scalingPolicy := &applicationautoscaling.ScalingPolicy{
			PolicyName: &policyName,
			PolicyARN:  &policyArn,
			ResourceId: &resourceID,
		}
		scalingPolicies := []*applicationautoscaling.ScalingPolicy{scalingPolicy}

		outOfDateTargetDescription = applicationautoscaling.DescribeScalableTargetsOutput{
			ScalableTargets: scalableTargets,
		}
		outOfDatePolicyDescription = applicationautoscaling.DescribeScalingPoliciesOutput{
			ScalingPolicies: scalingPolicies,
		}

		receivedRequests = List{}
		mockAutoscalingClientBuilder = NewMockAutoscalingClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		hostingautoscalingpolicy = createHAPWithFinalizer()
		err := k8sClient.Create(context.Background(), hostingautoscalingpolicy)
		hostingautoscalingpolicy.Status.ResourceIDList = []string{"endpoint/endpoint-xyz/variant/variant-xyz"}
		err = k8sClient.Status().Update(context.Background(), hostingautoscalingpolicy)

		Expect(err).ToNot(HaveOccurred())

		// Mark job as deleting in Kubernetes.
		err = k8sClient.Delete(context.Background(), hostingautoscalingpolicy)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should do nothing if the HAP is not in sagemaker", func() {
		applicationAutoscalingClient := mockAutoscalingClientBuilder.
			AddDescribeScalableTargetsErrorResponse("ValidationException", "Could not find HAP xyz", 400, "request-id").
			Build()

		controller := createReconciler(k8sClient, applicationAutoscalingClient)
		request := CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(receivedRequests.Len()).To(Equal(1))
	})

	It("should delete the HAP in sagemaker", func() {
		a := hostingautoscalingpolicyv1.HostingAutoscalingPolicy{}
		applicationAutoscalingClient := mockAutoscalingClientBuilder.
			AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
			AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
			AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
			AddDeregisterScalableTargetsResponse(applicationautoscaling.DeregisterScalableTargetOutput{}).
			Build()

		controller := createReconciler(k8sClient, applicationAutoscalingClient)
		request := CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		Expect(hostingautoscalingpolicy.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))
		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(receivedRequests.Len()).To(Equal(4))

		// entry should be deleted from k8s
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
			Name:      hostingautoscalingpolicy.ObjectMeta.Name,
		}, &a)

		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("Verify that finalizer is removed (or object is deleted) when HAP does not exist", func() {
		a := &hostingautoscalingpolicyv1.HostingAutoscalingPolicy{}
		applicationAutoscalingClient := mockAutoscalingClientBuilder.
			AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
			AddDescribeScalingPoliciesErrorResponse("ValidationException", "Could not find HAP xyz", 400, "request-id").
			Build()

		controller := createReconciler(k8sClient, applicationAutoscalingClient)
		request := CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))

		// We can't verify about finalizer but can make sure that object has been deleted from k8s
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: hostingautoscalingpolicy.ObjectMeta.Namespace,
			Name:      hostingautoscalingpolicy.ObjectMeta.Name,
		}, a)

		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
		Expect(receivedRequests.Len()).To(Equal(2))
	})

	It("Verify that finalizer is not removed and error returned when Delete fails", func() {
		applicationAutoscalingClient := mockAutoscalingClientBuilder.
			AddDescribeScalableTargetsResponse(outOfDateTargetDescription).
			AddDescribeScalingPoliciesResponse(outOfDatePolicyDescription).
			AddDeleteScalingPolicyResponse(applicationautoscaling.DeleteScalingPolicyOutput{}).
			AddDeregisterScalableTargetsErrorResponse("InternalServiceException", "Server Error", 500, "request-id").
			Build()

		controller := createReconciler(k8sClient, applicationAutoscalingClient)
		request := CreateReconciliationRequest(hostingautoscalingpolicy.ObjectMeta.Name, hostingautoscalingpolicy.ObjectMeta.Namespace)

		result, err := controller.Reconcile(request)

		// Should requeue
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(hostingautoscalingpolicy.ObjectMeta.GetFinalizers()).To(ContainElement(SageMakerResourceFinalizerName))
		Expect(receivedRequests.Len()).To(Equal(4))
	})
})

func createReconciler(k8sClient k8sclient.Client, applicationAutoscalingClient applicationautoscalingiface.ApplicationAutoScalingAPI) Reconciler {

	return Reconciler{
		Client:                             k8sClient,
		Log:                                ctrl.Log,
		createApplicationAutoscalingClient: CreateMockAutoscalingClientWrapperProvider(applicationAutoscalingClient),
		awsConfigLoader:                    CreateMockAwsConfigLoader(),
	}
}

func createHAPWithGeneratedNames() *hostingautoscalingpolicyv1.HostingAutoscalingPolicy {
	k8sName := "hap-" + uuid.New().String()
	k8sNamespace := "namespace-hap"
	CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)
	return createHAP(false, k8sName, k8sNamespace)
}

func createHAPWithFinalizer() *hostingautoscalingpolicyv1.HostingAutoscalingPolicy {
	k8sName := "hap-" + uuid.New().String()
	k8sNamespace := "namespace-hap"
	CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)
	return createHAP(true, k8sName, k8sNamespace)
}

func createHAP(withFinalizer bool, k8sName, k8sNamespace string) *hostingautoscalingpolicyv1.HostingAutoscalingPolicy {
	finalizers := []string{}

	if withFinalizer {
		finalizers = append(finalizers, SageMakerResourceFinalizerName)
	}
	return &hostingautoscalingpolicyv1.HostingAutoscalingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       k8sName,
			Namespace:  k8sNamespace,
			Finalizers: finalizers,
		},
		Spec: hostingautoscalingpolicyv1.HostingAutoscalingPolicySpec{
			PolicyName: ToStringPtr("test-policy-name"),
			ResourceID: []*commonv1.AutoscalingResource{
				{
					EndpointName: ToStringPtr("endpoint-xyz"),
					VariantName:  ToStringPtr("variant-xyz"),
				},
			},
			TargetTrackingScalingPolicyConfiguration: &commonv1.TargetTrackingScalingPolicyConfig{
				PredefinedMetricSpecification: &commonv1.PredefinedMetricSpecification{},
			},
			Region:      ToStringPtr("region-xyz"),
			MinCapacity: ToInt64Ptr(1),
			MaxCapacity: ToInt64Ptr(2),
		},
	}
}
