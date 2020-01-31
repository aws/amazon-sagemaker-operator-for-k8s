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

	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	endpointconfigv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/endpointconfig"
	hostingv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingdeployment"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	controllercommon "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers"
	"github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/sdkutil/clientwrapper"
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
		controller := createReconcilerWithMockedDependencies(k8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(false))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})

	It("should requeue if there was an error", func() {
		mockK8sClient := FailToGetK8sClient{}
		controller := createReconcilerWithMockedDependencies(mockK8sClient, sageMakerClient, "1s")

		request := CreateReconciliationRequest("non-existent-name", "namespace")

		result, err := controller.Reconcile(request)

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(Equal(true))
		Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
	})
})

var _ = Describe("Reconciling a HostingDeployment that exists", func() {

	var (
		// The requests received by the mock SageMaker client.
		receivedRequests List

		// SageMaker client builder used to create mock responses.
		mockSageMakerClientBuilder *MockSageMakerClientBuilder

		// The total number of requests added to the mock SageMaker client builder.
		expectedRequestCount int

		// Subreconciler that manages model creation in k8s.
		modelReconciler *mockModelReconciler

		// Subreconciler that manages endpoint config creation in k8s.
		endpointConfigReconciler *mockEndpointConfigReconciler

		// A list of mock SageMaker endpoint names that are returned by
		// the endpointConfigReconciler.
		sageMakerEndpointConfigNames List

		// The mock deployment.
		deployment *hostingv1.HostingDeployment

		// The kubernetes client to use in the test. This is different than the default
		// test client as some tests use a special test client.
		kubernetesClient k8sclient.Client

		// The poll duration that the controller is configured with.
		pollDuration string

		// A generated name to be used in the EndpointConfig.Status SageMaker name.
		endpointConfigSageMakerName string

		// Whether or not the test deployment should have deletion timestamp set.
		shouldHaveDeletionTimestamp bool

		// Whether or not the test deployment should have a finalizer.
		shouldHaveFinalizer bool

		// Whether or not the test should have an endpoint config.
		shouldHaveEndpointConfig bool

		// The controller result.
		reconcileResult ctrl.Result

		// The controller error result.
		reconcileError error
	)

	BeforeEach(func() {
		pollDuration = "1s"

		endpointConfigSageMakerName = "endpoint-config-" + uuid.New().String()

		shouldHaveDeletionTimestamp = false
		shouldHaveFinalizer = false
		shouldHaveEndpointConfig = false

		kubernetesClient = k8sClient

		receivedRequests = List{}
		mockSageMakerClientBuilder = NewMockSageMakerClientBuilder(GinkgoT()).WithRequestList(&receivedRequests)

		sageMakerEndpointConfigNames = List{}

		deployment = createDeploymentWithGeneratedNames()
	})

	JustBeforeEach(func() {

		modelReconciler = &mockModelReconciler{
			subreconcilerCallTracker: subreconcilerCallTracker{
				DesiredDeployments:          &List{},
				ShouldDeleteUnusedResources: &List{},
			},
		}
		endpointConfigReconciler = &mockEndpointConfigReconciler{
			subreconcilerCallTracker: subreconcilerCallTracker{
				DesiredDeployments:          &List{},
				ShouldDeleteUnusedResources: &List{},
				EndpointConfigNames:         &sageMakerEndpointConfigNames,
			},
		}

		sageMakerClient := mockSageMakerClientBuilder.Build()
		expectedRequestCount = mockSageMakerClientBuilder.GetAddedResponsesLen()

		controller := createReconciler(kubernetesClient, sageMakerClient, modelReconciler, endpointConfigReconciler, pollDuration)

		err := k8sClient.Create(context.Background(), deployment)
		Expect(err).ToNot(HaveOccurred())

		if shouldHaveFinalizer {
			AddFinalizer(deployment)
		}

		if shouldHaveDeletionTimestamp {
			SetDeletionTimestamp(deployment)
		}

		if shouldHaveEndpointConfig {
			CreateEndpointConfigWithSageMakerName(deployment, endpointConfigSageMakerName)
		}

		request := CreateReconciliationRequest(deployment.ObjectMeta.GetName(), deployment.ObjectMeta.GetNamespace())
		reconcileResult, reconcileError = controller.Reconcile(request)
	})

	AfterEach(func() {
		Expect(receivedRequests.Len()).To(Equal(expectedRequestCount), "Expect that all SageMaker responses were consumed")
	})

	Context("DescribeEndpoint fails", func() {

		var failureMessage string

		BeforeEach(func() {
			failureMessage = "error message " + uuid.New().String()
			mockSageMakerClientBuilder.AddDescribeEndpointErrorResponse("Exception", failureMessage, 500, "request id")
		})

		It("Requeues immediately", func() {
			ExpectRequeueImmediately(reconcileResult, reconcileError)
		})

		It("Updates status", func() {
			ExpectAdditionalToContain(deployment, failureMessage)
			ExpectStatusToBe(deployment, ReconcilingEndpointStatus)
		})
	})

	Context("Endpoint does not exist", func() {

		BeforeEach(func() {
			mockSageMakerClientBuilder.
				AddDescribeEndpointErrorResponse(clientwrapper.DescribeEndpoint404Code, clientwrapper.DescribeEndpoint404MessagePrefix, 400, "request id")
		})

		Context("HasDeletionTimestamp", func() {

			BeforeEach(func() {
				shouldHaveDeletionTimestamp = true
				shouldHaveFinalizer = true
			})

			It("Cleans up resources", func() {
				ExpectNthSubreconcilerCallToDeleteUnusedResources(modelReconciler, endpointConfigReconciler, 0)
			})

			It("Removes finalizer", func() {
				ExpectDeploymentToBeDeleted(deployment)
			})

			It("Requeues after interval", func() {
				ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
			})
		})

		Context("!HasDeletionTimestamp", func() {
			BeforeEach(func() {
				mockSageMakerClientBuilder.
					AddCreateEndpointResponse(sagemaker.CreateEndpointOutput{}).
					AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(sagemaker.EndpointStatusCreating))

				shouldHaveDeletionTimestamp = false
				shouldHaveFinalizer = true
				shouldHaveEndpointConfig = true

				// Add twice because there are two calls to GetSageMakerEndpointConfigName.
				sageMakerEndpointConfigNames.PushBack(endpointConfigSageMakerName)
				sageMakerEndpointConfigNames.PushBack(endpointConfigSageMakerName)
			})

			It("Creates necessary resources", func() {
				ExpectNthSubreconcilerCallToKeepUnusedResources(modelReconciler, endpointConfigReconciler, 0)
			})

			It("Creates an Endpoint", func() {

				req := receivedRequests.Front().Next().Value
				Expect(req).To(BeAssignableToTypeOf((*sagemaker.CreateEndpointInput)(nil)))

				createdRequest := req.(*sagemaker.CreateEndpointInput)
				Expect(*createdRequest.EndpointConfigName).To(Equal(endpointConfigSageMakerName))
				Expect(*createdRequest.EndpointName).To(Equal(GetGeneratedResourceName(deployment.ObjectMeta.GetUID(), deployment.ObjectMeta.GetName(), 63)))
			})

			It("Requeues after interval", func() {
				ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
			})

			It("Updates status", func() {
				ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusCreating))
			})
		})
	})

	Context("Endpoint exists", func() {

		var expectedStatus sagemaker.EndpointStatus

		BeforeEach(func() {
			shouldHaveFinalizer = true
			shouldHaveEndpointConfig = true
		})

		Context("Endpoint has status 'Creating'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.EndpointStatusCreating
				mockSageMakerClientBuilder.
					AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))

			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(deployment, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(deployment, controllercommon.SageMakerResourceFinalizerName)
					})
				})

			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Deleting' and does not delete HostingDeployment", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})

			})

		})

		Context("Endpoint has status 'Deleting'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.EndpointStatusDeleting
				mockSageMakerClientBuilder.
					AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))

			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(deployment, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(deployment, controllercommon.SageMakerResourceFinalizerName)
					})
				})

			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Deleting' and does not delete HostingDeployment", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})

			})

		})

		Context("Endpoint has status 'OutOfService'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.EndpointStatusOutOfService
				mockSageMakerClientBuilder.
					AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))

			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(deployment, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(deployment, controllercommon.SageMakerResourceFinalizerName)
					})
				})

			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Deleting' and does not delete HostingDeployment", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})

			})

		})

		Context("Endpoint has status 'RollingBack'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.EndpointStatusRollingBack
				mockSageMakerClientBuilder.
					AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))

			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(deployment, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(deployment, controllercommon.SageMakerResourceFinalizerName)
					})
				})

			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Deleting' and does not delete HostingDeployment", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})

			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(deployment, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(deployment, controllercommon.SageMakerResourceFinalizerName)
					})
				})

			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Deleting' and does not delete HostingDeployment", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})

			})

		})

		Context("Endpoint has status 'SystemUpdating'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.EndpointStatusSystemUpdating
				mockSageMakerClientBuilder.
					AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))

			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(deployment, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(deployment, controllercommon.SageMakerResourceFinalizerName)
					})
				})

			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Deleting' and does not delete HostingDeployment", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})

			})

		})

		Context("Endpoint has status 'Updating'", func() {
			BeforeEach(func() {
				expectedStatus = sagemaker.EndpointStatusUpdating
				mockSageMakerClientBuilder.
					AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))

			})

			When("!HasDeletionTimestamp", func() {
				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(deployment, string(expectedStatus))
				})

				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(deployment, controllercommon.SageMakerResourceFinalizerName)
					})
				})

			})

			When("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					shouldHaveDeletionTimestamp = true
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to 'Deleting' and does not delete HostingDeployment", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})

			})

		})

		Context("Endpoint has status 'Failed'", func() {

			BeforeEach(func() {
				expectedStatus = sagemaker.EndpointStatusFailed
				mockSageMakerClientBuilder.
					AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(expectedStatus))

			})

			Context("!HasDeletionTimestamp", func() {

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status", func() {
					ExpectStatusToBe(deployment, string(expectedStatus))
				})
				Context("Does not have a finalizer", func() {
					BeforeEach(func() {
						shouldHaveFinalizer = false
					})

					It("Adds a finalizer", func() {
						ExpectToHaveFinalizer(deployment, controllercommon.SageMakerResourceFinalizerName)
					})
				})
			})

			Context("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					mockSageMakerClientBuilder.
						AddDeleteEndpointResponse(sagemaker.DeleteEndpointOutput{})

					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the endpoint", func() {
					ExpectRequestToDeleteHostingDeployment(receivedRequests.Front().Next().Value, deployment)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to deleting", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})
			})
		})

		Context("Endpoint has status 'InService'", func() {

			Context("HasDeletionTimestamp", func() {
				BeforeEach(func() {
					mockSageMakerClientBuilder.
						AddDescribeEndpointResponse(CreateDescribeOutputWithOnlyStatus(sagemaker.EndpointStatusInService)).
						AddDeleteEndpointResponse(sagemaker.DeleteEndpointOutput{})

					shouldHaveDeletionTimestamp = true
				})

				It("Deletes the endpoint", func() {
					ExpectRequestToDeleteHostingDeployment(receivedRequests.Front().Next().Value, deployment)
				})

				It("Requeues after interval", func() {
					ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
				})

				It("Updates status to deleting", func() {
					ExpectStatusToBe(deployment, string(sagemaker.EndpointStatusDeleting))
				})
			})

			Context("!HasDeletionTimestamp", func() {
				BeforeEach(func() {
					// Add twice because there are two calls to GetSageMakerEndpointConfigName.
					sageMakerEndpointConfigNames.PushBack("")
					sageMakerEndpointConfigNames.PushBack(endpointConfigSageMakerName)
				})

				Context("The HostingDeployment endpointconfig name differs from SageMaker", func() {

					BeforeEach(func() {
						mockSageMakerClientBuilder.
							AddDescribeEndpointResponse(CreateDescribeOutput(sagemaker.EndpointStatusInService, "outdated-"+endpointConfigSageMakerName))
					})

					Context("The update succeeds", func() {
						BeforeEach(func() {
							mockSageMakerClientBuilder.
								AddUpdateEndpointResponse(sagemaker.UpdateEndpointOutput{EndpointArn: ToStringPtr("xyz")})
						})

						It("Calls UpdateEndpoint", func() {
							ExpectRequestToUpdateHostingDeployment(receivedRequests.Front().Next().Value, deployment, endpointConfigSageMakerName)
						})

						It("Requeues after interval", func() {
							ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
						})
					})

					Context("The update failed", func() {
						var errorMessage string

						BeforeEach(func() {
							errorMessage = "some server error"

							mockSageMakerClientBuilder.
								AddUpdateEndpointErrorResponse("Exception", errorMessage, 500, "request id")
						})

						It("Requeues immediately", func() {
							ExpectRequeueImmediately(reconcileResult, reconcileError)
						})

						It("Updates status", func() {
							ExpectAdditionalToContain(deployment, errorMessage)
							ExpectStatusToBe(deployment, ReconcilingEndpointStatus)
						})
					})
				})

				Context("The HostingDeployment endpointconfig name is the same as SageMaker", func() {
					BeforeEach(func() {
						mockSageMakerClientBuilder.
							AddDescribeEndpointResponse(CreateDescribeOutput(sagemaker.EndpointStatusInService, endpointConfigSageMakerName))
					})

					It("Creates resources", func() {
						ExpectNthSubreconcilerCallToKeepUnusedResources(modelReconciler, endpointConfigReconciler, 0)
					})

					It("Cleans up resources", func() {
						ExpectNthSubreconcilerCallToDeleteUnusedResources(modelReconciler, endpointConfigReconciler, 1)
					})

					It("Requeues after interval", func() {
						ExpectRequeueAfterInterval(reconcileResult, reconcileError, pollDuration)
					})
				})
			})
		})
	})

})

