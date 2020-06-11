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

import (
	"context"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/common/v1"
	endpointconfigv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/endpointconfig/v1"
	hostingv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/hostingdeployment/v1"
	modelv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/model/v1"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	endpointconfigcontroller "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/endpointconfig"
	modelcontroller "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/model"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("EndpointConfigReconciler.Reconcile", func() {

	var (
		reconciler EndpointConfigReconciler
		desired    hostingv1.HostingDeployment
	)

	BeforeEach(func() {
		reconciler = NewEndpointConfigReconciler(k8sClient, ctrl.Log)

		desired = createHostingDeploymentWithGeneratedNames()

	})

	It("Returns an error when a ProductionVariant does not have a VariantName", func() {
		desired.Spec.ProductionVariants = []commonv1.ProductionVariant{
			{
				InitialInstanceCount: ToInt64Ptr(1),
				InstanceType:         "instance-type",
				ModelName:            ToStringPtr("model-name"),
			},
		}

		err := reconciler.Reconcile(context.Background(), &desired, true)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ProductionVariant has nil VariantName"))
	})

	It("Returns an error when a ProductionVariant does not have a ModelName", func() {

		desired.Spec.ProductionVariants = []commonv1.ProductionVariant{
			{
				InitialInstanceCount: ToInt64Ptr(1),
				InstanceType:         "instance-type",
				VariantName:          ToStringPtr("variant-name"),
			},
		}

		err := reconciler.Reconcile(context.Background(), &desired, true)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ProductionVariant"))
		Expect(err.Error()).To(ContainSubstring("has nil ModelName"))
	})

	Context("Fail to get k8s model", func() {
		BeforeEach(func() {
			reconciler = NewEndpointConfigReconciler(FailToGetK8sClient{}, ctrl.Log)
			desired = createHostingDeploymentWithBasicProductionVariant()
		})

		It("Returns error if unable to get K8s EndpointConfig", func() {
			err := reconciler.Reconcile(context.Background(), &desired, true)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unable to resolve SageMaker model name for model"))
		})
	})

	It("Will not create if an EndpointConfig exists already and they are deep equal", func() {
		desired = createHostingDeploymentWithBasicProductionVariant()
		key := GetSubresourceNamespacedName(desired.ObjectMeta.GetName(), desired)

		err := k8sClient.Create(context.Background(), &endpointconfigv1.EndpointConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels:    GetResourceOwnershipLabelsForHostingDeployment(desired),
			},
			Spec: endpointconfigv1.EndpointConfigSpec{
				ProductionVariants: []commonv1.ProductionVariant{
					{
						InitialInstanceCount: ToInt64Ptr(1),
						InstanceType:         "instance-type",
						VariantName:          ToStringPtr("variant-name"),
						ModelName:            ToStringPtr("model-name"),
					},
				},
				Region: desired.Spec.Region,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		updateEndpointConfigStatus(key, endpointconfigcontroller.CreatedStatus, "sagemaker-endpoint-name")

		modelName := *desired.Spec.ProductionVariants[0].ModelName
		modelNamespacedName := GetSubresourceNamespacedName(modelName, desired)
		Expect(createCreatedModelWithAnySageMakerName(modelNamespacedName, desired)).ToNot(HaveOccurred())

		// Fail test if Create is called on k8s client.
		// We expect Create to not be called for an existing deep equal endpoint config.
		reconciler = NewEndpointConfigReconciler(FailTestOnCreateK8sClient{
			ActualClient: k8sClient,
		}, ctrl.Log)
		err = reconciler.Reconcile(context.Background(), &desired, true)

		Expect(err).ToNot(HaveOccurred())
	})

	Context("Fail to create k8s EndpointConfig", func() {

		BeforeEach(func() {
			reconciler = NewEndpointConfigReconciler(FailToCreateK8sClient{
				ActualClient: k8sClient,
			}, ctrl.Log)

			desired = createHostingDeploymentWithBasicProductionVariant()

			// Create model correct status
			modelName := *desired.Spec.ProductionVariants[0].ModelName
			modelNamespacedName := GetSubresourceNamespacedName(modelName, desired)

			Expect(createCreatedModelWithAnySageMakerName(modelNamespacedName, desired)).ToNot(HaveOccurred())
		})

		It("Returns error if unable to create k8s EndpointConfig", func() {
			err := reconciler.Reconcile(context.Background(), &desired, true)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unable to create Kubernetes EndpointConfig"))
		})
	})

	Context("The deployment spec is valid", func() {

		var (
			tagKey                               string
			tagValue                             string
			expectedEndpointConfigNamespacedName types.NamespacedName
		)

		BeforeEach(func() {
			tagKey = "tag-key"
			tagValue = "tag-value"

			desired = createHostingDeploymentWithBasicProductionVariant()
			desired.Spec.Tags = []commonv1.Tag{
				{
					Key:   &tagKey,
					Value: &tagValue,
				},
			}

			expectedEndpointConfigNamespacedName = GetSubresourceNamespacedName(desired.ObjectMeta.GetName(), desired)

			// Create models

			modelName := *desired.Spec.ProductionVariants[0].ModelName
			modelNamespacedName := GetSubresourceNamespacedName(modelName, desired)
			Expect(createCreatedModelWithAnySageMakerName(modelNamespacedName, desired)).ToNot(HaveOccurred())

			reconciler.Reconcile(context.Background(), &desired, true)
		})

		AfterEach(func() {
			var endpointConfig endpointconfigv1.EndpointConfig
			err := k8sClient.Get(context.Background(), expectedEndpointConfigNamespacedName, &endpointConfig)
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Delete(context.Background(), &endpointConfig)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Created the k8s endpointconfig with correct ProductionVariant", func() {
			var endpointConfig endpointconfigv1.EndpointConfig
			err := k8sClient.Get(context.Background(), expectedEndpointConfigNamespacedName, &endpointConfig)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(endpointConfig.Spec.ProductionVariants)).To(Equal(1))
			Expect(*endpointConfig.Spec.ProductionVariants[0].InitialInstanceCount).To(Equal(*desired.Spec.ProductionVariants[0].InitialInstanceCount))
			Expect(endpointConfig.Spec.ProductionVariants[0].InstanceType).To(Equal(desired.Spec.ProductionVariants[0].InstanceType))
			Expect(*endpointConfig.Spec.ProductionVariants[0].VariantName).To(Equal(*desired.Spec.ProductionVariants[0].VariantName))
		})

		It("Created the k8s endpointconfig with correct region", func() {
			var endpointConfig endpointconfigv1.EndpointConfig
			err := k8sClient.Get(context.Background(), expectedEndpointConfigNamespacedName, &endpointConfig)
			Expect(err).ToNot(HaveOccurred())

			Expect(endpointConfig.Spec.Region).ToNot(BeNil())
			Expect(*endpointConfig.Spec.Region).To(Equal(*desired.Spec.Region))
		})

		It("Created the k8s endpointconfig with correct tags", func() {
			var endpointConfig endpointconfigv1.EndpointConfig
			err := k8sClient.Get(context.Background(), expectedEndpointConfigNamespacedName, &endpointConfig)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(endpointConfig.Spec.Tags)).To(Equal(1))
			Expect(endpointConfig.Spec.Tags[0].Key).ToNot(BeNil())
			Expect(*endpointConfig.Spec.Tags[0].Key).To(Equal(tagKey))
			Expect(endpointConfig.Spec.Tags[0].Value).ToNot(BeNil())
			Expect(*endpointConfig.Spec.Tags[0].Value).To(Equal(tagValue))
		})
	})
})

