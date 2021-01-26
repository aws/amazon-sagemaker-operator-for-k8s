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

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	hostingv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/hostingdeployment"
	modelv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/model"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrs "k8s.io/apimachinery/pkg/api/errors"

	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("ModelReconciler.Reconcile", func() {

	var (
		reconciler ModelReconciler
	)

	BeforeEach(func() {
		reconciler = NewModelReconciler(k8sClient, ctrl.Log)
	})

	It("Returns an error if two models have the same name", func() {

		modelName := "model-name-1"
		k8sName := "k8s-deployment-name"
		k8sNamespace := "k8s-namespace"

		desired := &hostingv1.HostingDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: k8sNamespace,
				UID:       types.UID(uuid.New().String()),
			},
			Spec: hostingv1.HostingDeploymentSpec{
				ProductionVariants: []commonv1.ProductionVariant{},
				Models: []commonv1.Model{
					{
						Name: &modelName,
					},
					{
						Name: &modelName,
					},
				},
			},
		}

		err := reconciler.Reconcile(context.Background(), desired, true)

		Expect(err).To(HaveOccurred())
	})

	It("Returns an error if two containers have the same name", func() {

		containerHostname := "container-hostname"
		k8sName := "k8s-deployment-name"
		k8sNamespace := "k8s-namespace"
		modelName := "model-name-1"

		containers := []*commonv1.ContainerDefinition{
			{
				ContainerHostname: &containerHostname,
			},
			{
				ContainerHostname: &containerHostname,
			},
		}

		desired := &hostingv1.HostingDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: k8sNamespace,
				UID:       types.UID(uuid.New().String()),
			},
			Spec: hostingv1.HostingDeploymentSpec{
				ProductionVariants: []commonv1.ProductionVariant{},
				Models: []commonv1.Model{
					{
						Name:       &modelName,
						Containers: containers,
					},
				},
			},
		}

		err := reconciler.Reconcile(context.Background(), desired, true)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("container hostnames must be unique."))
	})

	It("Returns an error if no model primary container is specified", func() {

		k8sName := "k8s-deployment-name"
		k8sNamespace := "k8s-namespace"
		containers := []*commonv1.ContainerDefinition{}

		desired := &hostingv1.HostingDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: k8sNamespace,
				UID:       types.UID(uuid.New().String()),
			},
			Spec: hostingv1.HostingDeploymentSpec{
				ProductionVariants: []commonv1.ProductionVariant{},
				Models: []commonv1.Model{
					{
						Name:       ToStringPtr("model-name"),
						Containers: containers,
					},
				},
			},
		}

		err := reconciler.Reconcile(context.Background(), desired, true)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Unable to determine primary container for model"))
	})

	It("Returns an error if the primary container definition is missing", func() {

		k8sName := "k8s-deployment-name"
		k8sNamespace := "k8s-namespace"

		containers := []*commonv1.ContainerDefinition{
			{
				ContainerHostname: ToStringPtr("present-container"),
			},
		}

		desired := &hostingv1.HostingDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: k8sNamespace,
				UID:       types.UID(uuid.New().String()),
			},
			Spec: hostingv1.HostingDeploymentSpec{
				ProductionVariants: []commonv1.ProductionVariant{},
				Models: []commonv1.Model{
					{
						Name:             ToStringPtr("model-name"),
						Containers:       containers,
						PrimaryContainer: ToStringPtr("missing-container"),
					},
				},
			},
		}

		err := reconciler.Reconcile(context.Background(), desired, true)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Unknown primary container"))
	})

	Context("Fail to list k8s model", func() {
		BeforeEach(func() {
			reconciler = NewModelReconciler(FailToListK8sClient{}, ctrl.Log)
		})

		It("Returns error if unable to list K8s model", func() {
			k8sName := "k8s-deployment-name"
			k8sNamespace := "k8s-namespace"
			containers := []*commonv1.ContainerDefinition{
				{
					ContainerHostname: ToStringPtr("container"),
				},
			}

			desired := &hostingv1.HostingDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      k8sName,
					Namespace: k8sNamespace,
					UID:       types.UID(uuid.New().String()),
				},
				Spec: hostingv1.HostingDeploymentSpec{
					ProductionVariants: []commonv1.ProductionVariant{},
					Models: []commonv1.Model{
						{
							Name:       ToStringPtr("model-name"),
							Containers: containers,
						},
					},
				},
			}

			err := reconciler.Reconcile(context.Background(), desired, true)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unable to get actual models"))
		})
	})

	Context("Fail to create k8s model", func() {
		BeforeEach(func() {
			reconciler = NewModelReconciler(FailToCreateK8sClient{
				ActualClient: k8sClient,
			}, ctrl.Log)
		})

		It("Returns error if unable to create k8s model", func() {
			k8sName := "k8s-deployment-name"
			k8sNamespace := "k8s-namespace"
			containers := []*commonv1.ContainerDefinition{
				{
					ContainerHostname: ToStringPtr("container"),
				},
			}

			desired := &hostingv1.HostingDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      k8sName,
					Namespace: k8sNamespace,
					UID:       types.UID(uuid.New().String()),
				},
				Spec: hostingv1.HostingDeploymentSpec{
					ProductionVariants: []commonv1.ProductionVariant{},
					Models: []commonv1.Model{
						{
							Name:       ToStringPtr("model-name"),
							Containers: containers,
						},
					},
				},
			}

			err := reconciler.Reconcile(context.Background(), desired, true)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unable to create model"))
		})
	})

	Context("The deployment spec is valid", func() {

		var (
			k8sName           string
			k8sNamespace      string
			region            string
			modelDataUrl      string
			expectedModelName string
			tagKey            string
			tagValue          string

			desired *hostingv1.HostingDeployment
		)

		BeforeEach(func() {

			k8sName = "k8s-deployment-name"
			k8sNamespace = "k8s-namespace"
			CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)

			modelDataUrl = "s3://bucket/model.tar.gz"
			region = "us-east-2"

			tagKey = "tag-key"
			tagValue = "tag-value"

			containers := []*commonv1.ContainerDefinition{
				{
					ContainerHostname: ToStringPtr("present-container"),
					ModelDataUrl:      &modelDataUrl,
				},
			}

			desired = &hostingv1.HostingDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      k8sName,
					Namespace: k8sNamespace,
					UID:       types.UID(uuid.New().String()),
				},
				Spec: hostingv1.HostingDeploymentSpec{
					Region:             &region,
					ProductionVariants: []commonv1.ProductionVariant{},
					Models: []commonv1.Model{
						{
							Name:             ToStringPtr("model-name"),
							Containers:       containers,
							ExecutionRoleArn: ToStringPtr("xxx-yyy"),
						},
					},
					Tags: []commonv1.Tag{
						{
							Key:   &tagKey,
							Value: &tagValue,
						},
					},
				},
			}

			expectedModelName = GetSubresourceNamespacedName("model-name", *desired).Name

			reconciler.Reconcile(context.Background(), desired, true)
		})

		AfterEach(func() {
			var model modelv1.Model
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: k8sNamespace,
				Name:      expectedModelName,
			}, &model)
			Expect(err).ToNot(HaveOccurred())

			err = k8sClient.Delete(context.Background(), &model)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Created the k8s model with correct ownership labels", func() {
			var model modelv1.Model
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: k8sNamespace,
				Name:      expectedModelName,
			}, &model)
			Expect(err).ToNot(HaveOccurred())

			labels := model.ObjectMeta.Labels

			expectedLabels := GetResourceOwnershipLabelsForHostingDeployment(*desired)
			for key, value := range expectedLabels {
				Expect(labels).To(HaveKeyWithValue(key, value))
			}
		})

		It("Created the k8s model with correct modelDataUrl", func() {
			var model modelv1.Model
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: k8sNamespace,
				Name:      expectedModelName,
			}, &model)
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Spec.PrimaryContainer).ToNot(BeNil())
			Expect(model.Spec.PrimaryContainer.ModelDataUrl).ToNot(BeNil())
			Expect(*model.Spec.PrimaryContainer.ModelDataUrl).To(Equal(modelDataUrl))
		})

		It("Created the k8s model with correct region", func() {
			var model modelv1.Model
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: k8sNamespace,
				Name:      expectedModelName,
			}, &model)
			Expect(err).ToNot(HaveOccurred())

			Expect(model.Spec.Region).ToNot(BeNil())
			Expect(*model.Spec.Region).To(Equal(region))
		})

		It("Created the k8s model with correct tags", func() {
			var model modelv1.Model
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Namespace: k8sNamespace,
				Name:      expectedModelName,
			}, &model)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(model.Spec.Tags)).To(Equal(1))
			Expect(model.Spec.Tags[0].Key).ToNot(BeNil())
			Expect(*model.Spec.Tags[0].Key).To(Equal(tagKey))
			Expect(model.Spec.Tags[0].Value).ToNot(BeNil())
			Expect(*model.Spec.Tags[0].Value).To(Equal(tagValue))
		})

		Context("The model has multiple containers", func() {
			BeforeEach(func() {
				multiContainers := []*commonv1.ContainerDefinition{
					{
						ContainerHostname: ToStringPtr("present-container"),
						ModelDataUrl:      &modelDataUrl,
					},
					{
						ContainerHostname: ToStringPtr("present-container-2"),
						ModelDataUrl:      &modelDataUrl,
					},
				}

				desired.Spec.Models[0].Containers = multiContainers

				expectedModelName = GetSubresourceNamespacedName("model-name", *desired).Name

				reconciler.Reconcile(context.Background(), desired, true)
			})

			It("Created the k8s model with all containers", func() {
				var model modelv1.Model
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Namespace: k8sNamespace,
					Name:      expectedModelName,
				}, &model)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(model.Spec.Containers)).To(Equal(2))
			})

			It("Created the k8s model without primary container", func() {
				var model modelv1.Model
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Namespace: k8sNamespace,
					Name:      expectedModelName,
				}, &model)
				Expect(err).ToNot(HaveOccurred())

				Expect(model.Spec.PrimaryContainer).To(BeNil())
			})
		})
	})
})