func createReconcilerWithMockedDependencies(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.ClientAPI, pollIntervalStr string) *HostingDeploymentReconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return &HostingDeploymentReconciler{
		Client:                         k8sClient,
		Log:                            ctrl.Log,
		PollInterval:                   pollInterval,
		createSageMakerClient:          CreateMockSageMakerClientProvider(sageMakerClient),
		awsConfigLoader:                CreateMockAwsConfigLoader(),
		createModelReconciler:          createModelReconcilerProvider(&mockModelReconciler{}),
		createEndpointConfigReconciler: createEndpointConfigReconcilerProvider(&mockEndpointConfigReconciler{}),
	}
}

func createReconciler(k8sClient k8sclient.Client, sageMakerClient sagemakeriface.ClientAPI, modelReconciler ModelReconciler, endpointConfigReconciler EndpointConfigReconciler, pollIntervalStr string) *HostingDeploymentReconciler {
	pollInterval := ParseDurationOrFail(pollIntervalStr)

	return &HostingDeploymentReconciler{
		Client:                         k8sClient,
		Log:                            ctrl.Log,
		PollInterval:                   pollInterval,
		createSageMakerClient:          CreateMockSageMakerClientProvider(sageMakerClient),
		awsConfigLoader:                CreateMockAwsConfigLoader(),
		createModelReconciler:          createModelReconcilerProvider(modelReconciler),
		createEndpointConfigReconciler: createEndpointConfigReconcilerProvider(endpointConfigReconciler),
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

// Mock implementation of EndpointConfigReconciler.
// This simply tracks invocations of Reconcile and the parameters it was called with.
// Return values are configurable.
type mockEndpointConfigReconciler struct {
	EndpointConfigReconciler
	subreconcilerCallTracker
}

// Mock implementation of Reconcile. This stores the parameters it was called with in the mock. It also will return a ReturnValue
// in each invocation.
func (r *mockEndpointConfigReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, shouldDeleteUnusedResources bool) error {
	return r.TrackAll(desiredDeployment, shouldDeleteUnusedResources)
}

// Mock implementation of GetSageMakerEndpointConfigName
func (r *mockEndpointConfigReconciler) GetSageMakerEndpointConfigName(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (string, error) {
	if r.EndpointConfigNames != nil && r.EndpointConfigNames.Len() > 0 {
		front := r.EndpointConfigNames.Front()
		r.EndpointConfigNames.Remove(front)
		return front.Value.(string), nil
	} else {
		return "", fmt.Errorf("no SageMaker endpoint config name provided for mockEndpointConfigReconciler")
	}
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
func (r *mockModelReconciler) Reconcile(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment, shouldDeletedUnusedModels bool) error {
	return r.TrackAll(desiredDeployment, shouldDeletedUnusedModels)
}

// Mock implementation of GetSageMakerModelNames.
func (r *mockModelReconciler) GetSageMakerModelNames(ctx context.Context, desiredDeployment *hostingv1.HostingDeployment) (map[string]string, error) {
	return map[string]string{}, nil
}

// Call tracker for sub reconcilers (ModelReconciler/EndpointConfigReconciler).
// Common logic and variables are refactored into this common struct.
// This simply tracks invocations of Reconcile and the parameters it was called with.
// Return values are configurable.
type subreconcilerCallTracker struct {

	// A list of HostingDeployments that are passed to Reconcile. This is useful if a test wants
	// to verify that parameters were correctly passed.
	// This must be non-nil in order for HostingDeployments to be stored here.
	DesiredDeployments *List

	ShouldDeleteUnusedResources *List

	// A list of errors that are returned from the mock Reconcile.
	// If this is nil, or if the number of calls to Reconcile is greater than the number of elements
	// originally in this list, Reconcile will return nil.
	ReconcileReturnValues *List

	EndpointConfigNames *List
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

func (r *subreconcilerCallTracker) TrackAll(desiredDeployment *hostingv1.HostingDeployment, shouldDeleteUnusedResources bool) error {

	if r.ShouldDeleteUnusedResources != nil {
		r.ShouldDeleteUnusedResources.PushBack(shouldDeleteUnusedResources)
	}

	return r.TrackOnlyDesiredDeployment(desiredDeployment)
}

// Helper function to return the parameters used for the Nth invocation of the subreconciler.
func (r *subreconcilerCallTracker) GetNthReconcileCall(index int) (*hostingv1.HostingDeployment, bool) {

	if r.DesiredDeployments == nil {
		Fail("Unable to get nth reconcile call because DesiredDeployment is nil")
	}

	if r.ShouldDeleteUnusedResources == nil {
		Fail("Unable to get nth reconcile call because ShouldDeleteUnusedResources is nil")
	}

	desiredDeploymentElement := r.DesiredDeployments.Front()
	shouldDeleteUnusedResourcesElement := r.ShouldDeleteUnusedResources.Front()
	for i := 0; i < index; i++ {
		desiredDeploymentElement = desiredDeploymentElement.Next()
		shouldDeleteUnusedResourcesElement = shouldDeleteUnusedResourcesElement.Next()
	}

	return desiredDeploymentElement.Value.(*hostingv1.HostingDeployment), shouldDeleteUnusedResourcesElement.Value.(bool)
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
			Models: []commonv1.Model{},
		},
	}
}

// Expect deployment.Status.Additional to contain the specified string.
func ExpectAdditionalToContain(deployment *hostingv1.HostingDeployment, substring string) {
	var actual hostingv1.HostingDeployment
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: deployment.ObjectMeta.Namespace,
		Name:      deployment.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.Status.Additional).To(ContainSubstring(substring))
}

// Expect deployment status to be as specified.
func ExpectStatusToBe(deployment *hostingv1.HostingDeployment, status string) {
	var actual hostingv1.HostingDeployment
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: deployment.ObjectMeta.Namespace,
		Name:      deployment.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(string(actual.Status.EndpointStatus)).To(Equal(status))
}

// Expect the deployment to have the specified finalizer.
func ExpectToHaveFinalizer(deployment *hostingv1.HostingDeployment, finalizer string) {
	var actual hostingv1.HostingDeployment
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: deployment.ObjectMeta.Namespace,
		Name:      deployment.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(actual.ObjectMeta.Finalizers).To(ContainElement(finalizer))
}

// Set the deletion timestamp to be nonzero.
func SetDeletionTimestamp(deployment *hostingv1.HostingDeployment) {
	var actual hostingv1.HostingDeployment
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: deployment.ObjectMeta.Namespace,
		Name:      deployment.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	Expect(k8sClient.Delete(context.Background(), &actual)).To(Succeed())
}

// Expect the deployment to not exist.
func ExpectDeploymentToBeDeleted(deployment *hostingv1.HostingDeployment) {
	var actual hostingv1.HostingDeployment
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: deployment.ObjectMeta.Namespace,
		Name:      deployment.ObjectMeta.Name,
	}, &actual)
	Expect(err).To(HaveOccurred())
	Expect(apierrs.IsNotFound(err)).To(Equal(true))
}