var _ = Describe("Delete EndpointConfigReconciler.Reconcile", func() {
	var (
		tagKey                               string
		tagValue                             string
		expectedEndpointConfigNamespacedName types.NamespacedName
		reconciler                           EndpointConfigReconciler
		desired                              hostingv1.HostingDeployment
	)

	BeforeEach(func() {
		tagKey = "tag-key"
		tagValue = "tag-value"

		reconciler = NewEndpointConfigReconciler(k8sClient, ctrl.Log)

		desired = createHostingDeploymentWithBasicProductionVariant()
		desired.Spec.Tags = []commonv1.Tag{
			{
				Key:   &tagKey,
				Value: &tagValue,
			},
		}

		expectedEndpointConfigNamespacedName = GetSubresourceNamespacedName(desired.ObjectMeta.GetName(), desired)

		// Create models

		modelName := *desired.Spec.ProductionVariants[0].ModelName
		modelNamespacedName := GetSubresourceNamespacedName(modelName, desired)
		Expect(createCreatedModelWithAnySageMakerName(modelNamespacedName, desired)).ToNot(HaveOccurred())

		reconciler.Reconcile(context.Background(), &desired, true)

		var endpointConfig endpointconfigv1.EndpointConfig
		err := k8sClient.Get(context.Background(), expectedEndpointConfigNamespacedName, &endpointConfig)
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Delete(context.Background(), &endpointConfig)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Verify that endpoint config has been deleted from k8s", func() {
		var endpointConfig endpointconfigv1.EndpointConfig
		err := k8sClient.Get(context.Background(), expectedEndpointConfigNamespacedName, &endpointConfig)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})

	It("Verify that reconciler returns error if deletion fails", func() {
		//TODO Gautam Kumar
	})
})