var _ = Describe("Delete ModelReconciler.Reconcile", func() {

	var (
		reconciler        ModelReconciler
		k8sName           string
		k8sNamespace      string
		region            string
		modelDataUrl      string
		expectedModelName string
		tagKey            string
		tagValue          string

		desired *hostingv1.HostingDeployment
	)

	BeforeEach(func() {
		reconciler = NewModelReconciler(k8sClient, ctrl.Log)
		k8sName = "k8s-deployment-name"
		k8sNamespace = "k8s-namespace"
		CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)

		modelDataUrl = "s3://bucket/model.tar.gz"
		region = "us-east-2"

		tagKey = "tag-key"
		tagValue = "tag-value"

		containers := []*commonv1.ContainerDefinition{
			{
				ContainerHostname: ToStringPtr("present-container"),
				ModelDataUrl:      &modelDataUrl,
			},
		}

		desired = &hostingv1.HostingDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: k8sNamespace,
				UID:       types.UID(uuid.New().String()),
			},
			Spec: hostingv1.HostingDeploymentSpec{
				Region:             &region,
				ProductionVariants: []commonv1.ProductionVariant{},
				Models: []commonv1.Model{
					{
						Name:             ToStringPtr("model-name"),
						Containers:       containers,
						ExecutionRoleArn: ToStringPtr("xxx-yyy"),
					},
				},
				Tags: []commonv1.Tag{
					{
						Key:   &tagKey,
						Value: &tagValue,
					},
				},
			},
		}

		expectedModelName = GetSubresourceNamespacedName("model-name", *desired).Name

		var model modelv1.Model

		err := reconciler.Reconcile(context.Background(), desired, true)

		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: k8sNamespace,
			Name:      expectedModelName,
		}, &model)
		Expect(err).ToNot(HaveOccurred())

		// Mark the model to be deleted
		err = k8sClient.Delete(context.Background(), &model)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Verify that model has been deleted from k8s", func() {

		var model modelv1.Model
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: k8sNamespace,
			Name:      expectedModelName,
		}, &model)
		Expect(err).To(HaveOccurred())
		Expect(apierrs.IsNotFound(err)).To(Equal(true))
	})
	It("Verify that reconciler returns error in case if deletion fails", func() {
		//TODO Gautam Kumar
	})
})