// Add a finalizer to the deployment.
func AddFinalizer(deployment *hostingv1.HostingDeployment) {
	var actual hostingv1.HostingDeployment
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: deployment.ObjectMeta.Namespace,
		Name:      deployment.ObjectMeta.Name,
	}, &actual)
	Expect(err).ToNot(HaveOccurred())

	actual.ObjectMeta.Finalizers = []string{controllercommon.SageMakerResourceFinalizerName}

	Expect(k8sClient.Update(context.Background(), &actual)).To(Succeed())
}

// Create an EndpointConfig with a SageMaker name in the status.
func CreateEndpointConfigWithSageMakerName(deployment *hostingv1.HostingDeployment, endpointConfigSageMakerName string) {
	namespacedName := GetKubernetesNamespacedName(deployment.ObjectMeta.GetName(), *deployment)

	endpointConfig := endpointconfigv1.EndpointConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    GetResourceOwnershipLabelsForHostingDeployment(*deployment),
		},
		Spec: endpointconfigv1.EndpointConfigSpec{
			Region: deployment.Spec.Region,
			ProductionVariants: []commonv1.ProductionVariant{
				{
					InitialInstanceCount: ToInt64Ptr(5),
					InstanceType:         "instance-type",
					ModelName:            ToStringPtr("model-name"),
					VariantName:          ToStringPtr("variant-name"),
				},
			},
		},
		Status: endpointconfigv1.EndpointConfigStatus{
			SageMakerEndpointConfigName: endpointConfigSageMakerName,
		},
	}

	Expect(k8sClient.Create(context.Background(), &endpointConfig)).ToNot(HaveOccurred())
	Expect(k8sClient.Status().Update(context.Background(), &endpointConfig)).ToNot(HaveOccurred())
}