var _ = Describe("Update EndpointConfigReconciler.Reconcile", func() {

	var (
		reconciler EndpointConfigReconciler

		desired                      *hostingv1.HostingDeployment
		endpointConfigNamespacedName types.NamespacedName
		modelName                    string
	)

	BeforeEach(func() {
		reconciler = NewEndpointConfigReconciler(k8sClient, ctrl.Log)
		modelName = "model-name"
		containers := []*commonv1.ContainerDefinition{
			{
				ContainerHostname: ToStringPtr("present-container"),
				ModelDataUrl:      ToStringPtr("s3://bucket/model.tar.gz"),
			},
		}

		desired = &hostingv1.HostingDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "k8s-name-" + uuid.New().String(),
				Namespace: "k8s-namespace-" + uuid.New().String(),
				UID:       types.UID(uuid.New().String()),
			},
			Spec: hostingv1.HostingDeploymentSpec{
				Region: ToStringPtr("us-east-1"),
				ProductionVariants: []commonv1.ProductionVariant{
					{
						InitialVariantWeight: ToInt64Ptr(1),
						InitialInstanceCount: ToInt64Ptr(4),
						VariantName:          ToStringPtr("variant-A"),
						ModelName:            &modelName,
					},
				},
				Models: []commonv1.Model{
					{
						Name:             &modelName,
						Containers:       containers,
						ExecutionRoleArn: ToStringPtr("xxx-yyy"),
					},
				},
			},
		}

		endpointConfigNamespacedName = GetSubresourceNamespacedName(desired.ObjectMeta.GetName(), *desired)
		err := createCreatedEndpointConfig(endpointConfigNamespacedName, *desired, "")
		Expect(err).ToNot(HaveOccurred())

		err = createCreatedModelWithAnySageMakerName(GetSubresourceNamespacedName(modelName, *desired), *desired)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		var endpointConfig endpointconfigv1.EndpointConfig
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: endpointConfigNamespacedName.Namespace,
			Name:      endpointConfigNamespacedName.Name,
		}, &endpointConfig)
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Delete(context.Background(), &endpointConfig)
		Expect(err).ToNot(HaveOccurred())

		modelNamespacedName := GetSubresourceNamespacedName(modelName, *desired)
		var model modelv1.Model
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: modelNamespacedName.Namespace,
			Name:      modelNamespacedName.Name,
		}, &model)
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Delete(context.Background(), &model)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Updates Kubernetes EndpointConfig", func() {
		newWeight := int64(5)
		updated := desired.DeepCopy()
		updated.Spec.ProductionVariants[0].InitialVariantWeight = &newWeight

		err := reconciler.Reconcile(context.Background(), updated, true)
		Expect(err).ToNot(HaveOccurred())

		var endpointConfig endpointconfigv1.EndpointConfig
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: endpointConfigNamespacedName.Namespace,
			Name:      endpointConfigNamespacedName.Name,
		}, &endpointConfig)
		Expect(err).ToNot(HaveOccurred())

		Expect(*endpointConfig.Spec.ProductionVariants[0].InitialVariantWeight).To(Equal(newWeight))
	})
})

