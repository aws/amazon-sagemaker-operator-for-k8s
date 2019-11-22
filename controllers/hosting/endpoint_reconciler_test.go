/*

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

/*
// TODO(cdnamz) I am keeping these commented instead of deleting them as some bits will be useful when writing HostingDeployment controller tests.

import (
	. "container/list"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/sagemakeriface"
	commonv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/common"
	endpointconfigv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/endpointconfig"
	hostingv1 "go.amzn.com/sagemaker/sagemaker-k8s-operator/api/v1/hostingdeployment"
	. "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/controllertest"
	endpointconfigcontroller "go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/endpointconfig"
	"go.amzn.com/sagemaker/sagemaker-k8s-operator/controllers/sdkutil/clientwrapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("EndpointReconciler.Reconcile", func() {

	var (
		reconciler      EndpointReconciler
		desired         hostingv1.HostingDeployment
		actual          *sagemaker.DescribeEndpointOutput
		sageMakerClient sagemakeriface.ClientAPI
	)

	BeforeEach(func() {
		Skip("Fix me later")
		sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).Build()
		reconciler = NewEndpointReconciler(k8sClient, ctrl.Log, clientwrapper.NewSageMakerClientWrapper(sageMakerClient))

		desired = createHostingDeploymentWithGeneratedNames()
		actual = nil
	})

	It("should return an error if the EndpointConfig is not found", func() {
		Skip("Fix me later")

		err := reconciler.Reconcile(context.Background(), &desired, actual)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Unable to resolve SageMaker EndpointConfig name for EndpointConfig"))
	})

	It("should return an error if there is an error obtaining the EndpointConfig", func() {
		Skip("Fix me later")
		reconciler = NewEndpointReconciler(FailToGetK8sClient{}, ctrl.Log, clientwrapper.NewSageMakerClientWrapper(sageMakerClient))

		err := reconciler.Reconcile(context.Background(), &desired, actual)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Unable to resolve SageMaker EndpointConfig name for EndpointConfig"))

	})

	Context("With valid EndpointConfig", func() {
		var endpointConfigNamespacedName types.NamespacedName

		BeforeEach(func() {
			Skip("Fix me later")
			endpointConfigNamespacedName = GetKubernetesEndpointConfigNamespacedName(desired)
			endpointConfig := createEndpointConfig(endpointConfigNamespacedName.Name, endpointConfigNamespacedName.Namespace, "us-east-1")

			err := k8sClient.Create(context.Background(), &endpointConfig)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error if the EndpointConfig does not have a SageMakerEndpointConfigName", func() {
			err := updateEndpointConfigStatus(endpointConfigNamespacedName, endpointconfigcontroller.CreatedStatus, "")
			Expect(err).ToNot(HaveOccurred())

			err = reconciler.Reconcile(context.Background(), &desired, actual)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not have a SageMakerEndpointConfigName"))
		})

		Context("With an EndpointConfig in SageMaker", func() {

			var sageMakerEndpointName string

			BeforeEach(func() {
				sageMakerEndpointName = "sagemaker-endpoint-name"
				err := updateEndpointConfigStatus(endpointConfigNamespacedName, endpointconfigcontroller.CreatedStatus, sageMakerEndpointName)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an error if the SageMaker client fails to create the endpoint", func() {
				errorMessage := "Some SageMaker failure message"
				sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).
					AddCreateEndpointErrorResponse("SomeException", errorMessage, 500, "request id").
					Build()
				reconciler = NewEndpointReconciler(k8sClient, ctrl.Log, clientwrapper.NewSageMakerClientWrapper(sageMakerClient))

				err := reconciler.Reconcile(context.Background(), &desired, actual)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(errorMessage))
			})

			It("should succeed if the SageMaker creation succeeds", func() {
				requestList := List{}
				sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).
					WithRequestList(&requestList).
					AddCreateEndpointResponse(sagemaker.CreateEndpointOutput{
						EndpointArn: ToStringPtr("arn"),
					}).
					Build()
				reconciler = NewEndpointReconciler(k8sClient, ctrl.Log, clientwrapper.NewSageMakerClientWrapper(sageMakerClient))

				err := reconciler.Reconcile(context.Background(), &desired, actual)

				Expect(err).ToNot(HaveOccurred())
				Expect(requestList.Len()).To(Equal(1))
			})

			It("should correctly send the Endpoint spec", func() {
				requestList := List{}
				sageMakerClient = NewMockSageMakerClientBuilder(GinkgoT()).
					WithRequestList(&requestList).
					AddCreateEndpointResponse(sagemaker.CreateEndpointOutput{
						EndpointArn: ToStringPtr("arn"),
					}).
					Build()

				reconciler = NewEndpointReconciler(k8sClient, ctrl.Log, clientwrapper.NewSageMakerClientWrapper(sageMakerClient))

				tagKey := "tag-key"
				tagValue := "tag-value"
				desired.Spec.Tags = []commonv1.Tag{
					{
						Key:   ToStringPtr(tagKey),
						Value: ToStringPtr(tagValue),
					},
				}

				reconciler.Reconcile(context.Background(), &desired, actual)

				Expect(requestList.Len()).To(Equal(1))
				createEndpointRequest := requestList.Front().Value.(*sagemaker.CreateEndpointInput)
				Expect(createEndpointRequest).ToNot(BeNil())

				Expect(createEndpointRequest.EndpointConfigName).ToNot(BeNil())
				Expect(*createEndpointRequest.EndpointConfigName).To(Equal(sageMakerEndpointName))

				Expect(createEndpointRequest.EndpointName).ToNot(BeNil())
				Expect(*createEndpointRequest.EndpointName).To(Equal(GetSageMakerEndpointName(desired)))

				Expect(len(createEndpointRequest.Tags)).To(Equal(1))
				Expect(createEndpointRequest.Tags[0].Key).ToNot(BeNil())
				Expect(*createEndpointRequest.Tags[0].Key).To(Equal(tagKey))
				Expect(createEndpointRequest.Tags[0].Value).ToNot(BeNil())
				Expect(*createEndpointRequest.Tags[0].Value).To(Equal(tagValue))
			})
		})
	})

})

var _ = Describe("Delete EndpointReconciler.Reconcile", func() {

	var ()

	BeforeEach(func() {

	})

	It("should succeed the deletion of endpoint in k8s ", func() {
		//TODO Gautam Kumar
	})

	It("should succeed the deletion of endpoint in sagemaker", func() {
		//TODO Gautam Kumar
	})
})

func updateEndpointConfigStatus(namespacedName types.NamespacedName, status, sageMakerEndpointConfigName string) error {
	var endpointconfig endpointconfigv1.EndpointConfig
	err := k8sClient.Get(context.Background(), namespacedName, &endpointconfig)
	if err != nil {
		return err
	}

	endpointconfig.Status.Status = status
	endpointconfig.Status.SageMakerEndpointConfigName = sageMakerEndpointConfigName
	err = k8sClient.Status().Update(context.Background(), &endpointconfig)
	if err != nil {
		return err
	}

	return nil
}

func createEndpointConfig(name, namespace, region string) endpointconfigv1.EndpointConfig {
	return endpointconfigv1.EndpointConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: endpointconfigv1.EndpointConfigSpec{
			Region: &region,
			ProductionVariants: []commonv1.ProductionVariant{
				{
					InitialInstanceCount: ToInt64Ptr(5),
					InstanceType:         "instance-type",
					ModelName:            ToStringPtr("model-name"),
					VariantName:          ToStringPtr("variant-name"),
				},
			},
		},
	}
}
*/