// Expect n-th subreconciler call to not attempt to delete unused resources.
func ExpectNthSubreconcilerCallToKeepUnusedResources(modelReconciler *mockModelReconciler, endpointConfigReconciler *mockEndpointConfigReconciler, index int) {
	ExpectNthSubreconcilerCallToHaveShouldDelete(modelReconciler, endpointConfigReconciler, index, false)
}

// Expect n-th subreconciler call to attempt to delete unused resources.
func ExpectNthSubreconcilerCallToDeleteUnusedResources(modelReconciler *mockModelReconciler, endpointConfigReconciler *mockEndpointConfigReconciler, index int) {
	ExpectNthSubreconcilerCallToHaveShouldDelete(modelReconciler, endpointConfigReconciler, index, true)
}

// Expect n-th subreconciler call to have a certain shouldDeleteUnusedResources value.
func ExpectNthSubreconcilerCallToHaveShouldDelete(modelReconciler *mockModelReconciler, endpointConfigReconciler *mockEndpointConfigReconciler, index int, expected bool) {
	var shouldDeleteUnusedResources bool
	_, shouldDeleteUnusedResources = modelReconciler.GetNthReconcileCall(index)
	Expect(shouldDeleteUnusedResources).To(Equal(expected))
	_, shouldDeleteUnusedResources = endpointConfigReconciler.GetNthReconcileCall(index)
	Expect(shouldDeleteUnusedResources).To(Equal(expected))
}