var _ = Describe("ModelReconciler.GetSageMakerModelNames", func() {
	var (
		reconciler ModelReconciler

		desired *hostingv1.HostingDeployment

		modelName           string
		k8sName             string
		k8sNamespace        string
		modelNamespacedName types.NamespacedName
	)

	BeforeEach(func() {

		modelName = "model-name-" + uuid.New().String()
		k8sName = "k8s-name-" + uuid.New().String()
		k8sNamespace = "k8s-namespace-" + uuid.New().String()
		CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)

		containers := []*commonv1.ContainerDefinition{
			{
				ContainerHostname: ToStringPtr("present-container"),
			},
		}

		reconciler = NewModelReconciler(k8sClient, ctrl.Log)

		desired = &hostingv1.HostingDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: k8sNamespace,
				UID:       types.UID(uuid.New().String()),
			},
			Spec: hostingv1.HostingDeploymentSpec{
				Region: ToStringPtr("us-east-1"),
				ProductionVariants: []commonv1.ProductionVariant{
					{
						VariantName: ToStringPtr("variant-A"),
						ModelName:   &modelName,
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

		modelNamespacedName = GetSubresourceNamespacedName(modelName, *desired)
	})

	AfterEach(func() {
		var model modelv1.Model
		err := k8sClient.Get(context.Background(), modelNamespacedName, &model)
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Delete(context.Background(), &model)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Gets the correct SageMaker model names", func() {
		sageMakerModelName := "sagemaker-name"

		err := createCreatedModelWithSageMakerName(modelNamespacedName, *desired, sageMakerModelName)
		Expect(err).ToNot(HaveOccurred())

		names, err := reconciler.GetSageMakerModelNames(context.Background(), desired)
		Expect(err).ToNot(HaveOccurred())

		Expect(names).To(Equal(map[string]string{
			modelName: sageMakerModelName,
		}))
	})

	It("Returns an error if not all names are populated", func() {
		sageMakerModelName := ""

		err := createCreatedModelWithSageMakerName(modelNamespacedName, *desired, sageMakerModelName)
		Expect(err).ToNot(HaveOccurred())

		names, err := reconciler.GetSageMakerModelNames(context.Background(), desired)
		Expect(err).To(HaveOccurred())
		Expect(names).To(BeNil())

	})
})

var _ = Describe("Update ModelReconciler.Reconcile", func() {

	var (
		reconciler ModelReconciler

		desired             *hostingv1.HostingDeployment
		k8sNamespace        string
		modelNamespacedName types.NamespacedName
	)

	BeforeEach(func() {
		k8sName := "k8s-name-" + uuid.New().String()
		k8sNamespace = "k8s-namespace-" + uuid.New().String()
		CreateMockNamespace(context.Background(), k8sClient, k8sNamespace)

		modelName := "model-name"

		reconciler = NewModelReconciler(k8sClient, ctrl.Log)

		containers := []*commonv1.ContainerDefinition{
			{
				ContainerHostname: ToStringPtr("present-container"),
				ModelDataUrl:      ToStringPtr("s3://bucket/model.tar.gz"),
			},
		}

		desired = &hostingv1.HostingDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: k8sNamespace,
				UID:       types.UID(uuid.New().String()),
			},
			Spec: hostingv1.HostingDeploymentSpec{
				Region: ToStringPtr("us-east-1"),
				ProductionVariants: []commonv1.ProductionVariant{
					{
						VariantName: ToStringPtr("variant-A"),
						ModelName:   &modelName,
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

		modelNamespacedName = GetSubresourceNamespacedName(modelName, *desired)
		err := createCreatedModelWithSageMakerName(modelNamespacedName, *desired, "")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		var model modelv1.Model
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: k8sNamespace,
			Name:      modelNamespacedName.Name,
		}, &model)
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Delete(context.Background(), &model)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Updates Kubernetes models", func() {
		newModelDataUrl := "s3://updated-bucket/model.tar.gz"
		updated := desired.DeepCopy()
		updated.Spec.Models[0].Containers[0].ModelDataUrl = &newModelDataUrl

		err := reconciler.Reconcile(context.Background(), updated, true)
		Expect(err).ToNot(HaveOccurred())

		var model modelv1.Model
		err = k8sClient.Get(context.Background(), types.NamespacedName{
			Namespace: k8sNamespace,
			Name:      modelNamespacedName.Name,
		}, &model)
		Expect(err).ToNot(HaveOccurred())

		Expect(*model.Spec.PrimaryContainer.ModelDataUrl).To(Equal(newModelDataUrl))

	})
})