// Create a K8s model that would have been created by ModelReconciler.
func createCreatedModelWithAnySageMakerName(namespacedName types.NamespacedName, deployment hostingv1.HostingDeployment) error {
	return createCreatedModelWithSageMakerName(namespacedName, deployment, "sagemaker-model-name")
}

// Create a K8s model that would have been created by ModelReconciler.
func createCreatedModelWithSageMakerName(namespacedName types.NamespacedName, deployment hostingv1.HostingDeployment, sageMakerName string) error {
	err := k8sClient.Create(context.Background(), &modelv1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    GetResourceOwnershipLabelsForHostingDeployment(deployment),
		},
		Spec: modelv1.ModelSpec{
			ExecutionRoleArn: ToStringPtr("xxx"),
			Region:           ToStringPtr(*deployment.Spec.Region),
		},
	})
	if err != nil {
		return err
	}

	var model modelv1.Model
	err = k8sClient.Get(context.Background(), namespacedName, &model)
	if err != nil {
		return err
	}

	model.Status.Status = modelcontroller.CreatedStatus
	model.Status.SageMakerModelName = sageMakerName
	err = k8sClient.Status().Update(context.Background(), &model)
	if err != nil {
		return err
	}

	return nil
}

// Create a K8s EndpointConfig that would have been created by EndpointConfigReconciler.
func createCreatedEndpointConfig(namespacedName types.NamespacedName, deployment hostingv1.HostingDeployment, sageMakerName string) error {

	err := k8sClient.Create(context.Background(), &endpointconfigv1.EndpointConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels:    GetResourceOwnershipLabelsForHostingDeployment(deployment),
		},
		Spec: endpointconfigv1.EndpointConfigSpec{
			Region:             ToStringPtr(*deployment.Spec.Region),
			ProductionVariants: deployment.Spec.ProductionVariants,
		},
	})
	if err != nil {
		return err
	}

	var endpointConfig endpointconfigv1.EndpointConfig
	err = k8sClient.Get(context.Background(), namespacedName, &endpointConfig)
	if err != nil {
		return err
	}

	endpointConfig.Status.SageMakerEndpointConfigName = sageMakerName
	endpointConfig.Status.Status = endpointconfigcontroller.CreatedStatus
	err = k8sClient.Status().Update(context.Background(), &endpointConfig)
	if err != nil {
		return err
	}

	return nil
}

func createHostingDeploymentWithBasicProductionVariant() hostingv1.HostingDeployment {
	deployment := createHostingDeploymentWithGeneratedNames()
	deployment.Spec.ProductionVariants = []commonv1.ProductionVariant{
		{
			InitialInstanceCount: ToInt64Ptr(1),
			InstanceType:         "instance-type",
			VariantName:          ToStringPtr("variant-name"),
			ModelName:            ToStringPtr("model-name"),
		},
	}

	return deployment
}

func createHostingDeploymentWithGeneratedNames() hostingv1.HostingDeployment {
	return createHostingDeployment("endpointconfig-"+uuid.New().String(), "namespace-"+uuid.New().String())
}

func createHostingDeployment(k8sName, k8sNamespace string) hostingv1.HostingDeployment {
	return hostingv1.HostingDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sName,
			Namespace: k8sNamespace,
			UID:       types.UID(uuid.New().String()),
		},
		Spec: hostingv1.HostingDeploymentSpec{
			ProductionVariants: []commonv1.ProductionVariant{},
			Models:             []commonv1.Model{},
			Region:             ToStringPtr("us-east-1"),
		},
	}
}

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