// Helper function to verify that the specified object is a DeleteEndpointInput and that it requests to delete the HostingDeployment.
func ExpectRequestToDeleteHostingDeployment(req interface{}, deployment *hostingv1.HostingDeployment) {
	Expect(req).To(BeAssignableToTypeOf((*sagemaker.DeleteEndpointInput)(nil)))

	deleteRequest := req.(*sagemaker.DeleteEndpointInput)
	Expect(*deleteRequest.EndpointName).To(Equal(GetGeneratedResourceName(deployment.ObjectMeta.GetUID(), deployment.ObjectMeta.GetName(), 63)))
}

// Helper function to verify that the specified object is n UpdateEndpointInput and that it requests to update the HostingDeployment correctly.
func ExpectRequestToUpdateHostingDeployment(req interface{}, deployment *hostingv1.HostingDeployment, expectedEndpointConfigName string) {
	Expect(req).To(BeAssignableToTypeOf((*sagemaker.UpdateEndpointInput)(nil)))

	updateRequest := req.(*sagemaker.UpdateEndpointInput)
	Expect(*updateRequest.EndpointName).To(Equal(GetGeneratedResourceName(deployment.ObjectMeta.GetUID(), deployment.ObjectMeta.GetName(), 63)))
	Expect(*updateRequest.EndpointConfigName).To(Equal(expectedEndpointConfigName))
}

// Helper function to create a DescribeEndpointOutput.
func CreateDescribeOutputWithOnlyStatus(status sagemaker.EndpointStatus) sagemaker.DescribeEndpointOutput {
	return sagemaker.DescribeEndpointOutput{
		EndpointStatus: status,
	}
}

// Helper function to create a DescribeEndpointOutput.
func CreateDescribeOutput(status sagemaker.EndpointStatus, endpointConfigName string) sagemaker.DescribeEndpointOutput {
	output := CreateDescribeOutputWithOnlyStatus(status)
	output.EndpointConfigName = &endpointConfigName
	return output
}
