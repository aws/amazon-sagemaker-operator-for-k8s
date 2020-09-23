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

package v1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	commonv1 "github.com/aws/amazon-sagemaker-operator-for-k8s/api/v1/common"
	. "github.com/aws/amazon-sagemaker-operator-for-k8s/controllers/controllertest"

	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// These tests are written in BDD-style using Ginkgo framework. Refer to
// http://onsi.github.io/ginkgo to learn more.

var _ = Describe("ProcessingJob", func() {
	var (
		key              types.NamespacedName
		created, fetched *ProcessingJob
	)

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	// Add Tests for OpenAPI validation (or additonal CRD features) specified in
	// your API definition.
	// Avoid adding tests for vanilla CRUD operations because they would
	// test Kubernetes API server, which isn't the goal here.
	Context("Create API", func() {

		It("should create an object successfully", func() {

			key = types.NamespacedName{
				Name:      "foo",
				Namespace: "default",
			}
			created = &ProcessingJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: ProcessingJobSpec{
					AppSpecification: &commonv1.AppSpecification{
						ContainerEntrypoint: []string{"python", "run_me.py"},
						ContainerArguments:  []string{"--region", "usa"},
						ImageURI:            "hello-world",
					},
					ProcessingOutputConfig: &commonv1.ProcessingOutputConfig{
						Outputs: []commonv1.ProcessingOutputStruct{
							{
								OutputName: "xyz",
								S3Output: commonv1.ProcessingS3Output{
									LocalPath:    "/opt/ml/output",
									S3UploadMode: "EndOfJob",
									S3Uri:        "s3://bucket/processing_output",
								},
							},
						},
					},
					ProcessingResources: &commonv1.ProcessingResources{
						ClusterConfig: &commonv1.ResourceConfig{
							InstanceCount:  ToInt64Ptr(1),
							InstanceType:   "xyz",
							VolumeSizeInGB: ToInt64Ptr(50),
						},
					},
					RoleArn:           ToStringPtr("xxxxxxxxxxxxxxxxxxxx"),
					Region:            ToStringPtr("region-xyz"),
					StoppingCondition: &commonv1.StoppingConditionNoSpot{},
				},
			}

			By("creating an API obj")
			Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

			fetched = &ProcessingJob{}
			Expect(k8sClient.Get(context.TODO(), key, fetched)).To(Succeed())
			Expect(fetched).To(Equal(created))

			By("deleting the created object")
			Expect(k8sClient.Delete(context.TODO(), created)).To(Succeed())
			Expect(k8sClient.Get(context.TODO(), key, created)).ToNot(Succeed())
		})

	})

})
